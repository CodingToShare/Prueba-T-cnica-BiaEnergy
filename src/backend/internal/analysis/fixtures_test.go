package analysis

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Synthetic fixtures use fictional meter IDs and a deterministic pseudo-noise
// (sines of the day and hour), never randomness and never the supplied data.

var day0 = time.Date(2030, 3, 1, 0, 0, 0, 0, time.UTC)

// at returns the timestamp of a day and hour of the synthetic calendar.
func at(day, hour int) time.Time {
	return day0.Add(time.Duration(day*24+hour) * time.Hour)
}

// stableSeries returns days × 24 hourly readings with a daily load profile
// and small variations: about ±2% consumption, ±0.4% voltage, ±0.8% power
// factor, and a current consistent with them.
func stableSeries(meterID string, days int, scale float64) []Reading {
	out := make([]Reading, 0, days*24)
	for d := range days {
		for h := range 24 {
			fd, fh := float64(d), float64(h)
			profile := 0.7 + 0.5*math.Exp(-(fh-13)*(fh-13)/18)
			c := scale * profile * (1 + 0.02*math.Sin(fd*1.7+fh*0.9))
			v := 220 * (1 + 0.004*math.Sin(fd*2.3+fh*1.1))
			pf := 0.94 * (1 + 0.008*math.Sin(fd*0.7+fh*1.9))
			i := c * 1000 / (v * pf * 1.06) * (1 + 0.01*math.Sin(fd*1.3+fh*2.9))
			out = append(out, Reading{
				MeterID: meterID, Timestamp: at(d, h),
				ConsumptionKWh: c, VoltageV: v, CurrentA: i, PowerFactor: pf,
			})
		}
	}
	return out
}

// modify applies change to every reading in [from, to).
func modify(readings []Reading, from, to time.Time, change func(*Reading)) {
	for i := range readings {
		if t := readings[i].Timestamp; !t.Before(from) && t.Before(to) {
			change(&readings[i])
		}
	}
}

// scaleLoad multiplies consumption and current (a real load change).
func scaleLoad(k float64) func(*Reading) {
	return func(r *Reading) {
		r.ConsumptionKWh *= k
		r.CurrentA *= k
	}
}

func newEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := New(DefaultConfig())
	require.NoError(t, err)
	return e
}

// evaluateSeries runs the baseline and signal stage on one sorted series.
func evaluateSeries(t *testing.T, readings []Reading) []evaluation {
	t.Helper()
	return newEngine(t).evaluate(readings)
}

// indexOf returns the position of a timestamp in a sorted series.
func indexOf(t *testing.T, readings []Reading, ts time.Time) int {
	t.Helper()
	for i, r := range readings {
		if r.Timestamp.Equal(ts) {
			return i
		}
	}
	t.Fatalf("no reading at %s", ts)
	return -1
}
