package analysis

import (
	"cmp"
	"slices"
)

// ComparePriority orders findings for investigation (OD-05). It returns a
// negative number when a comes first. The order is:
//  1. severity (HIGH, MEDIUM, LOW);
//  2. classification by operational risk (REAL_ANOMALY, DATA_QUALITY,
//     EXPLAINABLE_ANOMALY, FALSE_POSITIVE);
//  3. confidence, descending;
//  4. evidence strength, descending;
//  5. earlier onset, then meter ID, only to keep ties deterministic.
func ComparePriority(a, b Finding) int {
	return cmp.Or(
		cmp.Compare(severityRank(b.Severity), severityRank(a.Severity)),
		cmp.Compare(classificationRank(b.Type), classificationRank(a.Type)),
		cmp.Compare(b.Confidence, a.Confidence),
		cmp.Compare(b.EvidenceStrength, a.EvidenceStrength),
		a.StartedAt.Compare(b.StartedAt),
		cmp.Compare(a.MeterID, b.MeterID),
	)
}

// prioritize sorts findings and numbers them from 1.
func prioritize(findings []Finding) {
	slices.SortStableFunc(findings, ComparePriority)
	for i := range findings {
		findings[i].Priority = i + 1
	}
}

func severityRank(s Severity) int {
	switch s {
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	default:
		return 0
	}
}

func classificationRank(c Classification) int {
	switch c {
	case RealAnomaly:
		return 4
	case DataQuality:
		return 3
	case ExplainableAnomaly:
		return 2
	case FalsePositive:
		return 1
	default:
		return 0
	}
}
