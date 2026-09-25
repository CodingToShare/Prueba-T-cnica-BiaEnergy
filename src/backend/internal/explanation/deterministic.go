// Package explanation implements the explanation providers of ADR-006: the
// deterministic provider (default and fallback) and the optional local Ollama
// provider. Both only put a finding's deterministic result into words; the
// boundary they implement, analysisrun.ExplanationProvider, returns text and
// nothing that could change the type, severity, confidence, priority,
// evidence or recommended action code.
package explanation

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"bia-energy.local/backend/internal/analysisrun"
	"bia-energy.local/backend/internal/platform/jsontime"
)

// TemplateVersion identifies the deterministic templates; it is persisted as
// the prompt version of deterministic explanations. Change it deliberately
// when the wording rules change.
const TemplateVersion = "evidence-template-v1"

// Deterministic builds explanations from templates over the evidence. It
// needs no network, never fails and is the fallback of generative providers.
type Deterministic struct{}

// DeterministicSettings is the run provenance of the deterministic provider.
func DeterministicSettings() analysisrun.ExplanationSettings {
	return analysisrun.ExplanationSettings{Provider: "deterministic", PromptVersion: TemplateVersion}
}

// Explain implements analysisrun.ExplanationProvider.
func (Deterministic) Explain(_ context.Context, in analysisrun.ExplanationInput) (analysisrun.Explanation, error) {
	return analysisrun.Explanation{
		Text:          deterministicText(in),
		Source:        analysisrun.SourceDeterministic,
		PromptVersion: TemplateVersion,
	}, nil
}

func deterministicText(in analysisrun.ExplanationInput) analysisrun.ExplanationText {
	f := facts(in)
	var t analysisrun.ExplanationText
	switch in.Type {
	case "REAL_ANOMALY":
		t.Summary = fmt.Sprintf("Consumption %s its hourly baseline by %s for %s, and no recorded event explains the change.",
			f.movement, f.deviation, f.duration)
		t.WhyItMatters = "A persistent change that recorded operational context does not explain may point to a fault, an unexpected load or a problem in the installation. " + f.severitySentence
		t.EvidenceNarrative = join(f.persistence, f.support, f.eventSentences(), f.recovery)
	case "EXPLAINABLE_ANOMALY":
		context := "recorded operational context explains the change"
		if f.explaining != nil {
			context = fmt.Sprintf("in line with %s recorded %s", eventPhrase(f.explaining.Type), offsetPhrase(f.explaining.OffsetSeconds))
		}
		t.Summary = fmt.Sprintf("Consumption %s its hourly baseline by %s for %s, %s.", f.movement, f.deviation, f.duration, context)
		t.WhyItMatters = "The change is real, but recorded operational context accounts for it, so it calls for validating the new operating level rather than investigating a fault. " + f.severitySentence
		t.EvidenceNarrative = join(f.persistence, f.eventSentences(), f.support, f.recovery)
	case "FALSE_POSITIVE":
		context := "recorded operational context explains it"
		if f.explaining != nil {
			context = eventPhrase(f.explaining.Type) + " explains it"
		}
		recovered := ""
		if in.Evidence.Persistence.Recovery.Recovered {
			recovered = " and readings recovered afterwards"
		}
		t.Summary = fmt.Sprintf("Consumption %s its hourly baseline for %s, but %s%s.", f.movement, f.duration, context, recovered)
		t.WhyItMatters = "The deviation was real but explained by recorded operational context and temporary, so it does not need escalation. " + f.severitySentence
		t.EvidenceNarrative = join(f.persistence, f.eventSentences(), f.recovery)
	case "DATA_QUALITY":
		vars := "Electrical"
		if len(f.supporting) > 0 {
			vars = capitalize(listPhrase(f.supporting))
		}
		t.Summary = fmt.Sprintf("%s readings were inconsistent with each other while consumption stayed within ±%s of its baseline.",
			vars, pct(f.consumptionMaxAbs))
		t.WhyItMatters = "The measurements themselves may be unreliable, so this is a data-quality problem rather than a change in energy use; readings from this meter should not be relied on until it is validated. " + f.severitySentence
		t.EvidenceNarrative = join(
			fmt.Sprintf("The inconsistency lasted %s, with %d of %d readings in the episode flagged.", f.duration,
				in.Evidence.Persistence.FlaggedReadings, in.Evidence.Persistence.SpanReadings),
			f.eventSentences())
	default:
		// A classification unknown to these templates: restate the engine's
		// own sentence rather than inventing a meaning.
		t.Summary = in.Reason
		t.WhyItMatters = f.severitySentence
		t.EvidenceNarrative = join(f.persistence, f.support, f.eventSentences(), f.recovery)
	}
	t.RecommendedActionText = actionText(in.RecommendedAction)
	return t
}

// findingFacts are phrases formatted from the evidence, shared by templates.
type findingFacts struct {
	movement          string // "rose above" / "fell below"
	deviation         string
	duration          string
	persistence       string
	support           string
	recovery          string
	severitySentence  string
	supporting        []string
	consumptionMaxAbs float64
	explaining        *analysisrun.RelatedEvent
	events            []analysisrun.RelatedEvent
}

func facts(in analysisrun.ExplanationInput) findingFacts {
	e := in.Evidence
	f := findingFacts{
		movement:         "rose above",
		deviation:        pct(math.Abs(e.Consumption.DeviationPct)),
		duration:         hours(e.Persistence.DurationHours),
		events:           e.RelatedEvents,
		severitySentence: fmt.Sprintf("The analysis rates it %s severity.", strings.ToLower(in.Severity)),
	}
	if e.Consumption.Direction == "DOWN" {
		f.movement = "fell below"
	}
	for _, m := range e.Metrics {
		if m.Corroborates {
			f.supporting = append(f.supporting, metricName(m.Metric))
		}
		if m.Metric == "consumption_kwh" {
			f.consumptionMaxAbs = m.MaxAbsDeviationPct
		}
	}
	for i, ev := range e.RelatedEvents {
		if ev.Role == "EXPLAINS" {
			f.explaining = &e.RelatedEvents[i]
			break
		}
	}

	p := e.Persistence
	f.persistence = fmt.Sprintf("%d of %d hourly readings in the episode deviated from the baseline", p.FlaggedReadings, p.SpanReadings)
	if p.Sustained {
		f.persistence += ", a sustained change rather than a spike"
	}
	f.persistence += "."
	if len(f.supporting) > 0 {
		f.support = capitalize(listPhrase(f.supporting)) + " changed in the same period, supporting the finding."
	} else {
		f.support = "No other electrical variable changed consistently with it."
	}
	switch {
	case p.Recovery.Recovered && p.Recovery.RecoveredAt != nil:
		f.recovery = fmt.Sprintf("Readings returned to the baseline from %s.", sourceTime(*p.Recovery.RecoveredAt))
	case p.Recovery.Recovered:
		f.recovery = "Readings returned to the baseline afterwards."
	default:
		f.recovery = "Readings had not returned to the baseline by the end of the data."
	}
	return f
}

// eventSentences states each correlated event with its role; an event that is
// only context is never presented as an explanation.
func (f findingFacts) eventSentences() string {
	if len(f.events) == 0 {
		return "No operational or data event was recorded near the onset."
	}
	parts := make([]string, 0, len(f.events))
	for _, ev := range f.events {
		subject := capitalize(eventPhrase(ev.Type)) + " recorded " + offsetPhrase(ev.OffsetSeconds)
		switch ev.Role {
		case "EXPLAINS":
			parts = append(parts, subject+" explains the deviation.")
		case "CORROBORATES":
			parts = append(parts, subject+" corroborates the finding.")
		default:
			parts = append(parts, subject+" is context only and does not explain the change.")
		}
	}
	return join(parts...)
}

var metricNames = map[string]string{
	"consumption_kwh":                 "consumption",
	"voltage_v":                       "voltage",
	"current_a":                       "current",
	"power_factor":                    "power factor",
	"consumption_to_load_proxy_ratio": "consumption-to-load ratio",
}

func metricName(m string) string {
	if name, ok := metricNames[m]; ok {
		return name
	}
	return strings.ReplaceAll(m, "_", " ")
}

// eventPhrase names an event by its recorded type, e.g. "a scheduled outage
// event"; unknown types are named as recorded.
func eventPhrase(eventType string) string {
	name := strings.ToLower(strings.ReplaceAll(eventType, "_", " "))
	article := "a"
	if strings.ContainsRune("aeiou", rune(name[0])) {
		article = "an"
	}
	return article + " " + name + " event"
}

func offsetPhrase(seconds int64) string {
	switch {
	case seconds == 0:
		return "at the onset"
	case seconds < 0:
		return hoursApart(-seconds) + " before the onset"
	default:
		return hoursApart(seconds) + " after the onset"
	}
}

func hoursApart(seconds int64) string {
	if seconds%3600 == 0 {
		return hours(float64(seconds) / 3600)
	}
	return fmt.Sprintf("%d minutes", seconds/60)
}

var actionTexts = map[string]string{
	"INVESTIGATE_METER_AND_INSTALLATION": "Inspect the meter and its installation on site and confirm whether new loads, faults or equipment changes explain the deviation.",
	"VALIDATE_MEASUREMENT_OR_SENSOR":     "Check the meter's sensors, wiring and communication before trusting its electrical readings.",
	"VALIDATE_OPERATIONAL_CHANGE":        "Confirm with operations that the recorded change is expected, and review whether the new level should become the reference.",
	"NO_ESCALATION_MONITOR":              "No escalation is needed; keep monitoring the meter as usual.",
}

func actionText(code string) string {
	if text, ok := actionTexts[code]; ok {
		return text
	}
	return "Follow the recommended action " + code + "."
}

func pct(v float64) string { return fmt.Sprintf("%.0f%%", v) }

func hours(h float64) string {
	switch {
	case h == 1:
		return "1 hour"
	case h == math.Trunc(h):
		return fmt.Sprintf("%.0f hours", h)
	default:
		return fmt.Sprintf("%.1f hours", h)
	}
}

// sourceTime formats a source wall-clock time as recorded (ADR-008).
func sourceTime(t jsontime.Source) string { return time.Time(t).Format("2006-01-02 15:04") }

func listPhrase(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func join(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}
