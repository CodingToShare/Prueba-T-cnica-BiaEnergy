package analysis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Generalized acceptance scenarios on synthetic data with fictional meters,
// so that success does not depend on the supplied dataset.

const days = 14

// onsetB is the onset used by the shift scenarios: day 10, 10:00.
var onsetB = at(9, 10)

func analyze(t *testing.T, readings []Reading, events []Event) Result {
	t.Helper()
	res, err := newEngine(t).Analyze(context.Background(), readings, events)
	require.NoError(t, err)
	return res
}

// sustainedRise: +80% consumption with current following and power factor
// degrading, from onsetB to the end of the data.
func sustainedRise(meterID string) []Reading {
	r := stableSeries(meterID, days, 40)
	modify(r, onsetB, at(days, 0), func(x *Reading) {
		x.ConsumptionKWh *= 1.8
		x.CurrentA *= 1.8 / 0.8
		x.PowerFactor *= 0.8
	})
	return r
}

// temporaryDrop: −80% load for 10 hours from onsetB, then back to normal.
func temporaryDrop(meterID string) []Reading {
	r := stableSeries(meterID, days, 40)
	modify(r, onsetB, onsetB.Add(10*time.Hour), scaleLoad(0.2))
	return r
}

// intermittentInconsistency: from day 11, every third hour voltage jumps,
// current halves and power factor collapses while consumption is unchanged.
func intermittentInconsistency(meterID string) []Reading {
	r := stableSeries(meterID, days, 40)
	for i := range r {
		if ts := r[i].Timestamp; !ts.Before(at(11, 0)) && ts.Hour()%3 == 0 {
			r[i].VoltageV *= 1.08
			r[i].CurrentA *= 0.5
			r[i].PowerFactor *= 0.7
		}
	}
	return r
}

func onlyFinding(t *testing.T, res Result) Finding {
	t.Helper()
	require.Len(t, res.Findings, 1)
	return res.Findings[0]
}

func TestScenarioA_StableMeter_NoFinding(t *testing.T) {
	res := analyze(t, stableSeries("SYN-A", days, 40), nil)

	assert.Empty(t, res.Findings)
	require.Len(t, res.Meters, 1)
	assert.Equal(t, MeterSummary{MeterID: "SYN-A", Readings: days * 24, EvaluatedReadings: (days - 7) * 24, Status: StatusOK}, res.Meters[0])
}

func TestScenarioB_SustainedRiseWithElectricalSupportAndNoEvent_IsHighRealAnomaly(t *testing.T) {
	f := onlyFinding(t, analyze(t, sustainedRise("SYN-B"), nil))

	assert.Equal(t, RealAnomaly, f.Type)
	assert.Equal(t, RuleUnexplainedConsumptionShift, f.Rule)
	assert.Equal(t, SeverityHigh, f.Severity)
	assert.Equal(t, ActionInvestigateMeterAndInstallation, f.RecommendedAction)
	assert.Equal(t, onsetB, f.StartedAt)
	assert.InDelta(t, 0.8, f.Consumption.MedianDeviationPct, 0.05)
	assert.True(t, metric(f, MetricCurrent).Corroborates)
	assert.True(t, metric(f, MetricPowerFactor).Corroborates)
	assert.GreaterOrEqual(t, f.Confidence, 0.85)
	assert.False(t, f.Persistence.Recovery.Recovered)
}

func TestScenarioC_SameRiseWithOperationalChange_IsExplainable(t *testing.T) {
	events := []Event{{MeterID: "SYN-C", Timestamp: onsetB.Add(-30 * time.Minute), Type: EventOperationalChange, Description: "any text"}}

	f := onlyFinding(t, analyze(t, sustainedRise("SYN-C"), events))

	assert.Equal(t, ExplainableAnomaly, f.Type)
	assert.Equal(t, SeverityMedium, f.Severity)
	assert.Equal(t, ActionValidateOperationalChange, f.RecommendedAction)
	assert.InDelta(t, 0.8, f.Consumption.MedianDeviationPct, 0.05, "the deviation stays visible")
	require.Len(t, f.RelatedEvents, 1)
	assert.Equal(t, RoleExplains, f.RelatedEvents[0].Role)
	assert.Equal(t, -30*time.Minute, f.RelatedEvents[0].Offset)
}

func TestScenarioD_TemporaryDropWithScheduledOutageAndRecovery_IsLowFalsePositive(t *testing.T) {
	events := []Event{{MeterID: "SYN-D", Timestamp: onsetB, Type: EventScheduledOutage, Description: "any text"}}

	f := onlyFinding(t, analyze(t, temporaryDrop("SYN-D"), events))

	assert.Equal(t, FalsePositive, f.Type)
	assert.Equal(t, RuleScheduledOutageWithRecovery, f.Rule)
	assert.Equal(t, SeverityLow, f.Severity)
	assert.Equal(t, ActionNoEscalationMonitor, f.RecommendedAction)
	assert.InDelta(t, -0.8, f.Consumption.MedianDeviationPct, 0.05, "the detector saw the deviation")
	assert.Equal(t, 10*time.Hour, f.Duration)
	assert.True(t, f.Persistence.Recovery.Recovered)
	assert.Equal(t, onsetB.Add(10*time.Hour), f.Persistence.Recovery.RecoveredAt)
	assert.GreaterOrEqual(t, f.Confidence, 0.6, "a strong explanation gives confidence despite LOW severity")
}

func TestScenarioD_WithoutTheOutageEvent_IsNotAFalsePositive(t *testing.T) {
	f := onlyFinding(t, analyze(t, temporaryDrop("SYN-D"), nil))

	assert.Equal(t, RealAnomaly, f.Type, "event correlation is what makes it a false positive")
}

func TestScenarioD_OutageWithoutRecovery_StaysVisibleAsExplainable(t *testing.T) {
	r := stableSeries("SYN-D2", days, 40)
	modify(r, onsetB, at(days, 0), scaleLoad(0.2))
	events := []Event{{MeterID: "SYN-D2", Timestamp: onsetB, Type: EventScheduledOutage}}

	f := onlyFinding(t, analyze(t, r, events))

	assert.Equal(t, ExplainableAnomaly, f.Type)
	assert.Equal(t, RuleScheduledOutageNoRecovery, f.Rule)
	assert.Contains(t, f.Reason, "no recovery was observed")
}

func TestScenarioE_StableConsumptionWithRepeatedElectricalInconsistency_IsHighDataQuality(t *testing.T) {
	withEvent := onlyFinding(t, analyze(t, intermittentInconsistency("SYN-E"),
		[]Event{{MeterID: "SYN-E", Timestamp: at(11, 0), Type: EventDataQuality}}))
	withoutEvent := onlyFinding(t, analyze(t, intermittentInconsistency("SYN-E"), nil))

	for _, f := range []Finding{withEvent, withoutEvent} {
		assert.Equal(t, DataQuality, f.Type, "the readings alone are sufficient evidence")
		assert.Equal(t, SeverityHigh, f.Severity)
		assert.Equal(t, ActionValidateMeasurementOrSensor, f.RecommendedAction)
		assert.Less(t, math.Abs(f.Consumption.DeviationPct), 0.05, "consumption stays on its baseline")
		assert.Zero(t, metric(f, MetricConsumption).TriggeredReadings)
		assert.Equal(t, 24, f.Persistence.FlaggedReadings) // 8 per day on days 11–13
		assert.True(t, metric(f, MetricVoltage).Corroborates && metric(f, MetricCurrent).Corroborates)
	}
	assert.Greater(t, withEvent.Confidence, withoutEvent.Confidence, "the event corroborates")
	assert.Equal(t, RoleCorroborates, withEvent.RelatedEvents[0].Role)
}

func TestScenarioE_DataQualityEventAlone_CreatesNoFinding(t *testing.T) {
	events := []Event{{MeterID: "SYN-E2", Timestamp: at(10, 0), Type: EventDataQuality}}

	res := analyze(t, stableSeries("SYN-E2", days, 40), events)

	assert.Empty(t, res.Findings)
}

func TestScenarioF_IsolatedSpike_IsNotPromoted(t *testing.T) {
	r := stableSeries("SYN-F", days, 40)
	modify(r, onsetB, onsetB.Add(time.Hour), scaleLoad(1.3))

	res := analyze(t, r, nil)

	assert.Empty(t, res.Findings)
	assert.Equal(t, 1, res.Meters[0].FlaggedReadings, "the spike is detected, just not reportable")
	assert.Equal(t, StatusOK, res.Meters[0].Status)
}

func TestUnknownOrUnrecognizedEvent_DoesNotSuppressAStrongAnomaly(t *testing.T) {
	for _, ty := range []EventType{EventUnknown, "FIRMWARE_UPDATE"} {
		events := []Event{{MeterID: "SYN-U", Timestamp: onsetB, Type: ty, Description: "No operational event reported"}}

		f := onlyFinding(t, analyze(t, sustainedRise("SYN-U"), events))

		assert.Equal(t, RealAnomaly, f.Type, string(ty))
		assert.Equal(t, SeverityHigh, f.Severity)
		require.Len(t, f.RelatedEvents, 1, "the event is preserved as context")
		assert.Equal(t, RoleContext, f.RelatedEvents[0].Role)
		assert.Equal(t, 1.0, f.ConfidenceDetail.EventContext)
	}
}

func TestDistantOperationalChange_IsNotCorrelated(t *testing.T) {
	events := []Event{{MeterID: "SYN-G", Timestamp: onsetB.Add(-48 * time.Hour), Type: EventOperationalChange}}

	f := onlyFinding(t, analyze(t, sustainedRise("SYN-G"), events))

	assert.Equal(t, RealAnomaly, f.Type)
	assert.Empty(t, f.RelatedEvents)
}

func TestShortUncorroboratedUnexplainedShift_IsNotOverclassified(t *testing.T) {
	// Four hours of +35% consumption with no electrical metric following and
	// no event: persistent enough to group, too weak to call a real anomaly.
	r := stableSeries("SYN-W", days, 40)
	modify(r, onsetB, onsetB.Add(4*time.Hour), func(x *Reading) { x.ConsumptionKWh *= 1.35 })

	res := analyze(t, r, nil)

	assert.Empty(t, res.Findings)
	assert.Equal(t, 4, res.Meters[0].FlaggedReadings)
}

func TestConsumptionShiftContradictedByElectricalMetrics_IsNotCorroborated(t *testing.T) {
	// Consumption +80% while current, voltage and power factor stay flat: the
	// load ratio shifts because the electrical metrics did not follow, which
	// must not count as support for a HIGH real anomaly.
	r := stableSeries("SYN-X", days, 40)
	modify(r, onsetB, at(days, 0), func(x *Reading) { x.ConsumptionKWh *= 1.8 })

	f := onlyFinding(t, analyze(t, r, nil))

	ratio := metric(f, MetricLoadRatio)
	assert.Equal(t, f.Persistence.FlaggedReadings, ratio.TriggeredReadings)
	assert.False(t, ratio.Corroborates)
	assert.False(t, metric(f, MetricCurrent).Corroborates)
	assert.Equal(t, RealAnomaly, f.Type)
	assert.Equal(t, SeverityMedium, f.Severity, "sustained and large, but uncorroborated")
	assert.Zero(t, f.ConfidenceDetail.MultivariateSupport)
}

// mixedDataset combines every scenario on fictional meters.
func mixedDataset() ([]Reading, []Event) {
	var readings []Reading
	readings = append(readings, stableSeries("SYN-A", days, 40)...)
	readings = append(readings, sustainedRise("SYN-B")...)
	readings = append(readings, sustainedRise("SYN-C")...)
	readings = append(readings, temporaryDrop("SYN-D")...)
	readings = append(readings, intermittentInconsistency("SYN-E")...)
	events := []Event{
		{MeterID: "SYN-C", Timestamp: onsetB, Type: EventOperationalChange, Description: "c"},
		{MeterID: "SYN-D", Timestamp: onsetB, Type: EventScheduledOutage, Description: "d"},
		{MeterID: "SYN-E", Timestamp: at(11, 0), Type: EventDataQuality, Description: "e"},
		{MeterID: "SYN-B", Timestamp: onsetB, Type: EventUnknown, Description: "b"},
	}
	return readings, events
}

func TestMixedDataset_PrioritizesByEvidence(t *testing.T) {
	readings, events := mixedDataset()

	res := analyze(t, readings, events)

	var got []string
	for _, f := range res.Findings {
		got = append(got, f.MeterID+":"+string(f.Type)+"/"+string(f.Severity))
	}
	assert.Equal(t, []string{
		"SYN-B:REAL_ANOMALY/HIGH",
		"SYN-E:DATA_QUALITY/HIGH",
		"SYN-C:EXPLAINABLE_ANOMALY/MEDIUM",
		"SYN-D:FALSE_POSITIVE/LOW",
	}, got)
	statuses := map[string]MeterStatus{}
	for _, m := range res.Meters {
		statuses[m.MeterID] = m.Status
	}
	assert.Equal(t, map[string]MeterStatus{
		"SYN-A": StatusOK, "SYN-B": StatusCritical, "SYN-C": StatusAlert, "SYN-D": StatusOK, "SYN-E": StatusAlert,
	}, statuses)
}

func TestFindings_CarryCompleteEvidence(t *testing.T) {
	readings, events := mixedDataset()

	for _, f := range analyze(t, readings, events).Findings {
		assert.NotEmpty(t, f.Signals, f.MeterID)
		assert.Len(t, f.Metrics, int(metricCount), f.MeterID)
		assert.NotEmpty(t, f.Reason, f.MeterID)
		assert.Equal(t, recommendedAction(f.Type), f.RecommendedAction, f.MeterID)
		assert.True(t, f.Confidence > 0 && f.Confidence <= 1, f.MeterID)
		assert.Equal(t, f.StartedAt, f.Signals[0].Timestamp, f.MeterID)
		assert.Equal(t, f.LastObservedAt.Sub(f.StartedAt)+time.Hour, f.Duration, f.MeterID)
		for _, s := range f.Signals {
			assert.GreaterOrEqual(t, s.Strength, 1.0, "every signal passed its gate")
			assert.GreaterOrEqual(t, math.Abs(s.RobustZ), 3.5)
		}
	}
}

func TestReason_IsBuiltFromEvidence(t *testing.T) {
	f := onlyFinding(t, analyze(t, sustainedRise("SYN-B"), nil))

	want := fmt.Sprintf("Consumption was %.0f%% above its hourly baseline for 110 h from 2030-03-10 10:00",
		100*f.Consumption.MedianDeviationPct)
	assert.True(t, strings.HasPrefix(f.Reason, want), f.Reason)
	assert.Contains(t, f.Reason, fmt.Sprintf("current %+.0f%%", 100*metric(f, MetricCurrent).MedianDeviationPct))
	assert.Contains(t, f.Reason, "power factor -20%")
	assert.Contains(t, f.Reason, "no operational event explains it")
}

func TestAnalyze_IsDeterministicAndInputOrderIndependent(t *testing.T) {
	readings, events := mixedDataset()
	want := analyze(t, readings, events)

	for range 3 {
		assert.Equal(t, want, analyze(t, readings, events))
	}

	rng := rand.New(rand.NewPCG(3, 5)) // fixed seed: reproducible permutations
	for range 5 {
		r, e := slices.Clone(readings), slices.Clone(events)
		rng.Shuffle(len(r), func(i, j int) { r[i], r[j] = r[j], r[i] })
		rng.Shuffle(len(e), func(i, j int) { e[i], e[j] = e[j], e[i] })
		assert.Equal(t, want, analyze(t, r, e))
	}
}

func TestAnalyze_DoesNotModifyCallerSlices(t *testing.T) {
	readings, events := mixedDataset()
	slices.Reverse(readings)
	slices.Reverse(events)
	readingsBefore, eventsBefore := slices.Clone(readings), slices.Clone(events)

	analyze(t, readings, events)

	assert.Equal(t, readingsBefore, readings)
	assert.Equal(t, eventsBefore, events)
}

func TestAnalyze_RejectsStructurallyInvalidInput(t *testing.T) {
	valid := stableSeries("SYN-V", 2, 40)
	with := func(change func([]Reading) []Reading) []Reading { return change(slices.Clone(valid)) }

	tests := map[string]struct {
		readings []Reading
		events   []Event
		want     string
	}{
		"no readings":         {nil, nil, "no readings"},
		"duplicate":           {with(func(r []Reading) []Reading { return append(r, r[5]) }), nil, "duplicate reading for meter SYN-V"},
		"off-interval":        {with(func(r []Reading) []Reading { r[4].Timestamp = r[4].Timestamp.Add(30 * time.Minute); return r }), nil, "not a whole 1h0m0s"},
		"NaN value":           {with(func(r []Reading) []Reading { r[3].VoltageV = math.NaN(); return r }), nil, "non-finite"},
		"infinite value":      {with(func(r []Reading) []Reading { r[3].CurrentA = math.Inf(-1); return r }), nil, "non-finite"},
		"blank meter":         {with(func(r []Reading) []Reading { r[0].MeterID = " "; return r }), nil, "no meter ID"},
		"zero time":           {with(func(r []Reading) []Reading { r[0].Timestamp = time.Time{}; return r }), nil, "no timestamp"},
		"event without type":  {valid, []Event{{MeterID: "SYN-V", Timestamp: at(1, 0)}}, "has no type"},
		"event without meter": {valid, []Event{{Timestamp: at(1, 0), Type: EventUnknown}}, "no meter ID"},
		"event without time":  {valid, []Event{{MeterID: "SYN-V", Type: EventUnknown}}, "no timestamp"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := newEngine(t).Analyze(context.Background(), tc.readings, tc.events)

			require.ErrorIs(t, err, ErrInvalidInput)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestAnalyze_EventsForMetersWithoutReadingsAreIgnored(t *testing.T) {
	events := []Event{{MeterID: "SYN-NONE", Timestamp: at(9, 0), Type: EventOperationalChange}}

	res := analyze(t, stableSeries("SYN-A", days, 40), events)

	assert.Empty(t, res.Findings)
	assert.Len(t, res.Meters, 1)
}

func TestAnalyze_HonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newEngine(t).Analyze(ctx, stableSeries("SYN-A", days, 40), nil)

	assert.True(t, errors.Is(err, context.Canceled))
}

func TestConfig_DefaultsAreValidAndMistakesAreRejected(t *testing.T) {
	require.NoError(t, DefaultConfig().Validate())

	broken := DefaultConfig()
	broken.Weights.SignalStrength = 0.5
	broken.MinDeviation.Voltage = 0
	broken.RecoveryReadings = 2 // 2 h does not cover the 3 h MaxGap

	_, err := New(broken)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "weights must sum to 1")
	assert.Contains(t, err.Error(), "voltage_v")
	assert.Contains(t, err.Error(), "RecoveryReadings")
}

func metric(f Finding, m Metric) MetricEvidence {
	for _, me := range f.Metrics {
		if me.Metric == m {
			return me
		}
	}
	return MetricEvidence{}
}
