package explanation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"bia-energy.local/backend/internal/analysisrun"
)

// Maximum length of each generated field, in characters. The deterministic
// fields are well below these limits.
const (
	maxSummaryChars   = 320
	maxParagraphChars = 700
)

var fieldLimits = []struct {
	name  string
	value func(analysisrun.ExplanationText) string
	max   int
}{
	{"summary", func(t analysisrun.ExplanationText) string { return t.Summary }, maxSummaryChars},
	{"why_it_matters", func(t analysisrun.ExplanationText) string { return t.WhyItMatters }, maxParagraphChars},
	{"evidence_narrative", func(t analysisrun.ExplanationText) string { return t.EvidenceNarrative }, maxParagraphChars},
	{"recommended_action_text", func(t analysisrun.ExplanationText) string { return t.RecommendedActionText }, maxParagraphChars},
}

// parseText decodes the model's reply strictly: one JSON object with exactly
// the four string fields. Anything else (prose, Markdown fences, extra or
// missing fields) is rejected rather than repaired.
func parseText(content string) (analysisrun.ExplanationText, error) {
	dec := json.NewDecoder(strings.NewReader(content))
	token, err := dec.Token()
	if err != nil || token != json.Delim('{') {
		return analysisrun.ExplanationText{}, errors.New("reply is not the expected JSON object")
	}
	allowed := map[string]bool{
		"summary": true, "why_it_matters": true,
		"evidence_narrative": true, "recommended_action_text": true,
	}
	values := make(map[string]string, len(allowed))
	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			return analysisrun.ExplanationText{}, fmt.Errorf("reply is not the expected JSON object: %w", err)
		}
		key, ok := keyToken.(string)
		if !ok || !allowed[key] {
			return analysisrun.ExplanationText{}, fmt.Errorf("reply contains unexpected field %q", key)
		}
		if _, exists := values[key]; exists {
			return analysisrun.ExplanationText{}, fmt.Errorf("reply contains duplicate field %q", key)
		}
		var value string
		if err := dec.Decode(&value); err != nil {
			return analysisrun.ExplanationText{}, fmt.Errorf("reply field %q is not a string: %w", key, err)
		}
		values[key] = strings.TrimSpace(value)
	}
	if _, err := dec.Token(); err != nil {
		return analysisrun.ExplanationText{}, fmt.Errorf("reply is not the expected JSON object: %w", err)
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return analysisrun.ExplanationText{}, errors.New("reply has content after the JSON object")
	}
	if len(values) != len(allowed) {
		return analysisrun.ExplanationText{}, errors.New("reply is missing a required field")
	}
	return analysisrun.ExplanationText{
		Summary:               values["summary"],
		WhyItMatters:          values["why_it_matters"],
		EvidenceNarrative:     values["evidence_narrative"],
		RecommendedActionText: values["recommended_action_text"],
	}, nil
}

// validate checks generated text against the finding it explains. It is a
// guard, not a proof of truth: it rejects the failure modes that matter most
// (invented numbers or explanatory events, contradicting the classification
// or the action, markup, runaway length) and the fallback takes over.
func validate(t analysisrun.ExplanationText, in analysisrun.ExplanationInput, _ []byte) error {
	if err := validateTextFields(t); err != nil {
		return err
	}
	p := payloadOf(in)
	severitySentence := fmt.Sprintf(" The analysis rates it %s severity.", strings.ToLower(in.Severity))
	if t.WhyItMatters != p.ClassificationMeaning && t.WhyItMatters != p.ClassificationMeaning+severitySentence {
		return errors.New("why_it_matters is not the supplied classification meaning")
	}
	if t.EvidenceNarrative != p.EvidenceNarrative {
		return errors.New("evidence_narrative is not the supplied controlled evidence")
	}
	if t.RecommendedActionText != p.RecommendedAction.Description {
		return errors.New("recommended_action_text is not the supplied action")
	}
	// Numeric facts stay in the structured evidence rendered beside the
	// narrative. Rejecting model-authored numbers avoids accepting a real
	// number attached to the wrong metric, duration or quantity.
	generatedNarrative := t.Summary + " " + t.EvidenceNarrative
	if len(numbersIn(generatedNarrative)) > 0 {
		return errors.New("generated narrative contains a number; numeric facts must use the structured evidence")
	}

	all := strings.Join([]string{t.Summary, t.WhyItMatters, t.EvidenceNarrative, t.RecommendedActionText}, " ")
	lower := strings.ToLower(all)
	if err := validateEventClaims(lower, in); err != nil {
		return err
	}

	if in.Type != "FALSE_POSITIVE" && strings.Contains(lower, "false positive") {
		return errors.New("contradicts the classification")
	}
	return nil
}

func validateTextFields(t analysisrun.ExplanationText) error {
	for _, f := range fieldLimits {
		v := f.value(t)
		if v == "" {
			return fmt.Errorf("%s is empty", f.name)
		}
		if utf8.RuneCountInString(v) > f.max {
			return fmt.Errorf("%s exceeds %d characters", f.name, f.max)
		}
		if strings.ContainsAny(v, "<>`#*") || hasControl(v) {
			return fmt.Errorf("%s contains markup or control characters", f.name)
		}
	}
	return nil
}

var eventPhrases = map[string][]string{
	"SCHEDULED_OUTAGE":   {"scheduled_outage", "scheduled outage", "outage"},
	"OPERATIONAL_CHANGE": {"operational_change", "operational change"},
	"UNKNOWN":            {"unknown event"},
	"DATA_QUALITY":       {"data_quality event", "data quality event", "data-quality event"},
}

func validateEventClaims(lower string, in analysisrun.ExplanationInput) error {
	present := map[string]bool{}
	hasExplainingEvent := false
	for _, ev := range in.Evidence.RelatedEvents {
		present[ev.Type] = true
		hasExplainingEvent = hasExplainingEvent || ev.Role == "EXPLAINS"
		if ev.Role != "EXPLAINS" {
			name := strings.ToLower(strings.ReplaceAll(ev.Type, "_", " "))
			for _, claim := range []string{name + " caused", name + " explains", name + " accounts for", name + " led to", name + " resulted in"} {
				if strings.Contains(lower, claim) {
					return fmt.Errorf("presents a %s event with role %s as causal", ev.Type, ev.Role)
				}
			}
		}
	}
	for eventType, phrases := range eventPhrases {
		if present[eventType] {
			continue
		}
		for _, phrase := range phrases {
			if strings.Contains(lower, phrase) {
				return fmt.Errorf("mentions a %s event that is not in the evidence", eventType)
			}
		}
	}
	if !hasExplainingEvent {
		causal := []string{"event caused", "event accounts for", "because of the event", "due to the event", "event led to", "event resulted in"}
		for _, phrase := range causal {
			if strings.Contains(lower, phrase) {
				return errors.New("presents a non-explaining event as causal")
			}
		}
	}
	return nil
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' {
			return true
		}
	}
	return false
}

var numberPattern = regexp.MustCompile(`(\d{1,3}(?:,\d{3})+(?:\.\d+)?|\d+(?:\.\d+)?)(\s*(?:%|percent))?`)

type number struct {
	value   float64
	percent bool
}

func numbersIn(s string) []number {
	var out []number
	for _, m := range numberPattern.FindAllStringSubmatch(s, -1) {
		if v, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64); err == nil {
			out = append(out, number{value: v, percent: m[2] != ""})
		}
	}
	return out
}
