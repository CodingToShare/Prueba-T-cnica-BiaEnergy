package analysis

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

// ErrInvalidInput marks structurally invalid readings or events.
var ErrInvalidInput = errors.New("invalid analysis input")

// Engine runs the analysis with a fixed configuration. It is safe for
// concurrent use because it holds no mutable state.
type Engine struct {
	cfg Config
}

// New returns an engine for a validated configuration.
func New(cfg Config) (*Engine, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Engine{cfg: cfg}, nil
}

// Analyze classifies the readings of every meter, correlating the events.
// The inputs are neither modified nor retained, and their order does not
// matter. It fails on duplicate or non-finite readings, malformed events, an
// empty reading set, or a cancelled context.
func (e *Engine) Analyze(ctx context.Context, readings []Reading, events []Event) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	meters, err := groupReadings(readings, e.cfg.ReadingInterval)
	if err != nil {
		return Result{}, err
	}
	if err := validateEvents(events); err != nil {
		return Result{}, err
	}
	index := newEventIndex(events)

	result := Result{Meters: make([]MeterSummary, 0, len(meters))}
	for _, series := range meters {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		meterID := series[0].MeterID
		evals := e.evaluate(series)
		summary := MeterSummary{MeterID: meterID, Readings: len(series)}
		for _, ev := range evals {
			if ev.evaluated {
				summary.EvaluatedReadings++
			}
			if ev.flagged {
				summary.FlaggedReadings++
			}
		}
		var meterFindings []Finding
		for _, ep := range e.buildEpisodes(evals) {
			if f, ok := e.assess(meterID, evals, ep, index); ok {
				meterFindings = append(meterFindings, f)
			}
		}
		summary.Findings = len(meterFindings)
		summary.Status = meterStatus(meterFindings)
		result.Meters = append(result.Meters, summary)
		result.Findings = append(result.Findings, meterFindings...)
	}
	prioritize(result.Findings)
	return result, nil
}

// meterStatus derives the computed meter status from its findings (OD-10):
// CRITICAL for a HIGH real anomaly; ALERT for any other real anomaly or any
// finding of MEDIUM or HIGH severity; otherwise OK.
func meterStatus(findings []Finding) MeterStatus {
	status := StatusOK
	for _, f := range findings {
		switch {
		case f.Type == RealAnomaly && f.Severity == SeverityHigh:
			return StatusCritical
		case f.Type == RealAnomaly || f.Severity != SeverityLow:
			status = StatusAlert
		}
	}
	return status
}

// groupReadings validates and copies the readings, sorted by meter and time,
// one slice per meter. Readings of a meter must be distinct and a whole
// number of intervals apart (gaps are allowed), because baselines, durations
// and runs are counted in intervals.
func groupReadings(readings []Reading, interval time.Duration) ([][]Reading, error) {
	if len(readings) == 0 {
		return nil, fmt.Errorf("%w: no readings", ErrInvalidInput)
	}
	sorted := slices.Clone(readings)
	for i, r := range sorted {
		switch {
		case strings.TrimSpace(r.MeterID) == "":
			return nil, fmt.Errorf("%w: reading %d has no meter ID", ErrInvalidInput, i)
		case r.Timestamp.IsZero():
			return nil, fmt.Errorf("%w: reading %d of meter %s has no timestamp", ErrInvalidInput, i, r.MeterID)
		case !allFinite(r.ConsumptionKWh, r.VoltageV, r.CurrentA, r.PowerFactor):
			return nil, fmt.Errorf("%w: reading of meter %s at %s has a non-finite value",
				ErrInvalidInput, r.MeterID, r.Timestamp.Format(reasonTimeLayout))
		}
	}
	slices.SortFunc(sorted, func(a, b Reading) int {
		return cmp.Or(cmp.Compare(a.MeterID, b.MeterID), a.Timestamp.Compare(b.Timestamp))
	})

	var groups [][]Reading
	start := 0
	for i := 1; i <= len(sorted); i++ {
		if i < len(sorted) && sorted[i].MeterID == sorted[start].MeterID {
			switch step := sorted[i].Timestamp.Sub(sorted[i-1].Timestamp); {
			case step == 0:
				return nil, fmt.Errorf("%w: duplicate reading for meter %s at %s",
					ErrInvalidInput, sorted[i].MeterID, sorted[i].Timestamp.Format(reasonTimeLayout))
			case step%interval != 0:
				return nil, fmt.Errorf("%w: reading of meter %s at %s is not a whole %s after the previous one",
					ErrInvalidInput, sorted[i].MeterID, sorted[i].Timestamp.Format(reasonTimeLayout), interval)
			}
			continue
		}
		groups = append(groups, sorted[start:i:i])
		start = i
	}
	return groups, nil
}

func validateEvents(events []Event) error {
	for i, ev := range events {
		switch {
		case strings.TrimSpace(ev.MeterID) == "":
			return fmt.Errorf("%w: event %d has no meter ID", ErrInvalidInput, i)
		case ev.Timestamp.IsZero():
			return fmt.Errorf("%w: event %d of meter %s has no timestamp", ErrInvalidInput, i, ev.MeterID)
		case strings.TrimSpace(string(ev.Type)) == "":
			return fmt.Errorf("%w: event %d of meter %s has no type", ErrInvalidInput, i, ev.MeterID)
		}
	}
	return nil
}

func allFinite(values ...float64) bool {
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}
