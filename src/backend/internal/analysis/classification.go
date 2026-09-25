package analysis

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// assess turns an episode into a finding, or reports that it is not
// reportable: too few flagged readings, or an unexplained consumption change
// that is neither sustained nor corroborated by any electrical metric.
func (e *Engine) assess(meterID string, evals []evaluation, ep episode, events eventIndex) (Finding, bool) {
	if len(ep.flagged) < e.cfg.MinEpisodeReadings {
		return Finding{}, false
	}
	start, end := evals[ep.first()].at, evals[ep.last()].at
	f := Finding{
		MeterID:        meterID,
		StartedAt:      start,
		LastObservedAt: end,
		Duration:       end.Sub(start) + e.cfg.ReadingInterval,
	}
	f.Persistence = e.persistence(evals, ep)
	var corroborating int
	f.Metrics, corroborating = e.metricEvidence(evals, ep)
	f.Consumption = consumptionEvidence(evals, ep)
	f.EvidenceStrength = e.evidenceStrength(evals, ep)
	f.RelatedEvents = correlate(ep.kind, start, events.near(meterID, start, e.cfg.EventWindow))

	f.Type, f.Rule = classify(ep.kind, f.RelatedEvents, f.Persistence.Recovery.Recovered)
	if f.Type == RealAnomaly && corroborating == 0 && !f.Persistence.Sustained {
		return Finding{}, false
	}
	f.Severity = e.severity(f, corroborating)
	f.ConfidenceDetail = e.confidenceBreakdown(f, corroborating)
	f.Confidence = e.cfg.Weights.combine(f.ConfidenceDetail)
	f.RecommendedAction = recommendedAction(f.Type)
	for _, i := range ep.flagged {
		f.Signals = append(f.Signals, e.signalsOf(evals[i])...)
	}
	f.Reason = reason(f)
	return f, true
}

// classify applies the decision rules in order: inconsistent measurements
// are a data-quality problem; a consumption change explained by a compatible
// event is explainable, or a false positive when a scheduled outage ends
// with an observed recovery; any other consumption change is real.
func classify(k kind, related []RelatedEvent, recovered bool) (Classification, Rule) {
	if k == kindInconsistent {
		return DataQuality, RuleRepeatedElectricalInconsistency
	}
	ex, ok := explanation(related)
	switch {
	case !ok:
		return RealAnomaly, RuleUnexplainedConsumptionShift
	case ex.Type == EventScheduledOutage && recovered:
		return FalsePositive, RuleScheduledOutageWithRecovery
	case ex.Type == EventScheduledOutage:
		return ExplainableAnomaly, RuleScheduledOutageNoRecovery
	default:
		return ExplainableAnomaly, RuleOperationalChangeExplains
	}
}

// severity rates operational importance from the classification and the
// evidence (OD-04):
//   - REAL_ANOMALY: HIGH when sustained, at least HighDeviation and
//     corroborated; MEDIUM when sustained or large; otherwise LOW;
//   - DATA_QUALITY: HIGH when sustained (the meter's measurements are
//     unreliable for a material period); otherwise MEDIUM;
//   - EXPLAINABLE_ANOMALY: MEDIUM when sustained (the change must be
//     validated); otherwise LOW; never HIGH, because an explained change is
//     not escalated (BR-03);
//   - FALSE_POSITIVE: LOW, because it is explained and recovered.
func (e *Engine) severity(f Finding, corroborating int) Severity {
	sustained := f.Persistence.Sustained
	switch f.Type {
	case FalsePositive:
		return SeverityLow
	case ExplainableAnomaly:
		if sustained {
			return SeverityMedium
		}
		return SeverityLow
	case DataQuality:
		if sustained {
			return SeverityHigh
		}
		return SeverityMedium
	default:
		large := math.Abs(f.Consumption.MedianDeviationPct) >= e.cfg.HighDeviation
		switch {
		case sustained && large && corroborating > 0:
			return SeverityHigh
		case sustained || large:
			return SeverityMedium
		default:
			return SeverityLow
		}
	}
}

// confidenceBreakdown scores how well the evidence supports the
// classification (OD-03). Every component is in [0, 1]:
//   - SignalStrength: how far beyond its threshold the primary signal is;
//   - Persistence: flagged readings relative to FullPersistenceReadings;
//   - MultivariateSupport: corroborating electrical metrics relative to
//     FullMultivariateMetrics;
//   - EventContext: for explained findings, how close the explaining event
//     is to the onset; for real anomalies, 1 unless an explanatory-type event
//     that cannot explain this direction is nearby; for data quality, 1 with
//     a corroborating event and 0.5 without;
//   - PatternSupport: for false positives, how completely consumption
//     returned to baseline; for data quality, how stable consumption stayed;
//     for real and explainable anomalies, 1 while the shift persists and 0.5
//     once it recovered.
func (e *Engine) confidenceBreakdown(f Finding, corroborating int) ConfidenceBreakdown {
	cfg := e.cfg
	b := ConfidenceBreakdown{
		SignalStrength:      clamp01((f.EvidenceStrength - 1) / (cfg.SignalSaturation - 1)),
		Persistence:         clamp01(float64(f.Persistence.FlaggedReadings) / float64(cfg.FullPersistenceReadings)),
		MultivariateSupport: clamp01(float64(corroborating) / float64(cfg.FullMultivariateMetrics)),
	}
	persisting := 1.0
	if f.Persistence.Recovery.Recovered {
		persisting = 0.5
	}
	switch f.Type {
	case RealAnomaly:
		b.EventContext = 1
		for _, re := range f.RelatedEvents {
			if re.Type == EventOperationalChange || re.Type == EventScheduledOutage {
				b.EventContext = 0.5
			}
		}
		b.PatternSupport = persisting
	case ExplainableAnomaly, FalsePositive:
		ex, _ := explanation(f.RelatedEvents)
		b.EventContext = 1
		if cfg.EventWindow > 0 {
			b.EventContext = 1 - 0.5*clamp01(float64(absDuration(ex.Offset))/float64(cfg.EventWindow))
		}
		b.PatternSupport = persisting
		if f.Type == FalsePositive {
			b.PatternSupport = 1 - clamp01(math.Abs(f.Persistence.Recovery.MedianConsumptionDeviationPct)/cfg.MinDeviation.Consumption)
		}
	case DataQuality:
		b.EventContext = 0.5
		for _, re := range f.RelatedEvents {
			if re.Role == RoleCorroborates {
				b.EventContext = 1
			}
		}
		b.PatternSupport = 1 - clamp01(math.Abs(f.Consumption.MedianDeviationPct)/cfg.MinDeviation.Consumption)
	}
	return b
}

// combine is the weighted sum of the components, bounded to [0, 1].
func (w ConfidenceWeights) combine(b ConfidenceBreakdown) float64 {
	return clamp01(w.SignalStrength*b.SignalStrength +
		w.Persistence*b.Persistence +
		w.MultivariateSupport*b.MultivariateSupport +
		w.EventContext*b.EventContext +
		w.PatternSupport*b.PatternSupport)
}

// recommendedAction maps a classification to its action (BR-08).
func recommendedAction(c Classification) Action {
	switch c {
	case RealAnomaly:
		return ActionInvestigateMeterAndInstallation
	case DataQuality:
		return ActionValidateMeasurementOrSensor
	case ExplainableAnomaly:
		return ActionValidateOperationalChange
	default:
		return ActionNoEscalationMonitor
	}
}

// label is the operator wording of a metric.
func label(m Metric) string {
	switch m {
	case MetricConsumption:
		return "consumption"
	case MetricVoltage:
		return "voltage"
	case MetricCurrent:
		return "current"
	case MetricPowerFactor:
		return "power factor"
	default:
		return "consumption-to-load ratio"
	}
}

const reasonTimeLayout = "2006-01-02 15:04"

// reason writes a one-sentence summary using only the finding's evidence.
func reason(f Finding) string {
	start := f.StartedAt.Format(reasonTimeLayout)
	hours := strconv.FormatFloat(f.Duration.Hours(), 'f', -1, 64) + " h"

	if f.Type == DataQuality {
		var affected []string
		within := 0.0
		for _, me := range f.Metrics {
			if me.Corroborates {
				affected = append(affected, label(me.Metric))
			}
			if me.Metric == MetricConsumption {
				within = me.MaxAbsDeviationPct
			}
		}
		context := "no data-quality event was reported"
		for _, re := range f.RelatedEvents {
			if re.Role == RoleCorroborates {
				context = fmt.Sprintf("a %s event at %s corroborates it", re.Type, re.Timestamp.Format(reasonTimeLayout))
			}
		}
		return fmt.Sprintf("%d of %d readings over %s from %s had inconsistent %s while consumption stayed within %.0f%% of its hourly baseline; %s.",
			f.Persistence.FlaggedReadings, f.Persistence.SpanReadings, hours, start,
			joinLabels(affected), math.Ceil(100*within), context)
	}

	side := "above"
	if f.Consumption.Direction == DirectionDown {
		side = "below"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Consumption was %.0f%% %s its hourly baseline for %s from %s",
		100*math.Abs(f.Consumption.MedianDeviationPct), side, hours, start)
	var changes []string
	for _, me := range f.Metrics {
		if me.Corroborates {
			changes = append(changes, fmt.Sprintf("%s %+.0f%%", label(me.Metric), 100*me.MedianDeviationPct))
		}
	}
	if len(changes) > 0 {
		b.WriteString(", with " + joinLabels(changes))
	}

	ex, explained := explanation(f.RelatedEvents)
	exAt := ex.Timestamp.Format(reasonTimeLayout)
	switch {
	case !explained && len(f.RelatedEvents) > 0:
		var types []string
		for _, re := range f.RelatedEvents {
			types = append(types, string(re.Type))
		}
		if len(types) == 1 {
			fmt.Fprintf(&b, "; the correlated %s event does not explain it", types[0])
		} else {
			fmt.Fprintf(&b, "; the correlated %s events do not explain it", joinLabels(types))
		}
	case !explained:
		b.WriteString("; no operational event explains it")
	case f.Type == FalsePositive:
		fmt.Fprintf(&b, "; it began with the %s event at %s and returned to within %.0f%% of baseline from %s, so it is not escalated",
			ex.Type, exAt, 100*math.Abs(f.Persistence.Recovery.MedianConsumptionDeviationPct),
			f.Persistence.Recovery.RecoveredAt.Format(reasonTimeLayout))
	case f.Rule == RuleScheduledOutageNoRecovery:
		fmt.Fprintf(&b, "; it began with the %s event at %s, but no recovery was observed", ex.Type, exAt)
	default:
		fmt.Fprintf(&b, "; it began with the %s event at %s", ex.Type, exAt)
	}
	b.WriteString(".")
	return b.String()
}

func joinLabels(labels []string) string {
	switch len(labels) {
	case 0:
		return "electrical readings"
	case 1:
		return labels[0]
	default:
		return strings.Join(labels[:len(labels)-1], ", ") + " and " + labels[len(labels)-1]
	}
}
