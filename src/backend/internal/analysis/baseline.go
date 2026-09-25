package analysis

import (
	"math"
	"time"
)

// metricIndex addresses the per-reading feature arrays.
type metricIndex int

const (
	idxConsumption metricIndex = iota
	idxVoltage
	idxCurrent
	idxPowerFactor
	idxLoadRatio
	metricCount
)

var metricNames = [metricCount]Metric{
	MetricConsumption, MetricVoltage, MetricCurrent, MetricPowerFactor, MetricLoadRatio,
}

// electricalMetrics can corroborate or contradict consumption.
var electricalMetrics = [...]metricIndex{idxVoltage, idxCurrent, idxPowerFactor, idxLoadRatio}

// kind is the reading-level interpretation that episodes group by.
type kind int

const (
	kindNone kind = iota
	// kindLoadUp and kindLoadDown: consumption itself deviates.
	kindLoadUp
	kindLoadDown
	// kindInconsistent: consumption is on its baseline but several electrical
	// metrics deviate, so the measurements contradict each other.
	kindInconsistent
	kindCount
)

// features holds the analyzed values of one reading.
type features struct {
	values    [metricCount]float64
	available [metricCount]bool
}

// featuresOf derives the analyzed values. The load ratio is unavailable when
// V × I × PF is not positive or the ratio is not finite; the reading is still
// analyzed on its other metrics.
func featuresOf(r Reading) features {
	f := features{values: [metricCount]float64{
		r.ConsumptionKWh, r.VoltageV, r.CurrentA, r.PowerFactor, 0,
	}}
	for m := range idxLoadRatio {
		f.available[m] = true
	}
	if proxyKW := r.VoltageV * r.CurrentA * r.PowerFactor / 1000; proxyKW > 0 && finite(proxyKW) {
		if ratio := r.ConsumptionKWh / proxyKW; finite(ratio) {
			f.values[idxLoadRatio] = ratio
			f.available[idxLoadRatio] = true
		}
	}
	return f
}

// evaluation is the analysis of one reading against its baseline.
type evaluation struct {
	at       time.Time
	features features
	// evaluated: the consumption baseline was mature, so the reading was tested.
	evaluated bool
	measured  [metricCount]bool
	dev       [metricCount]deviation
	triggered [metricCount]bool
	flagged   bool // at least one metric triggered
	kind      kind
}

// evaluate tests each reading of one meter, in time order, against the
// baseline for its hour of day: the median and MAD of the most recent
// HistoryObservations same-hour readings that were not flagged.
//
// Only earlier readings enter a baseline, so there is no future leakage.
// Flagged readings never enter it, so an ongoing episode keeps being compared
// with its pre-episode behavior instead of teaching the baseline that the
// anomaly is normal.
func (e *Engine) evaluate(readings []Reading) []evaluation {
	cfg := e.cfg
	var history [24][]features
	out := make([]evaluation, len(readings))
	values := make([]float64, 0, cfg.HistoryObservations)

	for i, r := range readings {
		ev := &out[i]
		ev.at = r.Timestamp
		ev.features = featuresOf(r)
		hour := r.Timestamp.Hour()
		hist := history[hour]

		if len(hist) == cfg.HistoryObservations {
			for m := range metricCount {
				if !ev.features.available[m] {
					continue
				}
				values = values[:0]
				for _, h := range hist {
					if h.available[m] {
						values = append(values, h.values[m])
					}
				}
				if len(values) < cfg.HistoryObservations {
					continue
				}
				d, ok := robustDeviation(ev.features.values[m], values, cfg.MinSpreadFraction)
				if !ok {
					continue
				}
				ev.measured[m] = true
				ev.dev[m] = d
				ev.triggered[m] = math.Abs(d.pct) >= cfg.threshold(m) && math.Abs(d.robustZ) >= cfg.MinRobustZ
			}
			ev.evaluated = ev.measured[idxConsumption]
		}
		if !ev.evaluated {
			// Without a consumption baseline nothing is judged.
			ev.measured, ev.triggered, ev.dev = [metricCount]bool{}, [metricCount]bool{}, [metricCount]deviation{}
		}
		ev.kind, ev.flagged = e.interpret(ev)

		if !ev.flagged {
			if len(hist) == cfg.HistoryObservations {
				copy(hist, hist[1:])
				hist[len(hist)-1] = ev.features
			} else {
				hist = append(hist, ev.features)
			}
			history[hour] = hist
		}
	}
	return out
}

// interpret assigns the reading-level kind: a consumption deviation is a load
// change; normal consumption with several deviating electrical metrics is an
// inconsistent measurement. Any other signal is not grouped into an episode,
// but the reading is still flagged and kept out of baselines.
func (e *Engine) interpret(ev *evaluation) (kind, bool) {
	flagged := false
	for _, t := range ev.triggered {
		flagged = flagged || t
	}
	if ev.triggered[idxConsumption] {
		if ev.dev[idxConsumption].pct > 0 {
			return kindLoadUp, true
		}
		return kindLoadDown, true
	}
	electrical := 0
	for _, m := range electricalMetrics {
		if ev.triggered[m] {
			electrical++
		}
	}
	if electrical >= e.cfg.MinInconsistentMetrics {
		return kindInconsistent, true
	}
	return kindNone, flagged
}

// signalsOf lists the triggered metrics of a reading as evidence.
func (e *Engine) signalsOf(ev evaluation) []Signal {
	var out []Signal
	for m := range metricCount {
		if !ev.triggered[m] {
			continue
		}
		d := ev.dev[m]
		out = append(out, Signal{
			Metric:       metricNames[m],
			Timestamp:    ev.at,
			Observed:     ev.features.values[m],
			Baseline:     d.baseline,
			Deviation:    d.diff,
			DeviationPct: d.pct,
			RobustZ:      d.robustZ,
			Direction:    directionOf(d.pct),
			Strength:     math.Abs(d.pct) / e.cfg.threshold(m),
		})
	}
	return out
}
