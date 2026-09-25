package analysis

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func ranked(t *testing.T, findings ...Finding) []string {
	t.Helper()
	prioritize(findings)
	var ids []string
	for i, f := range findings {
		assert.Equal(t, i+1, f.Priority)
		ids = append(ids, f.MeterID)
	}
	return ids
}

func TestPriority_SeverityFirst(t *testing.T) {
	ids := ranked(t,
		Finding{MeterID: "LOW-REAL", Severity: SeverityLow, Type: RealAnomaly, Confidence: 0.99},
		Finding{MeterID: "MEDIUM-EXPLAINED", Severity: SeverityMedium, Type: ExplainableAnomaly, Confidence: 0.5},
		Finding{MeterID: "HIGH-DQ", Severity: SeverityHigh, Type: DataQuality, Confidence: 0.4},
	)

	assert.Equal(t, []string{"HIGH-DQ", "MEDIUM-EXPLAINED", "LOW-REAL"}, ids)
}

func TestPriority_EqualSeverity_UnexplainedRealAnomalyBeforeDataQuality(t *testing.T) {
	ids := ranked(t,
		Finding{MeterID: "DQ", Severity: SeverityHigh, Type: DataQuality, Confidence: 0.99, EvidenceStrength: 9},
		Finding{MeterID: "REAL", Severity: SeverityHigh, Type: RealAnomaly, Confidence: 0.80, EvidenceStrength: 2},
	)

	assert.Equal(t, []string{"REAL", "DQ"}, ids, "classification outranks confidence")
}

func TestPriority_ClassificationOrderWithinASeverity(t *testing.T) {
	ids := ranked(t,
		Finding{MeterID: "FP", Severity: SeverityLow, Type: FalsePositive},
		Finding{MeterID: "EXPLAINED", Severity: SeverityLow, Type: ExplainableAnomaly},
		Finding{MeterID: "DQ", Severity: SeverityLow, Type: DataQuality},
		Finding{MeterID: "REAL", Severity: SeverityLow, Type: RealAnomaly},
	)

	assert.Equal(t, []string{"REAL", "DQ", "EXPLAINED", "FP"}, ids)
}

func TestPriority_ThenConfidenceThenEvidenceThenStableTieBreak(t *testing.T) {
	base := Finding{Severity: SeverityMedium, Type: RealAnomaly, Confidence: 0.7, EvidenceStrength: 2, StartedAt: at(9, 0)}
	with := func(id string, change func(*Finding)) Finding {
		f := base
		f.MeterID = id
		change(&f)
		return f
	}

	ids := ranked(t,
		with("TIE-B", func(*Finding) {}),
		with("LATER", func(f *Finding) { f.StartedAt = at(10, 0) }),
		with("STRONGER", func(f *Finding) { f.EvidenceStrength = 3 }),
		with("TIE-A", func(*Finding) {}),
		with("MORE-CONFIDENT", func(f *Finding) { f.Confidence = 0.8 }),
	)

	assert.Equal(t, []string{"MORE-CONFIDENT", "STRONGER", "TIE-A", "TIE-B", "LATER"}, ids)
}

func TestPriority_IsIndependentOfInputOrder(t *testing.T) {
	findings := []Finding{
		{MeterID: "A", Severity: SeverityHigh, Type: DataQuality, Confidence: 0.9},
		{MeterID: "B", Severity: SeverityHigh, Type: RealAnomaly, Confidence: 0.9},
		{MeterID: "C", Severity: SeverityLow, Type: FalsePositive, Confidence: 0.8},
		{MeterID: "D", Severity: SeverityMedium, Type: ExplainableAnomaly, Confidence: 0.7},
		{MeterID: "E", Severity: SeverityMedium, Type: ExplainableAnomaly, Confidence: 0.7},
	}
	want := ranked(t, slices.Clone(findings)...)

	rng := rand.New(rand.NewPCG(7, 11)) // fixed seed: reproducible permutations
	for range 20 {
		shuffled := slices.Clone(findings)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		assert.Equal(t, want, ranked(t, shuffled...))
	}
}
