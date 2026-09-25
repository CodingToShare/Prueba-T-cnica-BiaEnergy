package analysisrun

import (
	"time"

	"bia-energy.local/backend/internal/analysis"
	"bia-energy.local/backend/internal/platform/jsontime"
)

// EvidenceSchemaVersion versions the JSONB evidence document (ADR-003).
const EvidenceSchemaVersion = 1

// Evidence is the persisted and published form of a finding's structured
// evidence (anomalies.evidence). It is a typed copy of the engine's
// evidence: nothing is recomputed. Percentages (*_pct) are percent values
// (47.3 means 47.3%); fractions of 1 are named as such.
type Evidence struct {
	SchemaVersion       int                 `json:"schema_version"`
	Rule                string              `json:"rule"`
	EvidenceStrength    float64             `json:"evidence_strength"`
	Consumption         ConsumptionEvidence `json:"consumption"`
	Persistence         PersistenceEvidence `json:"persistence"`
	Metrics             []MetricEvidence    `json:"metrics"`
	RelatedEvents       []RelatedEvent      `json:"related_events"`
	ConfidenceBreakdown ConfidenceBreakdown `json:"confidence_breakdown"`
	Signals             []Signal            `json:"signals"`
}

// ConsumptionEvidence compares observed and baseline energy over the
// episode's flagged readings.
type ConsumptionEvidence struct {
	BaselineKWh        float64 `json:"baseline_kwh"`
	ObservedKWh        float64 `json:"observed_kwh"`
	DeviationPct       float64 `json:"deviation_pct"`
	MedianDeviationPct float64 `json:"median_deviation_pct"`
	Direction          string  `json:"direction"`
}

// PersistenceEvidence describes how sustained the episode is.
type PersistenceEvidence struct {
	DurationHours   float64          `json:"duration_hours"`
	FlaggedReadings int              `json:"flagged_readings"`
	SpanReadings    int              `json:"span_readings"`
	DensityFraction float64          `json:"density_fraction"`
	LongestRun      int              `json:"longest_run"`
	Sustained       bool             `json:"sustained"`
	Recovery        RecoveryEvidence `json:"recovery"`
}

// RecoveryEvidence reports what the readings after the episode showed.
type RecoveryEvidence struct {
	Recovered                     bool             `json:"recovered"`
	RecoveredAt                   *jsontime.Source `json:"recovered_at"`
	ReadingsObserved              int              `json:"readings_observed"`
	MedianConsumptionDeviationPct float64          `json:"median_consumption_deviation_pct"`
}

// MetricEvidence aggregates one metric over the flagged readings.
type MetricEvidence struct {
	Metric             string  `json:"metric"`
	EvaluatedReadings  int     `json:"evaluated_readings"`
	TriggeredReadings  int     `json:"triggered_readings"`
	MedianObserved     float64 `json:"median_observed"`
	MedianBaseline     float64 `json:"median_baseline"`
	MedianDeviationPct float64 `json:"median_deviation_pct"`
	MaxAbsDeviationPct float64 `json:"max_abs_deviation_pct"`
	Direction          *string `json:"direction"`
	Corroborates       bool    `json:"corroborates"`
}

// RelatedEvent is an event correlated with the onset of the finding.
type RelatedEvent struct {
	Timestamp     jsontime.Source `json:"timestamp"`
	Type          string          `json:"type"`
	Description   string          `json:"description"`
	OffsetSeconds int64           `json:"offset_seconds"`
	Role          string          `json:"role"`
}

// ConfidenceBreakdown holds the [0,1] components of the confidence.
type ConfidenceBreakdown struct {
	SignalStrength      float64 `json:"signal_strength"`
	Persistence         float64 `json:"persistence"`
	MultivariateSupport float64 `json:"multivariate_support"`
	EventContext        float64 `json:"event_context"`
	PatternSupport      float64 `json:"pattern_support"`
}

// Signal is one metric of one reading that passed both detection gates.
type Signal struct {
	Metric       string          `json:"metric"`
	Timestamp    jsontime.Source `json:"timestamp"`
	Observed     float64         `json:"observed"`
	Baseline     float64         `json:"baseline"`
	Deviation    float64         `json:"deviation"`
	DeviationPct float64         `json:"deviation_pct"`
	RobustZ      float64         `json:"robust_z"`
	Direction    string          `json:"direction"`
	Strength     float64         `json:"strength"`
}

// percent converts an engine fraction to the published percent value.
func percent(fraction float64) float64 { return fraction * 100 }

// EvidenceOf maps an engine finding to its evidence document.
func EvidenceOf(f analysis.Finding) Evidence {
	e := Evidence{
		SchemaVersion:    EvidenceSchemaVersion,
		Rule:             string(f.Rule),
		EvidenceStrength: f.EvidenceStrength,
		Consumption: ConsumptionEvidence{
			BaselineKWh:        f.Consumption.BaselineKWh,
			ObservedKWh:        f.Consumption.ObservedKWh,
			DeviationPct:       percent(f.Consumption.DeviationPct),
			MedianDeviationPct: percent(f.Consumption.MedianDeviationPct),
			Direction:          string(f.Consumption.Direction),
		},
		Persistence: PersistenceEvidence{
			DurationHours:   f.Duration.Hours(),
			FlaggedReadings: f.Persistence.FlaggedReadings,
			SpanReadings:    f.Persistence.SpanReadings,
			DensityFraction: f.Persistence.Density,
			LongestRun:      f.Persistence.LongestRun,
			Sustained:       f.Persistence.Sustained,
			Recovery: RecoveryEvidence{
				Recovered:                     f.Persistence.Recovery.Recovered,
				ReadingsObserved:              f.Persistence.Recovery.ReadingsObserved,
				MedianConsumptionDeviationPct: percent(f.Persistence.Recovery.MedianConsumptionDeviationPct),
			},
		},
		ConfidenceBreakdown: ConfidenceBreakdown{
			SignalStrength:      f.ConfidenceDetail.SignalStrength,
			Persistence:         f.ConfidenceDetail.Persistence,
			MultivariateSupport: f.ConfidenceDetail.MultivariateSupport,
			EventContext:        f.ConfidenceDetail.EventContext,
			PatternSupport:      f.ConfidenceDetail.PatternSupport,
		},
		Metrics:       make([]MetricEvidence, 0, len(f.Metrics)),
		RelatedEvents: make([]RelatedEvent, 0, len(f.RelatedEvents)),
		Signals:       make([]Signal, 0, len(f.Signals)),
	}
	if at := f.Persistence.Recovery.RecoveredAt; !at.IsZero() {
		e.Persistence.Recovery.RecoveredAt = jsontime.SourcePtr(&at)
	}
	for _, m := range f.Metrics {
		me := MetricEvidence{
			Metric:             string(m.Metric),
			EvaluatedReadings:  m.EvaluatedReadings,
			TriggeredReadings:  m.TriggeredReadings,
			MedianObserved:     m.MedianObserved,
			MedianBaseline:     m.MedianBaseline,
			MedianDeviationPct: percent(m.MedianDeviationPct),
			MaxAbsDeviationPct: percent(m.MaxAbsDeviationPct),
			Corroborates:       m.Corroborates,
		}
		if m.Direction != "" {
			d := string(m.Direction)
			me.Direction = &d
		}
		e.Metrics = append(e.Metrics, me)
	}
	for _, r := range f.RelatedEvents {
		e.RelatedEvents = append(e.RelatedEvents, RelatedEvent{
			Timestamp:     jsontime.Source(r.Timestamp),
			Type:          string(r.Type),
			Description:   r.Description,
			OffsetSeconds: int64(r.Offset / time.Second),
			Role:          string(r.Role),
		})
	}
	for _, s := range f.Signals {
		e.Signals = append(e.Signals, Signal{
			Metric:       string(s.Metric),
			Timestamp:    jsontime.Source(s.Timestamp),
			Observed:     s.Observed,
			Baseline:     s.Baseline,
			Deviation:    s.Deviation,
			DeviationPct: percent(s.DeviationPct),
			RobustZ:      s.RobustZ,
			Direction:    string(s.Direction),
			Strength:     s.Strength,
		})
	}
	return e
}
