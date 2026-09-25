package explanation

import (
	"time"

	"bia-energy.local/backend/internal/analysisrun"
	"bia-energy.local/backend/internal/platform/jsontime"
)

// Fictional findings (SYN-* meters, year 2030) shaped like the engine's
// evidence; no challenge meter or dataset value is needed.

func at(day, hour int) time.Time { return time.Date(2030, 1, day, hour, 0, 0, 0, time.UTC) }

func up() *string   { s := "UP"; return &s }
func down() *string { s := "DOWN"; return &s }

func baseInput() analysisrun.ExplanationInput {
	return analysisrun.ExplanationInput{
		MeterID: "SYN-1", Type: "REAL_ANOMALY", Severity: "HIGH", Confidence: 0.884, Priority: 1,
		RecommendedAction: "INVESTIGATE_METER_AND_INSTALLATION",
		Reason:            "Consumption was 95% above its hourly baseline for 58 h from 2030-01-12 14:00; no explanatory event.",
		StartedAt:         at(12, 14), LastObservedAt: at(14, 23),
		Evidence: analysisrun.Evidence{
			SchemaVersion: 1, Rule: "UNEXPLAINED_PERSISTENT_CONSUMPTION_SHIFT", EvidenceStrength: 3.8,
			Consumption: analysisrun.ConsumptionEvidence{BaselineKWh: 2000, ObservedKWh: 3904, DeviationPct: 95.2, MedianDeviationPct: 94.1, Direction: "UP"},
			Persistence: analysisrun.PersistenceEvidence{DurationHours: 58, FlaggedReadings: 58, SpanReadings: 58, DensityFraction: 1, LongestRun: 58, Sustained: true},
			Metrics: []analysisrun.MetricEvidence{
				{Metric: "consumption_kwh", EvaluatedReadings: 58, TriggeredReadings: 58, MedianDeviationPct: 94.1, MaxAbsDeviationPct: 120, Direction: up()},
				{Metric: "voltage_v", EvaluatedReadings: 58, MedianDeviationPct: -0.4, MaxAbsDeviationPct: 1.5},
				{Metric: "current_a", EvaluatedReadings: 58, TriggeredReadings: 58, MedianDeviationPct: 100.4, MaxAbsDeviationPct: 110, Direction: up(), Corroborates: true},
				{Metric: "power_factor", EvaluatedReadings: 58, TriggeredReadings: 50, MedianDeviationPct: -20.3, MaxAbsDeviationPct: 24, Direction: down(), Corroborates: true},
			},
			RelatedEvents: []analysisrun.RelatedEvent{
				{Timestamp: jsontime.Source(at(12, 14)), Type: "UNKNOWN", Description: "Unclassified note", OffsetSeconds: 0, Role: "CONTEXT"},
			},
			ConfidenceBreakdown: analysisrun.ConfidenceBreakdown{SignalStrength: 1, Persistence: 1, MultivariateSupport: 0.667, EventContext: 1, PatternSupport: 1},
		},
	}
}

func explainableInput() analysisrun.ExplanationInput {
	in := baseInput()
	in.MeterID, in.Type, in.Severity, in.Confidence, in.Priority = "SYN-2", "EXPLAINABLE_ANOMALY", "MEDIUM", 0.73, 3
	in.RecommendedAction = "VALIDATE_OPERATIONAL_CHANGE"
	in.Evidence.RelatedEvents = []analysisrun.RelatedEvent{
		{Timestamp: jsontime.Source(at(12, 11)), Type: "OPERATIONAL_CHANGE", Description: "New production line", OffsetSeconds: -3 * 3600, Role: "EXPLAINS"},
	}
	return in
}

func falsePositiveInput() analysisrun.ExplanationInput {
	in := baseInput()
	in.MeterID, in.Type, in.Severity, in.Confidence, in.Priority = "SYN-3", "FALSE_POSITIVE", "LOW", 0.76, 4
	in.RecommendedAction = "NO_ESCALATION_MONITOR"
	in.Evidence.Consumption.Direction = "DOWN"
	in.Evidence.Consumption.DeviationPct = -79.7
	in.Evidence.Persistence = analysisrun.PersistenceEvidence{
		DurationHours: 6, FlaggedReadings: 6, SpanReadings: 6, DensityFraction: 1, LongestRun: 6,
		Recovery: analysisrun.RecoveryEvidence{Recovered: true, RecoveredAt: jsontime.SourcePtr(ptr(at(13, 2))), ReadingsObserved: 6, MedianConsumptionDeviationPct: 1.2},
	}
	in.Evidence.RelatedEvents = []analysisrun.RelatedEvent{
		{Timestamp: jsontime.Source(at(12, 20)), Type: "SCHEDULED_OUTAGE", Description: "Planned maintenance", OffsetSeconds: 0, Role: "EXPLAINS"},
	}
	return in
}

func dataQualityInput() analysisrun.ExplanationInput {
	in := baseInput()
	in.MeterID, in.Type, in.Severity, in.Confidence, in.Priority = "SYN-4", "DATA_QUALITY", "HIGH", 0.92, 2
	in.RecommendedAction = "VALIDATE_MEASUREMENT_OR_SENSOR"
	in.Evidence.Rule = "REPEATED_ELECTRICAL_INCONSISTENCY"
	in.Evidence.Consumption = analysisrun.ConsumptionEvidence{BaselineKWh: 430.8, ObservedKWh: 437.9, DeviationPct: 1.6, MedianDeviationPct: 2.7, Direction: "UP"}
	in.Evidence.Persistence = analysisrun.PersistenceEvidence{DurationHours: 46, FlaggedReadings: 16, SpanReadings: 46, DensityFraction: 0.35, LongestRun: 4, Sustained: true}
	in.Evidence.Metrics = []analysisrun.MetricEvidence{
		{Metric: "consumption_kwh", EvaluatedReadings: 16, MedianDeviationPct: 2.7, MaxAbsDeviationPct: 9},
		{Metric: "voltage_v", EvaluatedReadings: 16, TriggeredReadings: 16, MedianDeviationPct: 0.3, MaxAbsDeviationPct: 12, Direction: up(), Corroborates: true},
		{Metric: "current_a", EvaluatedReadings: 16, TriggeredReadings: 16, MedianDeviationPct: 0.4, MaxAbsDeviationPct: 40, Direction: up(), Corroborates: true},
	}
	in.Evidence.RelatedEvents = []analysisrun.RelatedEvent{
		{Timestamp: jsontime.Source(at(13, 0)), Type: "DATA_QUALITY", Description: "Intermittent readings", OffsetSeconds: 0, Role: "CORROBORATES"},
	}
	return in
}

func ptr[T any](v T) *T { return &v }
