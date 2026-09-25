package analysisrun

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/analysis"
)

func TestProgress_FollowsTheRealStagesOnly(t *testing.T) {
	assert.Equal(t, 0, Progress(StageQueued))
	assert.Equal(t, 10, Progress(StageLoadingData))
	assert.Equal(t, 35, Progress(StageAnalyzing))
	assert.Equal(t, 85, Progress(StagePersistingResults))
	assert.Equal(t, 100, Progress(StageCompleted))
	assert.Less(t, Progress(StageLoadingData), Progress(StageAnalyzing))
	assert.Less(t, Progress(StageAnalyzing), Progress(StagePersistingResults))
}

func TestFailureMessages_AreSafeAndDefinedForEveryCode(t *testing.T) {
	for _, code := range []string{CodeLoadFailed, CodeAnalysisFailed, CodePersistenceFailed, CodeTimeout, CodeInterrupted} {
		msg := FailureMessage(code)
		assert.NotEmpty(t, msg, code)
		assert.NotContains(t, msg, "SQL")
		assert.NotContains(t, msg, "postgres")
	}
}

func TestFailureCode_ShutdownAndTimeoutTakePrecedence(t *testing.T) {
	stage := failAt(CodePersistenceFailed, errors.New("insert failed"))
	live := context.Background()

	assert.Equal(t, CodePersistenceFailed, failureCode(live, live, stage))
	assert.Equal(t, CodeAnalysisFailed, failureCode(live, live, errors.New("unclassified")))

	timedOut, cancel := context.WithTimeout(live, -time.Second)
	defer cancel()
	assert.Equal(t, CodeTimeout, failureCode(live, timedOut, stage))

	stopped, stop := context.WithCancel(live)
	stop()
	assert.Equal(t, CodeInterrupted, failureCode(stopped, timedOut, stage))
}

func sampleFinding() analysis.Finding {
	at := func(h int) time.Time { return time.Date(2030, 3, 10, h, 0, 0, 0, time.UTC) }
	dir := analysis.DirectionUp
	return analysis.Finding{
		Priority: 1, MeterID: "SYN-1", Type: analysis.RealAnomaly, Rule: analysis.RuleUnexplainedConsumptionShift,
		Severity: analysis.SeverityHigh, Confidence: 0.9333333333333333, RecommendedAction: analysis.ActionInvestigateMeterAndInstallation,
		StartedAt: at(10), LastObservedAt: at(13), Duration: 4 * time.Hour, EvidenceStrength: 4.39,
		Consumption: analysis.ConsumptionEvidence{BaselineKWh: 100, ObservedKWh: 210, DeviationPct: 1.1, MedianDeviationPct: 1.0983061925154751, Direction: dir},
		Persistence: analysis.Persistence{FlaggedReadings: 4, SpanReadings: 4, Density: 1, LongestRun: 4},
		Metrics: []analysis.MetricEvidence{
			{Metric: analysis.MetricConsumption, EvaluatedReadings: 4, TriggeredReadings: 4, MedianDeviationPct: 1.1, Direction: dir},
			{Metric: analysis.MetricVoltage, EvaluatedReadings: 4},
		},
		RelatedEvents: []analysis.RelatedEvent{{Timestamp: at(9), Type: analysis.EventUnknown, Description: "text", Offset: -time.Hour, Role: analysis.RoleContext}},
		Signals:       []analysis.Signal{{Metric: analysis.MetricConsumption, Timestamp: at(10), Observed: 21, Baseline: 10, Deviation: 11, DeviationPct: 1.1, RobustZ: 18.4, Direction: dir, Strength: 4.4}},
	}
}

func TestEvidenceOf_MapsWithoutRecomputing(t *testing.T) {
	f := sampleFinding()
	e := EvidenceOf(f)

	assert.Equal(t, EvidenceSchemaVersion, e.SchemaVersion)
	assert.Equal(t, "UNEXPLAINED_PERSISTENT_CONSUMPTION_SHIFT", e.Rule)
	assert.Equal(t, f.Consumption.MedianDeviationPct*100, e.Consumption.MedianDeviationPct, "fractions become percent values")
	assert.Equal(t, 4.0, e.Persistence.DurationHours)
	assert.Nil(t, e.Persistence.Recovery.RecoveredAt, "no recovery time is invented")
	assert.Nil(t, e.Metrics[1].Direction, "a metric that never triggered has no direction")
	assert.Equal(t, "UP", *e.Metrics[0].Direction)
	assert.Equal(t, int64(-3600), e.RelatedEvents[0].OffsetSeconds)
	assert.Equal(t, "CONTEXT", e.RelatedEvents[0].Role)
	assert.Equal(t, 18.4, e.Signals[0].RobustZ)
}

func TestEvidence_JSONUsesSourceTimesAndRoundTripsExactly(t *testing.T) {
	e := EvidenceOf(sampleFinding())

	data, err := json.Marshal(e)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"timestamp":"2030-03-10T10:00:00"`)
	assert.NotContains(t, string(data), `T10:00:00Z`)
	assert.Contains(t, string(data), `"recovered_at":null`)

	var back Evidence
	require.NoError(t, json.Unmarshal(data, &back))
	assert.Equal(t, e, back)
}
