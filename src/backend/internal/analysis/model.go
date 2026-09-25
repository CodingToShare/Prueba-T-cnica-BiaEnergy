// Package analysis is the deterministic anomaly engine (ADR-004).
//
// It turns hourly meter readings and known events into classified,
// prioritized findings that carry their own evidence. It performs no I/O and
// uses no clock, network or randomness: the same input and configuration
// always produce the same result. The algorithm is documented in
// docs/ai/anomaly-analysis.md.
package analysis

import "time"

// Reading is one hourly measurement of a meter. Timestamp is the source wall
// clock (ADR-008); only its calendar position and hour of day are used.
type Reading struct {
	MeterID        string
	Timestamp      time.Time
	ConsumptionKWh float64
	VoltageV       float64
	CurrentA       float64
	PowerFactor    float64
}

// Event is a known operational or data event reported for a meter.
type Event struct {
	MeterID     string
	Timestamp   time.Time
	Type        EventType
	Description string
}

// EventType is the structured type of an event. Types other than the
// constants below are preserved as context but never explain a finding.
type EventType string

// Known event types.
const (
	EventOperationalChange EventType = "OPERATIONAL_CHANGE"
	EventScheduledOutage   EventType = "SCHEDULED_OUTAGE"
	EventUnknown           EventType = "UNKNOWN"
	EventDataQuality       EventType = "DATA_QUALITY"
)

// Metric names an analyzed feature of a reading.
type Metric string

// Analyzed metrics, in evidence order.
const (
	MetricConsumption Metric = "consumption_kwh"
	MetricVoltage     Metric = "voltage_v"
	MetricCurrent     Metric = "current_a"
	MetricPowerFactor Metric = "power_factor"
	// MetricLoadRatio is consumption_kwh / (voltage_v × current_a ×
	// power_factor / 1000). It is a relative consistency feature compared
	// with the meter's own history, not a physical identity: the wiring and
	// conversion factors are unknown, so it is never checked against 1.
	MetricLoadRatio Metric = "consumption_to_load_proxy_ratio"
)

// Direction is the sign of a deviation from the baseline.
type Direction string

// Directions. DirectionMixed appears only in aggregated metric evidence.
const (
	DirectionUp    Direction = "UP"
	DirectionDown  Direction = "DOWN"
	DirectionMixed Direction = "MIXED"
)

// Classification is the canonical finding type (BR-01).
type Classification string

// Classifications.
const (
	RealAnomaly        Classification = "REAL_ANOMALY"
	ExplainableAnomaly Classification = "EXPLAINABLE_ANOMALY"
	FalsePositive      Classification = "FALSE_POSITIVE"
	DataQuality        Classification = "DATA_QUALITY"
)

// Severity is the operational importance of a finding. It is independent of
// confidence, which measures how well the evidence supports the finding.
type Severity string

// Severities.
const (
	SeverityHigh   Severity = "HIGH"
	SeverityMedium Severity = "MEDIUM"
	SeverityLow    Severity = "LOW"
)

// Action is the deterministic recommended next step (BR-08).
type Action string

// Recommended actions, one per classification.
const (
	ActionInvestigateMeterAndInstallation Action = "INVESTIGATE_METER_AND_INSTALLATION"
	ActionValidateOperationalChange       Action = "VALIDATE_OPERATIONAL_CHANGE"
	ActionValidateMeasurementOrSensor     Action = "VALIDATE_MEASUREMENT_OR_SENSOR"
	ActionNoEscalationMonitor             Action = "NO_ESCALATION_MONITOR"
)

// Rule identifies the decision path that produced a classification.
type Rule string

// Classification rules.
const (
	RuleUnexplainedConsumptionShift     Rule = "UNEXPLAINED_PERSISTENT_CONSUMPTION_SHIFT"
	RuleOperationalChangeExplains       Rule = "CONSUMPTION_SHIFT_EXPLAINED_BY_OPERATIONAL_CHANGE"
	RuleScheduledOutageWithRecovery     Rule = "TEMPORARY_DEVIATION_EXPLAINED_BY_SCHEDULED_OUTAGE"
	RuleScheduledOutageNoRecovery       Rule = "SCHEDULED_OUTAGE_WITHOUT_OBSERVED_RECOVERY"
	RuleRepeatedElectricalInconsistency Rule = "REPEATED_ELECTRICAL_INCONSISTENCY"
)

// EventRole is how a correlated event relates to a finding.
type EventRole string

// Event roles.
const (
	// RoleExplains marks an event whose type can account for the deviation.
	RoleExplains EventRole = "EXPLAINS"
	// RoleCorroborates marks a data-quality event supporting a data-quality
	// finding; it never creates one.
	RoleCorroborates EventRole = "CORROBORATES"
	// RoleContext marks an event kept for the operator without explaining
	// anything (for example UNKNOWN or an unrecognized type).
	RoleContext EventRole = "CONTEXT"
)

// MeterStatus is the analytically computed meter health, distinct from the
// source status column (TD-07).
type MeterStatus string

// Meter statuses.
const (
	StatusOK       MeterStatus = "OK"
	StatusAlert    MeterStatus = "ALERT"
	StatusCritical MeterStatus = "CRITICAL"
)

// Result is the output of one analysis.
type Result struct {
	// Findings are ordered by priority; Findings[i].Priority == i+1.
	Findings []Finding
	// Meters has one summary per analyzed meter, ordered by meter ID.
	Meters []MeterSummary
}

// Finding is one classified episode of abnormal behavior with the evidence
// that supports it. Evidence is the source of truth for any later wording.
type Finding struct {
	Priority          int
	MeterID           string
	Type              Classification
	Rule              Rule
	Severity          Severity
	Confidence        float64
	ConfidenceDetail  ConfidenceBreakdown
	RecommendedAction Action
	// Reason is a one-sentence summary generated only from the evidence below.
	Reason string

	StartedAt      time.Time
	LastObservedAt time.Time
	// Duration spans from the first to the last flagged reading, inclusive
	// of the last reading's interval.
	Duration time.Duration
	// EvidenceStrength is the median size of the primary signals relative to
	// their thresholds (1 means exactly at the threshold).
	EvidenceStrength float64

	Consumption   ConsumptionEvidence
	Persistence   Persistence
	Metrics       []MetricEvidence
	RelatedEvents []RelatedEvent
	Signals       []Signal
}

// ConsumptionEvidence compares observed and expected consumption over the
// episode's flagged readings.
type ConsumptionEvidence struct {
	BaselineKWh        float64
	ObservedKWh        float64
	DeviationPct       float64 // ObservedKWh / BaselineKWh − 1
	MedianDeviationPct float64 // median of the hourly relative deviations
	Direction          Direction
}

// Persistence describes how sustained an episode is.
type Persistence struct {
	FlaggedReadings int
	// SpanReadings counts every reading from the first to the last flagged one.
	SpanReadings int
	// Density is FlaggedReadings / SpanReadings.
	Density float64
	// LongestRun is the longest sequence of flagged readings one interval apart.
	LongestRun int
	// Sustained reports Duration >= Config.SustainedDuration.
	Sustained bool
	Recovery  Recovery
}

// Recovery reports whether readings after the episode returned toward the
// baseline.
type Recovery struct {
	Recovered bool
	// RecoveredAt is the first reading of the recovery sequence (zero if none).
	RecoveredAt time.Time
	// ReadingsObserved counts consecutive evaluated readings after the episode
	// without its signal, up to Config.RecoveryReadings.
	ReadingsObserved int
	// MedianConsumptionDeviationPct is measured over those readings.
	MedianConsumptionDeviationPct float64
}

// MetricEvidence aggregates one metric over the episode's flagged readings.
type MetricEvidence struct {
	Metric             Metric
	EvaluatedReadings  int
	TriggeredReadings  int
	MedianObserved     float64
	MedianBaseline     float64
	MedianDeviationPct float64
	MaxAbsDeviationPct float64
	// Direction is the dominant direction of the triggered readings (empty
	// when none triggered).
	Direction Direction
	// Corroborates reports that this electrical metric changed in at least
	// Config.CorroborationShare of the flagged readings and supports the
	// finding. In a consumption finding, current must move in the same
	// direction and the load ratio never corroborates (its change means the
	// electrical metrics did not follow the load). Always false for
	// consumption, the primary metric. Voltage and power factor confirm load
	// only with coherent current support. A corroborating derived ratio is
	// retained as evidence but never adds an independent confidence vote.
	Corroborates bool
}

// Signal is one metric of one reading that passed both detection gates.
type Signal struct {
	Metric       Metric
	Timestamp    time.Time
	Observed     float64
	Baseline     float64
	Deviation    float64
	DeviationPct float64
	RobustZ      float64
	Direction    Direction
	// Strength is |DeviationPct| divided by the metric's threshold (>= 1).
	Strength float64
}

// RelatedEvent is an event correlated with the onset of a finding.
type RelatedEvent struct {
	Timestamp   time.Time
	Type        EventType
	Description string
	// Offset is the event time minus the finding's StartedAt.
	Offset time.Duration
	Role   EventRole
}

// ConfidenceBreakdown holds the normalized [0,1] components whose weighted
// sum is the confidence. Weights are in Config.Weights.
type ConfidenceBreakdown struct {
	SignalStrength      float64
	Persistence         float64
	MultivariateSupport float64
	EventContext        float64
	PatternSupport      float64
}

// MeterSummary describes how one meter was analyzed.
type MeterSummary struct {
	MeterID  string
	Readings int
	// EvaluatedReadings had a mature baseline.
	EvaluatedReadings int
	// FlaggedReadings had at least one signal.
	FlaggedReadings int
	Findings        int
	Status          MeterStatus
}
