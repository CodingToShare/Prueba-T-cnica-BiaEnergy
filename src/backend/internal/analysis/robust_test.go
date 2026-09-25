package analysis

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMedian_OddAndEvenCounts(t *testing.T) {
	assert.Equal(t, 3.0, median([]float64{5, 1, 3}))
	assert.Equal(t, 2.5, median([]float64{4, 1, 3, 2}))
	assert.Equal(t, 7.0, median([]float64{7}))
	assert.Equal(t, 0.0, median(nil), "empty input is defined, not NaN")
}

func TestMedian_IsOrderIndependentAndDoesNotModifyInput(t *testing.T) {
	a := []float64{9, 2, 7, 4, 5}
	b := []float64{5, 4, 7, 2, 9}

	assert.Equal(t, median(a), median(b))
	assert.Equal(t, []float64{9, 2, 7, 4, 5}, a, "caller slice must not be sorted in place")
}

func TestMedianAbsDeviation(t *testing.T) {
	values := []float64{1, 2, 3, 4, 100} // median 3; |dev| = 2,1,0,1,97
	assert.Equal(t, 1.0, medianAbsDeviation(values, 3), "an outlier barely moves the MAD")
	assert.Equal(t, 0.0, medianAbsDeviation([]float64{5, 5, 5, 5}, 5))
	assert.InDelta(t, 0.01, medianAbsDeviation([]float64{10, 10.01, 9.99, 10.02, 9.98}, 10), 1e-9)
}

func TestRobustDeviation_PositiveNegativeAndStable(t *testing.T) {
	history := []float64{96, 98, 100, 102, 104} // median 100, MAD 2

	up, ok := robustDeviation(110, history, 0.001)
	require.True(t, ok)
	assert.Equal(t, 100.0, up.baseline)
	assert.Equal(t, 10.0, up.diff)
	assert.InDelta(t, 0.10, up.pct, 1e-12)
	assert.InDelta(t, 0.6745*10/2, up.robustZ, 1e-12)

	down, ok := robustDeviation(90, history, 0.001)
	require.True(t, ok)
	assert.InDelta(t, -0.10, down.pct, 1e-12)
	assert.Less(t, down.robustZ, 0.0)

	same, ok := robustDeviation(100, history, 0.001)
	require.True(t, ok)
	assert.Zero(t, same.pct)
	assert.Zero(t, same.robustZ)
}

func TestRobustDeviation_ZeroMAD_UsesSpreadFloorAndStaysFinite(t *testing.T) {
	history := []float64{220, 220, 220, 220, 220, 220, 220}

	d, ok := robustDeviation(220.5, history, 0.001)

	require.True(t, ok)
	assert.InDelta(t, 0.22, d.spread, 1e-12, "floor = 0.1% of the median")
	assert.False(t, math.IsInf(d.robustZ, 0) || math.IsNaN(d.robustZ))
	assert.InDelta(t, 0.6745*0.5/0.22, d.robustZ, 1e-9)
}

func TestRobustDeviation_ZeroMedianOrEmptyHistory_IsUndefined(t *testing.T) {
	_, ok := robustDeviation(1, []float64{0, 0, 0}, 0.001)
	assert.False(t, ok, "a relative deviation from zero is undefined")

	_, ok = robustDeviation(1, nil, 0.001)
	assert.False(t, ok)
}

func TestClamp01(t *testing.T) {
	assert.Equal(t, 0.0, clamp01(-3))
	assert.Equal(t, 0.4, clamp01(0.4))
	assert.Equal(t, 1.0, clamp01(7))
	assert.Equal(t, 0.0, clamp01(math.NaN()))
	assert.Equal(t, 1.0, clamp01(math.Inf(1)))
}
