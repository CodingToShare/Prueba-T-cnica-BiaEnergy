package analysis

import (
	"math"
	"slices"
	"time"
)

// episode is a group of flagged readings of one kind, consecutive or
// separated by at most Config.MaxGap. It indexes into a meter's evaluations.
type episode struct {
	kind    kind
	flagged []int
}

func (ep episode) first() int { return ep.flagged[0] }
func (ep episode) last() int  { return ep.flagged[len(ep.flagged)-1] }

// buildEpisodes groups the flagged readings of one meter. Readings of
// different kinds never merge; a gap longer than MaxGap closes an episode.
// Episodes are returned in order of onset.
func (e *Engine) buildEpisodes(evals []evaluation) []episode {
	var open [kindCount]*episode
	var done []episode
	for i, ev := range evals {
		k := ev.kind
		if k == kindNone {
			continue
		}
		if cur := open[k]; cur != nil && ev.at.Sub(evals[cur.last()].at) <= e.cfg.MaxGap {
			cur.flagged = append(cur.flagged, i)
			continue
		}
		if open[k] != nil {
			done = append(done, *open[k])
		}
		open[k] = &episode{kind: k, flagged: []int{i}}
	}
	for _, ep := range open {
		if ep != nil {
			done = append(done, *ep)
		}
	}
	slices.SortStableFunc(done, func(a, b episode) int { return a.first() - b.first() })
	return done
}

// persistence measures how sustained an episode is and whether the readings
// after it recovered.
func (e *Engine) persistence(evals []evaluation, ep episode) Persistence {
	first, last := evals[ep.first()], evals[ep.last()]
	p := Persistence{
		FlaggedReadings: len(ep.flagged),
		SpanReadings:    ep.last() - ep.first() + 1,
		LongestRun:      1,
	}
	p.Density = float64(p.FlaggedReadings) / float64(p.SpanReadings)
	p.Sustained = last.at.Sub(first.at)+e.cfg.ReadingInterval >= e.cfg.SustainedDuration

	run := 1
	for j := 1; j < len(ep.flagged); j++ {
		if evals[ep.flagged[j]].at.Sub(evals[ep.flagged[j-1]].at) == e.cfg.ReadingInterval {
			run++
		} else {
			run = 1
		}
		p.LongestRun = max(p.LongestRun, run)
	}
	p.Recovery = e.recovery(evals, ep)
	return p
}

// recovery looks at the evaluated readings after the episode. A load episode
// recovers when consumption no longer deviates; an inconsistency episode
// recovers when readings are consistent again. Recovery needs
// RecoveryReadings such readings in a row.
func (e *Engine) recovery(evals []evaluation, ep episode) Recovery {
	var r Recovery
	var pcts []float64
	for j := ep.last() + 1; j < len(evals) && r.ReadingsObserved < e.cfg.RecoveryReadings; j++ {
		ev := evals[j]
		if !ev.evaluated || ev.at.Sub(evals[j-1].at) != e.cfg.ReadingInterval {
			// Missing evidence cannot establish continuous recovery from an outage.
			break
		}
		if ep.kind == kindInconsistent && ev.kind == kindInconsistent ||
			ep.kind != kindInconsistent && ev.triggered[idxConsumption] {
			break
		}
		if r.ReadingsObserved == 0 {
			r.RecoveredAt = ev.at
		}
		r.ReadingsObserved++
		pcts = append(pcts, ev.dev[idxConsumption].pct)
	}
	r.Recovered = r.ReadingsObserved >= e.cfg.RecoveryReadings
	if !r.Recovered {
		r.RecoveredAt = time.Time{}
	}
	r.MedianConsumptionDeviationPct = median(pcts)
	return r
}

// metricEvidence aggregates every metric over the flagged readings and
// returns how many electrical metrics corroborate the episode.
func (e *Engine) metricEvidence(evals []evaluation, ep episode) ([]MetricEvidence, int) {
	loadDirection := DirectionUp
	if ep.kind == kindLoadDown {
		loadDirection = DirectionDown
	}
	// Current must track consumption before ancillary electrical changes
	// can confirm a load change. Otherwise they may be measurement faults.
	currentFollowing, currentOpposing := 0, 0
	for _, i := range ep.flagged {
		ev := evals[i]
		if ev.triggered[idxCurrent] {
			if directionOf(ev.dev[idxCurrent].pct) == loadDirection {
				currentFollowing++
			} else {
				currentOpposing++
			}
		}
	}
	coherentCurrent := currentOpposing == 0 && float64(currentFollowing)/float64(len(ep.flagged)) >= e.cfg.CorroborationShare
	out := make([]MetricEvidence, 0, metricCount)
	corroborating := 0
	for m := range metricCount {
		me := MetricEvidence{Metric: metricNames[m]}
		var observed, baseline, pcts []float64
		up, down := 0, 0
		for _, i := range ep.flagged {
			ev := evals[i]
			if !ev.measured[m] {
				continue
			}
			d := ev.dev[m]
			observed = append(observed, ev.features.values[m])
			baseline = append(baseline, d.baseline)
			pcts = append(pcts, d.pct)
			me.MaxAbsDeviationPct = max(me.MaxAbsDeviationPct, math.Abs(d.pct))
			if ev.triggered[m] {
				if d.pct > 0 {
					up++
				} else {
					down++
				}
			}
		}
		me.EvaluatedReadings = len(pcts)
		me.TriggeredReadings = up + down
		me.MedianObserved = median(observed)
		me.MedianBaseline = median(baseline)
		me.MedianDeviationPct = median(pcts)
		switch {
		case me.TriggeredReadings == 0:
		case up > 0 && down > 0:
			me.Direction = DirectionMixed
		case up > 0:
			me.Direction = DirectionUp
		default:
			me.Direction = DirectionDown
		}
		if m != idxConsumption {
			// In a load episode, current must move with consumption, and a
			// ratio change means the electrical metrics did not follow the
			// load, which contradicts rather than supports it.
			share := float64(me.TriggeredReadings) / float64(len(ep.flagged))
			supports := ep.kind == kindInconsistent ||
				m == idxCurrent && me.Direction == loadDirection ||
				coherentCurrent && (m == idxVoltage || m == idxPowerFactor)
			me.Corroborates = share >= e.cfg.CorroborationShare && supports
			// The ratio remains evidence, but is derived from these same
			// measurements and is not another independent confidence vote.
			if me.Corroborates && m != idxLoadRatio {
				corroborating++
			}
		}
		out = append(out, me)
	}
	return out, corroborating
}

// consumptionEvidence compares observed and expected energy over the
// flagged readings.
func consumptionEvidence(evals []evaluation, ep episode) ConsumptionEvidence {
	var c ConsumptionEvidence
	pcts := make([]float64, 0, len(ep.flagged))
	for _, i := range ep.flagged {
		ev := evals[i]
		c.ObservedKWh += ev.features.values[idxConsumption]
		c.BaselineKWh += ev.dev[idxConsumption].baseline
		pcts = append(pcts, ev.dev[idxConsumption].pct)
	}
	if c.BaselineKWh != 0 {
		c.DeviationPct = c.ObservedKWh/c.BaselineKWh - 1
	}
	c.MedianDeviationPct = median(pcts)
	c.Direction = directionOf(c.MedianDeviationPct)
	return c
}

// evidenceStrength is the median, over the flagged readings, of the primary
// signal's size relative to its threshold: consumption for load episodes,
// the strongest electrical metric for inconsistency episodes.
func (e *Engine) evidenceStrength(evals []evaluation, ep episode) float64 {
	strengths := make([]float64, 0, len(ep.flagged))
	for _, i := range ep.flagged {
		ev := evals[i]
		if ep.kind != kindInconsistent {
			strengths = append(strengths, math.Abs(ev.dev[idxConsumption].pct)/e.cfg.threshold(idxConsumption))
			continue
		}
		strongest := 0.0
		for _, m := range electricalMetrics {
			if ev.triggered[m] {
				strongest = max(strongest, math.Abs(ev.dev[m].pct)/e.cfg.threshold(m))
			}
		}
		strengths = append(strengths, strongest)
	}
	return median(strengths)
}
