package analysis

import (
	"math"
	"slices"
)

// madToRobustZ rescales a deviation measured in MADs to standard-deviation
// units: for normally distributed data, MAD ≈ 0.6745 σ.
const madToRobustZ = 0.6745

// median returns the median of values without modifying them. It returns 0
// for an empty slice; callers only pass non-empty histories.
func median(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// medianAbsDeviation returns the median of |v − center|.
func medianAbsDeviation(values []float64, center float64) float64 {
	abs := make([]float64, len(values))
	for i, v := range values {
		abs[i] = math.Abs(v - center)
	}
	return median(abs)
}

// deviation compares one observation with its robust baseline.
type deviation struct {
	baseline float64 // median of the history
	spread   float64 // MAD, floored at MinSpreadFraction × |baseline|
	diff     float64 // observed − baseline
	pct      float64 // diff / |baseline|
	robustZ  float64 // 0.6745 × diff / spread
}

// robustDeviation measures x against history. It returns false when the
// history is empty or its median is zero, because a relative deviation is
// then undefined. The spread floor keeps robustZ finite for a constant
// history; the relative gate in the detector stops such tiny spreads from
// turning trivial changes into signals.
func robustDeviation(x float64, history []float64, minSpreadFraction float64) (deviation, bool) {
	if len(history) == 0 {
		return deviation{}, false
	}
	center := median(history)
	scale := math.Abs(center)
	if scale == 0 {
		return deviation{}, false
	}
	spread := max(medianAbsDeviation(history, center), minSpreadFraction*scale)
	diff := x - center
	d := deviation{
		baseline: center,
		spread:   spread,
		diff:     diff,
		pct:      diff / scale,
		robustZ:  madToRobustZ * diff / spread,
	}
	if !finite(d.pct) || !finite(d.robustZ) {
		return deviation{}, false
	}
	return d, true
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// clamp01 bounds v to [0, 1]; NaN becomes 0.
func clamp01(v float64) float64 {
	if !(v > 0) {
		return 0
	}
	return min(v, 1)
}

func directionOf(v float64) Direction {
	if v < 0 {
		return DirectionDown
	}
	return DirectionUp
}
