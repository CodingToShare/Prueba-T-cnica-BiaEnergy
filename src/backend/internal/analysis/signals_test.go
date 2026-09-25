package analysis

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// evaluateChange changes one reading on the first mature day of a stable
// series and returns its evaluation.
func evaluateChange(t *testing.T, change func(*Reading)) evaluation {
	t.Helper()
	readings := stableSeries("TEST-SIG", 9, 40)
	target := at(8, 14)
	modify(readings, target, target.Add(1), change)
	return evaluateSeries(t, readings)[indexOf(t, readings, target)]
}

func TestSignals_DualGate_StatisticallyLargeButOperationallyTinyChangeIsNotASignal(t *testing.T) {
	// Voltage is almost constant, so a 1% change is many MADs away...
	readings := stableSeries("TEST-SIG", 9, 40)
	for i := range readings {
		readings[i].VoltageV = 220 + 0.01*math.Sin(float64(i))
	}
	target := at(8, 14)
	modify(readings, target, target.Add(1), func(r *Reading) { r.VoltageV = 222.2 })
	ev := evaluateSeries(t, readings)[indexOf(t, readings, target)]

	require.True(t, ev.measured[idxVoltage])
	assert.Greater(t, math.Abs(ev.dev[idxVoltage].robustZ), 3.5, "the statistical gate alone would fire")
	assert.Less(t, math.Abs(ev.dev[idxVoltage].pct), 0.03)
	// ...but it is below the relative gate, so it is not a signal.
	assert.False(t, ev.triggered[idxVoltage])
	assert.False(t, ev.flagged)
}

func TestSignals_EachMetricAndDirection(t *testing.T) {
	tests := map[string]struct {
		change    func(*Reading)
		metric    metricIndex
		direction Direction
		kind      kind
	}{
		"consumption rise": {
			change: func(r *Reading) { r.ConsumptionKWh *= 1.5; r.CurrentA *= 1.5 },
			metric: idxConsumption, direction: DirectionUp, kind: kindLoadUp,
		},
		"consumption drop": {
			change: scaleLoad(0.4), metric: idxConsumption, direction: DirectionDown, kind: kindLoadDown,
		},
		"voltage alone is flagged but not grouped": {
			change: func(r *Reading) { r.VoltageV *= 1.06 },
			metric: idxVoltage, direction: DirectionUp, kind: kindNone,
		},
		"power factor alone is flagged but not grouped": {
			change: func(r *Reading) { r.PowerFactor *= 0.85 },
			metric: idxPowerFactor, direction: DirectionDown, kind: kindNone,
		},
		"current without consumption is inconsistent": {
			change: func(r *Reading) { r.CurrentA *= 0.6 },
			metric: idxCurrent, direction: DirectionDown, kind: kindInconsistent,
		},
		"relationship changes while every metric stays under its gate": {
			// +22% consumption, −2% V, −10% I, −5% PF: none reaches its own
			// threshold, but consumption no longer matches V·I·PF (+46%).
			change: func(r *Reading) {
				r.ConsumptionKWh *= 1.22
				r.VoltageV *= 0.98
				r.CurrentA *= 0.90
				r.PowerFactor *= 0.95
			},
			metric: idxLoadRatio, direction: DirectionUp, kind: kindNone,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ev := evaluateChange(t, tc.change)

			require.True(t, ev.triggered[tc.metric], "metric %s", metricNames[tc.metric])
			assert.Equal(t, tc.direction, directionOf(ev.dev[tc.metric].pct))
			assert.Equal(t, tc.kind, ev.kind)
			assert.True(t, ev.flagged)
		})
	}
}

func TestSignals_RelationshipOnlyChange_OnlyTheRatioTriggers(t *testing.T) {
	ev := evaluateChange(t, func(r *Reading) {
		r.ConsumptionKWh *= 1.22
		r.VoltageV *= 0.98
		r.CurrentA *= 0.90
		r.PowerFactor *= 0.95
	})

	assert.Equal(t, [metricCount]bool{idxLoadRatio: true}, ev.triggered)
}

func TestSignals_SignalEvidenceCarriesTheMeasurement(t *testing.T) {
	e := newEngine(t)
	ev := evaluateChange(t, scaleLoad(0.4))

	signals := e.signalsOf(ev)

	require.NotEmpty(t, signals)
	s := signals[0]
	assert.Equal(t, MetricConsumption, s.Metric)
	assert.Equal(t, at(8, 14), s.Timestamp)
	assert.InDelta(t, s.Observed-s.Baseline, s.Deviation, 1e-9)
	assert.InDelta(t, -0.6, s.DeviationPct, 0.05)
	assert.Less(t, s.RobustZ, -3.5)
	assert.Equal(t, DirectionDown, s.Direction)
	assert.InDelta(t, math.Abs(s.DeviationPct)/0.25, s.Strength, 1e-12)
}

func TestSignals_InvalidDerivedRatio_IsEvidenceNotACrash(t *testing.T) {
	ev := evaluateChange(t, func(r *Reading) { r.CurrentA = 0 })

	assert.False(t, ev.measured[idxLoadRatio], "the ratio is undefined with zero current")
	assert.True(t, ev.triggered[idxCurrent], "the zero current itself is a signal")
	for m := range metricCount {
		d := ev.dev[m]
		assert.True(t, finite(d.pct) && finite(d.robustZ) && finite(d.baseline), "metric %s", metricNames[m])
	}
}
