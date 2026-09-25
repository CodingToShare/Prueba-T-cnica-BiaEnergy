package analysis

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClassify_DecisionRules(t *testing.T) {
	onset := at(8, 0)
	events := func(types ...EventType) []Event {
		var out []Event
		for _, ty := range types {
			out = append(out, Event{MeterID: "TEST-A", Timestamp: onset, Type: ty})
		}
		return out
	}
	tests := map[string]struct {
		kind      kind
		events    []Event
		recovered bool
		want      Classification
		rule      Rule
	}{
		"unexplained persistent shift":                             {kindLoadUp, nil, false, RealAnomaly, RuleUnexplainedConsumptionShift},
		"UNKNOWN does not suppress":                                {kindLoadUp, events(EventUnknown), false, RealAnomaly, RuleUnexplainedConsumptionShift},
		"unrecognized type does not suppress":                      {kindLoadDown, events("FIRMWARE_UPDATE"), true, RealAnomaly, RuleUnexplainedConsumptionShift},
		"outage cannot explain a rise":                             {kindLoadUp, events(EventScheduledOutage), true, RealAnomaly, RuleUnexplainedConsumptionShift},
		"data-quality event does not explain a consumption change": {kindLoadUp, events(EventDataQuality), false, RealAnomaly, RuleUnexplainedConsumptionShift},
		"operational change explains a shift":                      {kindLoadUp, events(EventOperationalChange), false, ExplainableAnomaly, RuleOperationalChangeExplains},
		"outage with recovery is a false positive":                 {kindLoadDown, events(EventScheduledOutage), true, FalsePositive, RuleScheduledOutageWithRecovery},
		"outage without recovery stays visible":                    {kindLoadDown, events(EventScheduledOutage), false, ExplainableAnomaly, RuleScheduledOutageNoRecovery},
		"inconsistent measurements without event":                  {kindInconsistent, nil, false, DataQuality, RuleRepeatedElectricalInconsistency},
		"inconsistent measurements with event":                     {kindInconsistent, events(EventDataQuality), false, DataQuality, RuleRepeatedElectricalInconsistency},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, rule := classify(tc.kind, correlate(tc.kind, onset, tc.events), tc.recovered)

			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.rule, rule)
		})
	}
}

// finding builds the evidence that severity and confidence read.
func finding(ty Classification, sustained bool, deviation float64, flagged int, strength float64) Finding {
	return Finding{
		Type:             ty,
		EvidenceStrength: strength,
		Consumption:      ConsumptionEvidence{MedianDeviationPct: deviation},
		Persistence:      Persistence{FlaggedReadings: flagged, Sustained: sustained},
	}
}

func TestSeverity_FollowsFromEvidenceNotOnlyFromType(t *testing.T) {
	e := newEngine(t)

	// Unexplained: sustained, large and corroborated by electrical changes → HIGH.
	assert.Equal(t, SeverityHigh, e.severity(finding(RealAnomaly, true, 1.0, 58, 4), 2))
	// The same deviation without any corroborating electrical metric is less certain to matter → MEDIUM.
	assert.Equal(t, SeverityMedium, e.severity(finding(RealAnomaly, true, 1.0, 58, 4), 0))
	// Sustained but moderate (below HighDeviation) → MEDIUM.
	assert.Equal(t, SeverityMedium, e.severity(finding(RealAnomaly, true, 0.3, 58, 1.2), 2))
	// Short and moderate → LOW.
	assert.Equal(t, SeverityLow, e.severity(finding(RealAnomaly, false, 0.3, 4, 1.2), 1))

	// Explained changes are never escalated to HIGH, however large (BR-03).
	assert.Equal(t, SeverityMedium, e.severity(finding(ExplainableAnomaly, true, 2.0, 96, 8), 3))
	assert.Equal(t, SeverityLow, e.severity(finding(ExplainableAnomaly, false, 0.5, 5, 2), 1))

	// Unreliable measurements over a material period → HIGH; a short burst → MEDIUM.
	assert.Equal(t, SeverityHigh, e.severity(finding(DataQuality, true, 0.02, 16, 3), 4))
	assert.Equal(t, SeverityMedium, e.severity(finding(DataQuality, false, 0.02, 3, 3), 4))

	// Explained and recovered → LOW even when the deviation was deep.
	assert.Equal(t, SeverityLow, e.severity(finding(FalsePositive, true, -0.8, 12, 3.2), 1))
}

func TestConfidence_IsBoundedFiniteAndDeterministic(t *testing.T) {
	e := newEngine(t)
	for _, ty := range []Classification{RealAnomaly, ExplainableAnomaly, FalsePositive, DataQuality} {
		for _, strength := range []float64{0, 1, 2.5, 1e9, math.NaN(), math.Inf(1)} {
			for _, flagged := range []int{0, 3, 1000} {
				for _, corroborating := range []int{0, 2, 50} {
					f := finding(ty, flagged > 3, 0.4, flagged, strength)
					f.Persistence.Recovery.MedianConsumptionDeviationPct = strength
					f.RelatedEvents = []RelatedEvent{{Type: EventScheduledOutage, Offset: 10 * time.Hour, Role: RoleExplains}}

					b := e.confidenceBreakdown(f, corroborating)
					c := e.cfg.Weights.combine(b)

					for _, v := range []float64{b.SignalStrength, b.Persistence, b.MultivariateSupport, b.EventContext, b.PatternSupport, c} {
						assert.True(t, v >= 0 && v <= 1, "%s: component %v out of [0,1]", ty, v)
					}
					assert.Equal(t, c, e.cfg.Weights.combine(e.confidenceBreakdown(f, corroborating)))
				}
			}
		}
	}
}

func TestConfidence_StrongerEvidenceNeverLowersConfidence(t *testing.T) {
	e := newEngine(t)
	confidence := func(f Finding, corroborating int) float64 {
		return e.cfg.Weights.combine(e.confidenceBreakdown(f, corroborating))
	}
	weak := finding(RealAnomaly, false, 0.26, 3, 1.05)
	strong := finding(RealAnomaly, true, 1.1, 58, 4.4)

	assert.Less(t, confidence(weak, 0), 0.6, "weak evidence yields lower confidence")
	assert.Greater(t, confidence(strong, 2), 0.85)
	assert.LessOrEqual(t, confidence(weak, 0), confidence(weak, 1))
	assert.LessOrEqual(t, confidence(finding(RealAnomaly, false, 0.3, 3, 1.2), 1), confidence(finding(RealAnomaly, false, 0.3, 3, 2), 1))
	assert.LessOrEqual(t, confidence(finding(RealAnomaly, false, 0.3, 3, 2), 1), confidence(finding(RealAnomaly, false, 0.3, 12, 2), 1))
}

func TestConfidence_ComponentSemantics(t *testing.T) {
	e := newEngine(t)

	t.Run("event alignment and recovery strengthen a false positive", func(t *testing.T) {
		aligned := finding(FalsePositive, true, -0.8, 12, 3.2)
		aligned.RelatedEvents = []RelatedEvent{{Type: EventScheduledOutage, Offset: 0, Role: RoleExplains}}
		aligned.Persistence.Recovery = Recovery{Recovered: true, MedianConsumptionDeviationPct: 0.01}

		loose := aligned
		loose.RelatedEvents = []RelatedEvent{{Type: EventScheduledOutage, Offset: 3 * time.Hour, Role: RoleExplains}}
		loose.Persistence.Recovery = Recovery{Recovered: true, MedianConsumptionDeviationPct: 0.15}

		a, l := e.confidenceBreakdown(aligned, 1), e.confidenceBreakdown(loose, 1)
		assert.Equal(t, 1.0, a.EventContext)
		assert.Equal(t, 0.5, l.EventContext, "event at the window edge")
		assert.Greater(t, a.PatternSupport, l.PatternSupport, "a complete recovery is stronger evidence")
		assert.Greater(t, e.cfg.Weights.combine(a), e.cfg.Weights.combine(l))
	})

	t.Run("absence of explanation supports a real anomaly unless an incompatible explanation is nearby", func(t *testing.T) {
		f := finding(RealAnomaly, true, 1.0, 58, 4)
		f.RelatedEvents = []RelatedEvent{{Type: EventUnknown, Role: RoleContext}}
		assert.Equal(t, 1.0, e.confidenceBreakdown(f, 2).EventContext, "UNKNOWN confirms no explanation")

		f.RelatedEvents = []RelatedEvent{{Type: EventScheduledOutage, Role: RoleContext}}
		assert.Equal(t, 0.5, e.confidenceBreakdown(f, 2).EventContext, "an outage near a rise leaves doubt")
	})

	t.Run("a data-quality event corroborates but is not required", func(t *testing.T) {
		f := finding(DataQuality, true, 0.02, 16, 3.6)
		without := e.confidenceBreakdown(f, 4)
		f.RelatedEvents = []RelatedEvent{{Type: EventDataQuality, Role: RoleCorroborates}}
		with := e.confidenceBreakdown(f, 4)

		assert.Equal(t, 0.5, without.EventContext)
		assert.Equal(t, 1.0, with.EventContext)
		assert.InDelta(t, 1-0.02/0.25, with.PatternSupport, 1e-12, "stable consumption supports a measurement problem")
	})

	t.Run("an ongoing shift supports real and explainable anomalies more than a recovered one", func(t *testing.T) {
		f := finding(RealAnomaly, true, 1.0, 58, 4)
		ongoing := e.confidenceBreakdown(f, 2).PatternSupport
		f.Persistence.Recovery.Recovered = true
		assert.Greater(t, ongoing, e.confidenceBreakdown(f, 2).PatternSupport)
	})
}

func TestRecommendedAction_IsCoherentWithClassification(t *testing.T) {
	assert.Equal(t, ActionInvestigateMeterAndInstallation, recommendedAction(RealAnomaly))
	assert.Equal(t, ActionValidateMeasurementOrSensor, recommendedAction(DataQuality))
	assert.Equal(t, ActionValidateOperationalChange, recommendedAction(ExplainableAnomaly))
	assert.Equal(t, ActionNoEscalationMonitor, recommendedAction(FalsePositive))
}
