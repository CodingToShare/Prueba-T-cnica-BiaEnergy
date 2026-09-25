package analysis

import (
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dayIndexedSeries makes hour-5 consumption equal to 100 + day, and every
// other hour a different, constant level, so a baseline value reveals exactly
// which observations produced it.
func dayIndexedSeries(days int) []Reading {
	out := stableSeries("TEST-BASE", days, 40)
	for i := range out {
		d := int(out[i].Timestamp.Sub(day0).Hours()) / 24
		if out[i].Timestamp.Hour() == 5 {
			out[i].ConsumptionKWh = 100 + float64(d)
		} else {
			out[i].ConsumptionKWh = 50
		}
	}
	return out
}

func TestBaseline_UsesSameHourOfThePreviousSevenDaysOnly(t *testing.T) {
	readings := dayIndexedSeries(10)
	evals := evaluateSeries(t, readings)

	day9hour5 := evals[indexOf(t, readings, at(9, 5))]
	require.True(t, day9hour5.evaluated)
	// Days 2..8 at 05:00 are 102..108 (median 105). Day 0 and 1 are outside
	// the window; the constant 50 kWh of other hours is never mixed in.
	assert.Equal(t, 105.0, day9hour5.dev[idxConsumption].baseline)

	day9hour6 := evals[indexOf(t, readings, at(9, 6))]
	assert.Equal(t, 50.0, day9hour6.dev[idxConsumption].baseline)
}

func TestBaseline_InsufficientHistory_IsNotEvaluatedUntilTheEighthDay(t *testing.T) {
	readings := stableSeries("TEST-BASE", 9, 40)
	evals := evaluateSeries(t, readings)

	for i, ev := range evals {
		day := i / 24
		assert.Equal(t, day >= 7, ev.evaluated, "reading %s", ev.at)
		if !ev.evaluated {
			assert.False(t, ev.flagged, "an immature reading is never judged")
		}
	}
}

func TestBaseline_NoFutureLeakage(t *testing.T) {
	full := stableSeries("TEST-LEAK", 14, 40)
	cut := indexOf(t, full, at(10, 0))

	// The same history with a drastically different future.
	alteredFuture := slices.Clone(full)
	modify(alteredFuture, at(10, 0), at(14, 0), scaleLoad(5))

	fromFull := evaluateSeries(t, full)
	fromPrefix := evaluateSeries(t, full[:cut])
	fromAltered := evaluateSeries(t, alteredFuture)

	assert.Equal(t, fromPrefix, fromFull[:cut], "evaluating more data must not change past results")
	assert.Equal(t, fromFull[:cut], fromAltered[:cut], "future readings must not influence past results")
	assert.True(t, fromAltered[cut].flagged, "the altered future itself is detected")
}

func TestBaseline_SustainedShift_StaysComparedWithPreEpisodeBehavior(t *testing.T) {
	// A +60% load shift lasting 12 days, longer than the 7-day window: a
	// rolling baseline would absorb it after four days.
	readings := stableSeries("TEST-FREEZE", 20, 40)
	modify(readings, at(8, 0), at(20, 0), scaleLoad(1.6))
	evals := evaluateSeries(t, readings)

	for i := indexOf(t, readings, at(8, 0)); i < len(evals); i++ {
		require.True(t, evals[i].triggered[idxConsumption], "reading %s must stay anomalous", evals[i].at)
	}

	// The last day's baseline is still built from days 1..7, the pre-episode week.
	last := evals[len(evals)-1]
	var preEpisode []float64
	for d := 1; d <= 7; d++ {
		preEpisode = append(preEpisode, readings[indexOf(t, readings, at(d, 23))].ConsumptionKWh)
	}
	assert.Equal(t, median(preEpisode), last.dev[idxConsumption].baseline)
}

func TestBaseline_FlaggedReadingIsSkippedAndLaterNormalReadingsResumeTheWindow(t *testing.T) {
	readings := dayIndexedSeries(11)
	modify(readings, at(8, 5), at(8, 6), func(r *Reading) { r.ConsumptionKWh = 400 })
	evals := evaluateSeries(t, readings)

	require.True(t, evals[indexOf(t, readings, at(8, 5))].flagged)
	// Day 9 skips the spike on day 8: window days 1..7 = 101..107.
	assert.Equal(t, 104.0, evals[indexOf(t, readings, at(9, 5))].dev[idxConsumption].baseline)
	// Day 10: days 2..7 and 9 = 102..107, 109.
	assert.Equal(t, 105.0, evals[indexOf(t, readings, at(10, 5))].dev[idxConsumption].baseline)
}

func TestFeatures_LoadRatioIsRelativeAndGuarded(t *testing.T) {
	normal := featuresOf(Reading{ConsumptionKWh: 22, VoltageV: 220, CurrentA: 100, PowerFactor: 0.95})
	require.True(t, normal.available[idxLoadRatio])
	assert.InDelta(t, 22/(220*100*0.95/1000), normal.values[idxLoadRatio], 1e-12)

	for name, r := range map[string]Reading{
		"zero current":          {ConsumptionKWh: 22, VoltageV: 220, CurrentA: 0, PowerFactor: 0.95},
		"zero voltage":          {ConsumptionKWh: 22, VoltageV: 0, CurrentA: 100, PowerFactor: 0.95},
		"negative power factor": {ConsumptionKWh: 22, VoltageV: 220, CurrentA: 100, PowerFactor: -0.2},
		"underflowing product":  {ConsumptionKWh: 22, VoltageV: 1e-300, CurrentA: 1e-300, PowerFactor: 0.9},
	} {
		f := featuresOf(r)
		assert.False(t, f.available[idxLoadRatio], name)
		assert.False(t, math.IsNaN(f.values[idxLoadRatio]) || math.IsInf(f.values[idxLoadRatio], 0), name)
		assert.True(t, f.available[idxConsumption] && f.available[idxCurrent], "%s: other metrics stay analyzable", name)
	}
}
