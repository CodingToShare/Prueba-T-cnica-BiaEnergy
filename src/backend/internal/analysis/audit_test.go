package analysis

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAudit_RecoveryRequiresUnbrokenEvaluatedHours(t *testing.T) {
	for _, mode := range []string{"missing hour", "unevaluated hour", "gap before recovery"} {
		t.Run(mode, func(t *testing.T) {
			r := temporaryDrop("AUDIT-RECOVERY")
			end := onsetB.Add(10 * time.Hour)
			r = r[:indexOf(t, r, end.Add(7*time.Hour))]
			if mode == "missing hour" || mode == "gap before recovery" {
				missing := end.Add(3 * time.Hour)
				if mode == "gap before recovery" {
					missing = end
				}
				r = slices.DeleteFunc(r, func(v Reading) bool { return v.Timestamp.Equal(missing) })
			}
			evals, eps := episodesOf(t, r)
			require.Len(t, eps, 1)
			if mode == "unevaluated hour" {
				evals[indexOf(t, r, end.Add(3*time.Hour))].evaluated = false
			}
			f, ok := newEngine(t).assess("AUDIT-RECOVERY", evals, eps[0], newEventIndex([]Event{{MeterID: "AUDIT-RECOVERY", Timestamp: onsetB, Type: EventScheduledOutage}}))
			require.True(t, ok)
			assert.False(t, f.Persistence.Recovery.Recovered)
			assert.Equal(t, ExplainableAnomaly, f.Type, "missing evidence cannot establish a recovered false positive")
		})
	}
}

func TestAudit_RatioRejectsOverflowingDenominator(t *testing.T) {
	f := featuresOf(Reading{ConsumptionKWh: 40, VoltageV: math.MaxFloat64, CurrentA: 2, PowerFactor: 1})
	assert.False(t, f.available[idxLoadRatio], "an infinite denominator must not manufacture a finite zero ratio")
}

func TestAudit_DerivedRatioIsNotAnIndependentConfidenceVote(t *testing.T) {
	r := stableSeries("AUDIT-RATIO", 10, 40)
	modify(r, at(8, 0), at(10, 0), func(v *Reading) { v.CurrentA *= 0.5 })
	f := onlyFinding(t, analyze(t, r, nil))
	assert.Equal(t, DataQuality, f.Type)
	assert.True(t, metric(f, MetricLoadRatio).Corroborates, "retain relationship evidence")
	assert.InDelta(t, 1.0/3, f.ConfidenceDetail.MultivariateSupport, 1e-12, "one measured variable and its derived ratio are one vote")
}

func TestAudit_ElectricalChangesWithoutFollowingCurrentDoNotConfirmLoad(t *testing.T) {
	r := stableSeries("AUDIT-LOAD", 10, 40)
	modify(r, at(8, 0), at(10, 0), func(v *Reading) { v.ConsumptionKWh *= 2; v.PowerFactor *= 0.8; v.VoltageV *= 1.1 })
	f := onlyFinding(t, analyze(t, r, nil))
	assert.Equal(t, SeverityMedium, f.Severity)
	assert.Zero(t, f.ConfidenceDetail.MultivariateSupport)
	assert.Positive(t, metric(f, MetricLoadRatio).TriggeredReadings)
}

func TestAudit_BorderlineFlagDoesNotCauseBaselineFeedback(t *testing.T) {
	r := stableSeries("AUDIT-FEEDBACK", 25, 40)
	for i := range r {
		r[i].ConsumptionKWh = 100
		r[i].CurrentA = 100
	}
	modify(r, at(8, 5), at(8, 6), func(v *Reading) { v.ConsumptionKWh = 125.01; v.CurrentA = 125.01 })
	evals := evaluateSeries(t, r)
	flagged := 0
	for _, ev := range evals {
		if ev.flagged {
			flagged++
		}
		if ev.evaluated {
			assert.Equal(t, 100.0, ev.dev[idxConsumption].baseline)
		}
	}
	assert.Equal(t, 1, flagged)
	assert.Empty(t, analyze(t, r, nil).Findings)
}

func TestAudit_PermanentExplainedShiftKeepsHistoricalBaseline(t *testing.T) {
	r := stableSeries("AUDIT-SHIFT", 25, 40)
	modify(r, at(8, 0), at(25, 0), scaleLoad(1.6))
	f := onlyFinding(t, analyze(t, r, []Event{{MeterID: "AUDIT-SHIFT", Timestamp: at(8, 0), Type: EventOperationalChange, Description: "Fictional process adjustment"}}))
	assert.Equal(t, ExplainableAnomaly, f.Type)
	assert.Equal(t, 17*24, f.Persistence.FlaggedReadings)
	assert.False(t, f.Persistence.Recovery.Recovered)
}

func TestAudit_DualGateRejectsLargeChangeWithinNoisyHistory(t *testing.T) {
	r := stableSeries("AUDIT-NOISE", 8, 40)
	history := []float64{60, 70, 80, 100, 120, 130, 140}
	for i := range r {
		if i/24 < 7 {
			r[i].ConsumptionKWh = history[i/24]
		} else {
			r[i].ConsumptionKWh = 140
		}
	}
	ev := evaluateSeries(t, r)[7*24]
	assert.InDelta(t, 0.4, ev.dev[idxConsumption].pct, 1e-12)
	assert.Less(t, math.Abs(ev.dev[idxConsumption].robustZ), 3.5)
	assert.False(t, ev.triggered[idxConsumption])
}

func TestAudit_EpisodeExactBoundaries(t *testing.T) {
	for _, gap := range []int{2, 3, 4} {
		t.Run((time.Duration(gap) * time.Hour).String(), func(t *testing.T) {
			r := stableSeries("AUDIT-GAP", 10, 40)
			shiftHours(r, 1.6, 0, gap, 2*gap)
			_, eps := episodesOf(t, r)
			if gap <= 3 {
				require.Len(t, eps, 1)
			} else {
				require.Len(t, eps, 3)
			}
		})
	}
	for _, n := range []int{2, 3, 11, 12} {
		t.Run("length-"+(time.Duration(n)*time.Hour).String(), func(t *testing.T) {
			r := stableSeries("AUDIT-LENGTH", 10, 40)
			modify(r, at(8, 0), at(8, n), scaleLoad(1.6))
			result := analyze(t, r, nil)
			if n < 3 {
				assert.Empty(t, result.Findings)
				return
			}
			f := onlyFinding(t, result)
			assert.Equal(t, n >= 12, f.Persistence.Sustained)
			assert.Equal(t, time.Duration(n)*time.Hour, f.Duration)
		})
	}
	for _, n := range []int{5, 6} {
		t.Run("recovery-"+(time.Duration(n)*time.Hour).String(), func(t *testing.T) {
			r := temporaryDrop("AUDIT-COUNT")
			end := onsetB.Add(time.Duration(10+n) * time.Hour)
			r = r[:indexOf(t, r, end)]
			f := onlyFinding(t, analyze(t, r, nil))
			assert.Equal(t, n == 6, f.Persistence.Recovery.Recovered)
			assert.Equal(t, n, f.Persistence.Recovery.ReadingsObserved)
		})
	}
}

func TestAudit_MissingSamplesKeepElapsedDurationDistinctFromCount(t *testing.T) {
	r := stableSeries("AUDIT-SPARSE", 10, 40)
	modify(r, at(8, 0), at(8, 13), scaleLoad(1.6))
	r = slices.DeleteFunc(r, func(v Reading) bool {
		return !v.Timestamp.Before(at(8, 0)) && v.Timestamp.Before(at(8, 13)) && v.Timestamp.Hour()%3 != 0
	})
	f := onlyFinding(t, analyze(t, r, nil))
	assert.Equal(t, 13*time.Hour, f.Duration)
	assert.Equal(t, 5, f.Persistence.FlaggedReadings)
	assert.Equal(t, 5, f.Persistence.SpanReadings)
	assert.Equal(t, 1, f.Persistence.LongestRun)
	assert.True(t, f.Persistence.Sustained)
}

func TestAudit_EventWindowAndTiesAreDeterministic(t *testing.T) {
	r := temporaryDrop("AUDIT-EVENT")
	for _, h := range []int{-4, -3, 0, 3, 4} {
		f := onlyFinding(t, analyze(t, r, []Event{{MeterID: "AUDIT-EVENT", Timestamp: onsetB.Add(time.Duration(h) * time.Hour), Type: EventScheduledOutage, Description: "Arbitrary words; no duration"}}))
		if h >= -3 && h <= 3 {
			assert.Equal(t, FalsePositive, f.Type)
		} else {
			assert.Equal(t, RealAnomaly, f.Type)
		}
	}
	events := []Event{
		{MeterID: "AUDIT-EVENT", Timestamp: onsetB.Add(-time.Hour), Type: EventOperationalChange, Description: "z"},
		{MeterID: "AUDIT-EVENT", Timestamp: onsetB.Add(time.Hour), Type: EventScheduledOutage, Description: "a"},
		{MeterID: "AUDIT-EVENT", Timestamp: onsetB.Add(-time.Hour), Type: EventOperationalChange, Description: "a"},
	}
	want := analyze(t, r, events)
	slices.Reverse(events)
	assert.Equal(t, want, analyze(t, r, events))
	assert.Equal(t, ExplainableAnomaly, onlyFinding(t, want).Type, "earlier event wins equal distance")
}

func TestAudit_ProportionalLoadKeepsRatioStable(t *testing.T) {
	r := stableSeries("AUDIT-PROPORTION", 10, 40)
	modify(r, at(8, 0), at(10, 0), scaleLoad(1.8))
	f := onlyFinding(t, analyze(t, r, nil))
	assert.True(t, metric(f, MetricCurrent).Corroborates)
	assert.Zero(t, metric(f, MetricLoadRatio).TriggeredReadings)
	assert.False(t, metric(f, MetricLoadRatio).Corroborates)
}

func TestAudit_ModerateSingleElectricalDriftIsNotReportable(t *testing.T) {
	r := stableSeries("AUDIT-PF", 10, 40)
	modify(r, at(8, 0), at(10, 0), func(v *Reading) { v.PowerFactor *= 0.85 })
	result := analyze(t, r, nil)
	assert.Empty(t, result.Findings)
	assert.Positive(t, result.Meters[0].FlaggedReadings)
}

func TestAudit_ConfidenceHasNoSustainedBoundaryCliff(t *testing.T) {
	e := newEngine(t)
	a := finding(RealAnomaly, false, 0.8, 11, 2)
	b := a
	b.Persistence.FlaggedReadings = 12
	b.Persistence.Sustained = true
	ca := e.cfg.Weights.combine(e.confidenceBreakdown(a, 1))
	cb := e.cfg.Weights.combine(e.confidenceBreakdown(b, 1))
	assert.GreaterOrEqual(t, cb, ca)
	assert.Less(t, cb-ca, 0.01)
}

func TestAudit_MeterIsolationAndHighestStatus(t *testing.T) {
	a := sustainedRise("AUDIT-A")
	b := stableSeries("AUDIT-B", 14, 4000)
	alone := analyze(t, a, nil)
	together := analyze(t, append(slices.Clone(a), b...), nil)
	assert.Equal(t, alone.Findings, together.Findings)
	for _, fs := range [][]Finding{
		{{Type: DataQuality, Severity: SeverityHigh}, {Type: RealAnomaly, Severity: SeverityHigh}},
		{{Type: RealAnomaly, Severity: SeverityHigh}, {Type: FalsePositive, Severity: SeverityLow}},
	} {
		assert.Equal(t, StatusCritical, meterStatus(fs))
	}
}

func TestAudit_UnavailableBaselinesNeverBecomeZeroEvidence(t *testing.T) {
	t.Run("zero consumption baseline", func(t *testing.T) {
		r := stableSeries("AUDIT-ZERO", 10, 40)
		for i := range r {
			r[i].ConsumptionKWh = 0
		}
		result := analyze(t, r, nil)
		assert.Empty(t, result.Findings)
		assert.Zero(t, result.Meters[0].EvaluatedReadings)
		assert.Zero(t, result.Meters[0].FlaggedReadings)
	})
	t.Run("insufficient valid ratio history", func(t *testing.T) {
		r := stableSeries("AUDIT-UNAVAILABLE", 10, 40)
		modify(r, at(6, 0), at(7, 0), func(v *Reading) { v.CurrentA = 0 })
		modify(r, at(7, 0), at(10, 0), scaleLoad(1.8))
		f := onlyFinding(t, analyze(t, r, nil))
		assert.Equal(t, RealAnomaly, f.Type)
		assert.Zero(t, metric(f, MetricLoadRatio).EvaluatedReadings)
		assert.False(t, metric(f, MetricLoadRatio).Corroborates)
		for _, s := range f.Signals {
			assert.NotEqual(t, MetricLoadRatio, s.Metric)
		}
	})
}
