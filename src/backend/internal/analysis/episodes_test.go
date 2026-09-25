package analysis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// episodesOf evaluates a series and groups its flagged readings.
func episodesOf(t *testing.T, readings []Reading) ([]evaluation, []episode) {
	t.Helper()
	e := newEngine(t)
	evals := e.evaluate(readings)
	return evals, e.buildEpisodes(evals)
}

// shiftHours applies a load change to the listed hours of day 8.
func shiftHours(readings []Reading, k float64, hours ...int) {
	for _, h := range hours {
		modify(readings, at(8, h), at(8, h).Add(time.Hour), scaleLoad(k))
	}
}

func TestEpisodes_IsolatedSpike_IsOneReadingAndNotReportable(t *testing.T) {
	e := newEngine(t)
	readings := stableSeries("TEST-EP", 10, 40)
	shiftHours(readings, 1.5, 11)
	evals, eps := episodesOf(t, readings)

	require.Len(t, eps, 1)
	assert.Len(t, eps[0].flagged, 1)
	_, reportable := e.assess("TEST-EP", evals, eps[0], newEventIndex(nil))
	assert.False(t, reportable, "a single reading is not a persistent condition")
}

func TestEpisodes_SustainedShift_FormsOneEpisodeWithBoundariesAndDuration(t *testing.T) {
	e := newEngine(t)
	readings := stableSeries("TEST-EP", 12, 40)
	modify(readings, at(8, 10), at(9, 11), scaleLoad(1.6)) // 25 hours
	evals, eps := episodesOf(t, readings)

	require.Len(t, eps, 1)
	ep := eps[0]
	assert.Equal(t, kindLoadUp, ep.kind)
	assert.Equal(t, at(8, 10), evals[ep.first()].at)
	assert.Equal(t, at(9, 10), evals[ep.last()].at)

	p := e.persistence(evals, ep)
	assert.Equal(t, 25, p.FlaggedReadings)
	assert.Equal(t, 25, p.SpanReadings)
	assert.Equal(t, 25, p.LongestRun)
	assert.Equal(t, 1.0, p.Density)
	assert.True(t, p.Sustained)
	assert.True(t, p.Recovery.Recovered)
	assert.Equal(t, at(9, 11), p.Recovery.RecoveredAt)
}

func TestEpisodes_SmallGap_IsBridged(t *testing.T) {
	e := newEngine(t)
	readings := stableSeries("TEST-EP", 10, 40)
	shiftHours(readings, 1.6, 10, 11, 14, 15) // 12 and 13 are normal: gap of 3 h
	evals, eps := episodesOf(t, readings)

	require.Len(t, eps, 1)
	p := e.persistence(evals, eps[0])
	assert.Equal(t, 4, p.FlaggedReadings)
	assert.Equal(t, 6, p.SpanReadings)
	assert.InDelta(t, 4.0/6, p.Density, 1e-12)
	assert.Equal(t, 2, p.LongestRun)
}

func TestEpisodes_GapBeyondMaxGap_SplitsEpisodes(t *testing.T) {
	readings := stableSeries("TEST-EP", 10, 40)
	shiftHours(readings, 1.6, 10, 11, 15, 16) // 12, 13, 14 normal: gap of 4 h
	_, eps := episodesOf(t, readings)

	require.Len(t, eps, 2)
	assert.Len(t, eps[0].flagged, 2)
	assert.Len(t, eps[1].flagged, 2)
}

func TestEpisodes_DifferentKinds_DoNotMerge(t *testing.T) {
	readings := stableSeries("TEST-EP", 10, 40)
	shiftHours(readings, 1.6, 10, 11, 12)
	for _, h := range []int{13, 14, 15} {
		modify(readings, at(8, h), at(8, h).Add(time.Hour), func(r *Reading) { r.CurrentA *= 0.5 })
	}
	_, eps := episodesOf(t, readings)

	require.Len(t, eps, 2)
	assert.Equal(t, kindLoadUp, eps[0].kind)
	assert.Equal(t, kindInconsistent, eps[1].kind)
}

func TestEpisodes_Recovery(t *testing.T) {
	t.Run("returns to baseline", func(t *testing.T) {
		e := newEngine(t)
		readings := stableSeries("TEST-EP", 10, 40)
		modify(readings, at(8, 0), at(8, 10), scaleLoad(0.2))
		evals, eps := episodesOf(t, readings)
		require.Len(t, eps, 1)

		r := e.persistence(evals, eps[0]).Recovery

		assert.True(t, r.Recovered)
		assert.Equal(t, at(8, 10), r.RecoveredAt)
		assert.Equal(t, 6, r.ReadingsObserved)
		assert.Less(t, abs(r.MedianConsumptionDeviationPct), 0.05)
	})
	t.Run("still shifted at the end of the data", func(t *testing.T) {
		e := newEngine(t)
		readings := stableSeries("TEST-EP", 10, 40)
		modify(readings, at(9, 0), at(10, 0), scaleLoad(0.2))
		evals, eps := episodesOf(t, readings)
		require.Len(t, eps, 1)

		r := e.persistence(evals, eps[0]).Recovery

		assert.False(t, r.Recovered)
		assert.True(t, r.RecoveredAt.IsZero())
		assert.Zero(t, r.ReadingsObserved)
	})
	t.Run("too few normal readings before the deviation returns", func(t *testing.T) {
		e := newEngine(t)
		readings := stableSeries("TEST-EP", 10, 40)
		modify(readings, at(8, 0), at(8, 4), scaleLoad(0.2))
		modify(readings, at(8, 8), at(8, 12), scaleLoad(0.2)) // 4 normal readings in between
		evals, eps := episodesOf(t, readings)
		require.Len(t, eps, 2)

		r := e.persistence(evals, eps[0]).Recovery

		assert.False(t, r.Recovered)
		assert.Equal(t, 4, r.ReadingsObserved)
	})
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
