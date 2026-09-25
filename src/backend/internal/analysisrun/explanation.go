package analysisrun

import (
	"context"
	"errors"
	"time"
)

// Explanations are operator-facing text generated once per finding during the
// run (ADR-006). They only describe the deterministic result: providers
// receive the finding and its evidence, and return text. Nothing they return
// can change the type, severity, confidence, priority, evidence or action
// code, which are persisted from the engine as before.

// ExplanationSource names what produced the persisted text.
type ExplanationSource string

// Explanation sources.
const (
	SourceDeterministic ExplanationSource = "DETERMINISTIC"
	SourceOllama        ExplanationSource = "OLLAMA"
)

// ExplanationInput is everything a provider receives for one finding: the
// deterministic conclusions and the finding's persisted evidence. It carries
// no other meters, readings outside the evidence, credentials or session data.
type ExplanationInput struct {
	MeterID           string
	Type              string
	Severity          string
	Confidence        float64
	Priority          int
	RecommendedAction string
	Reason            string
	StartedAt         time.Time
	LastObservedAt    time.Time
	Evidence          Evidence
}

// ExplanationText is the four-part explanation of one finding.
type ExplanationText struct {
	Summary               string `json:"summary"`
	WhyItMatters          string `json:"why_it_matters"`
	EvidenceNarrative     string `json:"evidence_narrative"`
	RecommendedActionText string `json:"recommended_action_text"`
}

// Explanation is a provider's result: the text and what produced it.
type Explanation struct {
	Text          ExplanationText
	Source        ExplanationSource
	Model         *string // generated text only
	PromptVersion string  // version of the prompt or template that produced the text
}

// ExplanationProvider turns one finding into operator language. It is the
// boundary between orchestration and the deterministic and Ollama providers
// (internal/explanation).
type ExplanationProvider interface {
	Explain(ctx context.Context, in ExplanationInput) (Explanation, error)
}

// ExplanationSettings describes how a run's explanations are produced. It is
// persisted with the run (analysis_runs.explanation_configuration) and never
// contains URLs or credentials.
type ExplanationSettings struct {
	Provider      string  `json:"provider"`
	Model         *string `json:"model"`
	PromptVersion string  `json:"prompt_version"`
	TimeoutMS     *int64  `json:"timeout_ms,omitempty"`
}

// Explainers configures explanation generation. Primary is the configured
// provider; Fallback (the deterministic provider) replaces its output when it
// fails. Fallback is nil when Primary is already deterministic.
type Explainers struct {
	Primary  ExplanationProvider
	Fallback ExplanationProvider
	Settings ExplanationSettings
}

// Sanitized reasons for using the fallback. They are stored and logged; raw
// provider errors never leave the process.
const (
	FallbackProviderUnavailable = "provider_unavailable"
	FallbackTimeout             = "timeout"
	FallbackProviderError       = "provider_error"
	FallbackInvalidResponse     = "invalid_response"
	FallbackValidationFailed    = "validation_failed"
)

// ExplanationError is a provider failure with its sanitized fallback code.
type ExplanationError struct {
	Code string
	Err  error
}

func (e *ExplanationError) Error() string { return "explanation " + e.Code + ": " + e.Err.Error() }
func (e *ExplanationError) Unwrap() error { return e.Err }

// FallbackCode classifies a provider error. Deadlines are timeouts whatever
// the provider reported.
func FallbackCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return FallbackTimeout
	}
	var ee *ExplanationError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return FallbackProviderError
}

// storedExplanation is what persist writes for one finding; nil when no
// provider produced text (the finding is still stored).
type storedExplanation struct {
	Explanation
	GeneratedAt  time.Time
	FallbackCode *string
}
