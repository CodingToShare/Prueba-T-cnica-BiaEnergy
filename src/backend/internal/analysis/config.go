package analysis

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// Config holds every threshold and weight of the engine. Defaults and their
// calibration rationale are documented in docs/ai/anomaly-analysis.md.
// A Config is copied into the Engine; there is no package-level mutable state.
// Its JSON form (durations in nanoseconds) is the configuration snapshot
// persisted with every analysis run.
type Config struct {
	// HistoryObservations is both the size and the minimum of a baseline: the
	// most recent same-meter, same-hour-of-day readings that were not flagged.
	HistoryObservations int `json:"history_observations"`
	// MinSpreadFraction floors the MAD at this fraction of |median| so a
	// near-constant history cannot divide by zero.
	MinSpreadFraction float64 `json:"min_spread_fraction"`
	// MinRobustZ is the statistical gate every signal must pass.
	MinRobustZ float64 `json:"min_robust_z"`
	// MinDeviation is the relative-deviation gate every signal must pass.
	MinDeviation DeviationThresholds `json:"min_deviation"`

	// ReadingInterval is the duration one reading covers.
	ReadingInterval time.Duration `json:"reading_interval_ns"`
	// MinInconsistentMetrics is how many electrical metrics must deviate,
	// while consumption does not, for a reading to be inconsistent.
	MinInconsistentMetrics int `json:"min_inconsistent_metrics"`

	// MaxGap is the longest time between two flagged readings of one episode.
	MaxGap time.Duration `json:"max_gap_ns"`
	// MinEpisodeReadings is the fewest flagged readings a reportable episode has.
	MinEpisodeReadings int `json:"min_episode_readings"`
	// RecoveryReadings is how many consecutive readings without the episode's
	// signal prove a recovery.
	RecoveryReadings int `json:"recovery_readings"`
	// SustainedDuration is the duration from which an episode is sustained.
	SustainedDuration time.Duration `json:"sustained_duration_ns"`
	// CorroborationShare is the share of flagged readings in which an
	// electrical metric must deviate to corroborate a finding.
	CorroborationShare float64 `json:"corroboration_share"`

	// EventWindow is the maximum distance between an event and an episode's
	// onset for the event to be correlated.
	EventWindow time.Duration `json:"event_window_ns"`

	// HighDeviation is the consumption deviation from which an unexplained
	// anomaly can be HIGH severity.
	HighDeviation float64 `json:"high_deviation"`

	// SignalSaturation is the evidence strength (multiple of the threshold)
	// at which the signal-strength component reaches 1.
	SignalSaturation float64 `json:"signal_saturation"`
	// FullPersistenceReadings is the flagged-reading count at which the
	// persistence component reaches 1.
	FullPersistenceReadings int `json:"full_persistence_readings"`
	// FullMultivariateMetrics is the corroborating-metric count at which the
	// multivariate component reaches 1.
	FullMultivariateMetrics int `json:"full_multivariate_metrics"`
	// Weights combine the confidence components; they sum to 1.
	Weights ConfidenceWeights `json:"confidence_weights"`
}

// DeviationThresholds are minimum |relative deviations| from the baseline.
type DeviationThresholds struct {
	Consumption float64 `json:"consumption"`
	Voltage     float64 `json:"voltage"`
	Current     float64 `json:"current"`
	PowerFactor float64 `json:"power_factor"`
	LoadRatio   float64 `json:"load_ratio"`
}

// ConfidenceWeights weight the ConfidenceBreakdown components.
type ConfidenceWeights struct {
	SignalStrength      float64 `json:"signal_strength"`
	Persistence         float64 `json:"persistence"`
	MultivariateSupport float64 `json:"multivariate_support"`
	EventContext        float64 `json:"event_context"`
	PatternSupport      float64 `json:"pattern_support"`
}

// EngineVersion identifies the analysis algorithm. Change it whenever the
// algorithm or the meaning of a configuration value changes, so persisted
// runs record which policy produced them.
const EngineVersion = "1.0.0"

// DefaultConfig returns the calibrated defaults.
func DefaultConfig() Config {
	return Config{
		HistoryObservations: 7,
		MinSpreadFraction:   0.001,
		MinRobustZ:          3.5,
		MinDeviation: DeviationThresholds{
			Consumption: 0.25,
			Voltage:     0.03,
			Current:     0.20,
			PowerFactor: 0.08,
			LoadRatio:   0.40,
		},
		ReadingInterval:         time.Hour,
		MinInconsistentMetrics:  2,
		MaxGap:                  3 * time.Hour,
		MinEpisodeReadings:      3,
		RecoveryReadings:        6,
		SustainedDuration:       12 * time.Hour,
		CorroborationShare:      0.5,
		EventWindow:             3 * time.Hour,
		HighDeviation:           0.50,
		SignalSaturation:        3,
		FullPersistenceReadings: 24,
		FullMultivariateMetrics: 3,
		Weights: ConfidenceWeights{
			SignalStrength:      0.25,
			Persistence:         0.20,
			MultivariateSupport: 0.20,
			EventContext:        0.20,
			PatternSupport:      0.15,
		},
	}
}

// Validate reports every inconsistent setting.
func (c Config) Validate() error {
	var errs []error
	check := func(ok bool, format string, args ...any) {
		if !ok {
			errs = append(errs, fmt.Errorf(format, args...))
		}
	}
	positive := func(v float64) bool { return v > 0 && !math.IsInf(v, 0) }

	check(c.HistoryObservations >= 3, "HistoryObservations must be at least 3")
	check(c.MinSpreadFraction > 0 && c.MinSpreadFraction < 1, "MinSpreadFraction must be in (0, 1)")
	check(positive(c.MinRobustZ), "MinRobustZ must be positive")
	for m := range metricCount {
		check(positive(c.threshold(m)), "MinDeviation for %s must be positive", metricNames[m])
	}
	check(c.ReadingInterval > 0, "ReadingInterval must be positive")
	check(c.MinInconsistentMetrics >= 1 && c.MinInconsistentMetrics <= len(electricalMetrics),
		"MinInconsistentMetrics must be between 1 and %d", len(electricalMetrics))
	check(c.MaxGap >= c.ReadingInterval, "MaxGap must be at least ReadingInterval")
	check(c.MinEpisodeReadings >= 1, "MinEpisodeReadings must be at least 1")
	check(c.RecoveryReadings >= 1 && time.Duration(c.RecoveryReadings)*c.ReadingInterval > c.MaxGap,
		"RecoveryReadings must cover more than MaxGap")
	check(c.SustainedDuration > 0, "SustainedDuration must be positive")
	check(c.CorroborationShare > 0 && c.CorroborationShare <= 1, "CorroborationShare must be in (0, 1]")
	check(c.EventWindow >= 0, "EventWindow must not be negative")
	check(positive(c.HighDeviation), "HighDeviation must be positive")
	check(c.SignalSaturation > 1 && !math.IsInf(c.SignalSaturation, 0), "SignalSaturation must be greater than 1")
	check(c.FullPersistenceReadings >= 1, "FullPersistenceReadings must be at least 1")
	check(c.FullMultivariateMetrics >= 1, "FullMultivariateMetrics must be at least 1")

	w := c.Weights
	weights := []float64{w.SignalStrength, w.Persistence, w.MultivariateSupport, w.EventContext, w.PatternSupport}
	sum := 0.0
	for _, v := range weights {
		check(v >= 0 && !math.IsInf(v, 0) && !math.IsNaN(v), "confidence weights must be finite and not negative")
		sum += v
	}
	check(math.Abs(sum-1) < 1e-9, "confidence weights must sum to 1, got %g", sum)

	if len(errs) > 0 {
		return fmt.Errorf("invalid analysis config: %w", errors.Join(errs...))
	}
	return nil
}

// threshold returns the relative-deviation gate of a metric.
func (c Config) threshold(m metricIndex) float64 {
	switch m {
	case idxConsumption:
		return c.MinDeviation.Consumption
	case idxVoltage:
		return c.MinDeviation.Voltage
	case idxCurrent:
		return c.MinDeviation.Current
	case idxPowerFactor:
		return c.MinDeviation.PowerFactor
	default:
		return c.MinDeviation.LoadRatio
	}
}
