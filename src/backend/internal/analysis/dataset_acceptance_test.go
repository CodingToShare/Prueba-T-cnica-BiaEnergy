package analysis_test

import (
	"context"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/analysis"
	"bia-energy.local/backend/internal/ingestion"
	"bia-energy.local/backend/internal/workspace"
)

// Acceptance on the supplied dataset: data/input CSVs → Phase 01 parser →
// the production engine with its default configuration. The meter IDs below
// are expected outcomes of the challenge; they appear only in this test.

func loadSupplied(tb testing.TB) ([]analysis.Reading, []analysis.Event) {
	tb.Helper()
	dir, err := workspace.FindDir("data/input")
	require.NoError(tb, err)
	ds, err := ingestion.ParseDir(dir)
	require.NoError(tb, err)

	readings := make([]analysis.Reading, len(ds.Readings))
	for i, r := range ds.Readings {
		readings[i] = analysis.Reading{
			MeterID: r.MeterID, Timestamp: r.Timestamp,
			ConsumptionKWh: r.ConsumptionKWh, VoltageV: r.VoltageV, CurrentA: r.CurrentA, PowerFactor: r.PowerFactor,
		}
	}
	events := make([]analysis.Event, len(ds.Events))
	for i, e := range ds.Events {
		events[i] = analysis.Event{MeterID: e.MeterID, Timestamp: e.Timestamp, Type: analysis.EventType(e.Type), Description: e.Description}
	}
	return readings, events
}

func run(t *testing.T, cfg analysis.Config, readings []analysis.Reading, events []analysis.Event) analysis.Result {
	t.Helper()
	engine, err := analysis.New(cfg)
	require.NoError(t, err)
	res, err := engine.Analyze(context.Background(), readings, events)
	require.NoError(t, err)
	return res
}

func byMeter(res analysis.Result) map[string]analysis.Finding {
	out := map[string]analysis.Finding{}
	for _, f := range res.Findings {
		out[f.MeterID] = f
	}
	return out
}

func metricOf(f analysis.Finding, m analysis.Metric) analysis.MetricEvidence {
	for _, me := range f.Metrics {
		if me.Metric == m {
			return me
		}
	}
	return analysis.MetricEvidence{}
}

func eventOf(f analysis.Finding, ty analysis.EventType) (analysis.RelatedEvent, bool) {
	for _, re := range f.RelatedEvents {
		if re.Type == ty {
			return re, true
		}
	}
	return analysis.RelatedEvent{}, false
}

func TestAcceptance_SuppliedDataset_ClassifiesTheFourScenariosWithEvidence(t *testing.T) {
	readings, events := loadSupplied(t)
	cfg := analysis.DefaultConfig()

	res := run(t, cfg, readings, events)

	require.Len(t, res.Findings, 4, "exactly the four challenge scenarios are reportable")
	findings := byMeter(res)

	t.Run("M-109 is an unexplained, corroborated, persistent real anomaly", func(t *testing.T) {
		f := findings["M-109"]
		assert.Equal(t, analysis.RealAnomaly, f.Type)
		assert.Equal(t, analysis.SeverityHigh, f.Severity)
		assert.Equal(t, analysis.DirectionUp, f.Consumption.Direction)
		assert.Greater(t, f.Consumption.MedianDeviationPct, 0.9, "consumption roughly doubles")
		assert.Greater(t, f.Consumption.ObservedKWh, 2*f.Consumption.BaselineKWh*0.95)
		assert.True(t, f.Persistence.Sustained)
		assert.GreaterOrEqual(t, f.Persistence.FlaggedReadings, 48)
		assert.False(t, f.Persistence.Recovery.Recovered)

		current := metricOf(f, analysis.MetricCurrent)
		assert.True(t, current.Corroborates)
		assert.Equal(t, analysis.DirectionUp, current.Direction, "current follows the load")
		pf := metricOf(f, analysis.MetricPowerFactor)
		assert.True(t, pf.Corroborates)
		assert.Equal(t, analysis.DirectionDown, pf.Direction, "power factor degrades")
		assert.Less(t, pf.MedianDeviationPct, -0.15)

		unknown, ok := eventOf(f, analysis.EventUnknown)
		require.True(t, ok, "the UNKNOWN event is preserved as context")
		assert.Equal(t, analysis.RoleContext, unknown.Role)
		for _, re := range f.RelatedEvents {
			assert.NotEqual(t, analysis.RoleExplains, re.Role, "nothing explains it")
		}
		assert.GreaterOrEqual(t, f.Confidence, 0.85)
		assert.Equal(t, analysis.ActionInvestigateMeterAndInstallation, f.RecommendedAction)
		assert.Contains(t, f.Reason, "does not explain it")
	})

	t.Run("M-104 is a visible, explained persistent change", func(t *testing.T) {
		f := findings["M-104"]
		assert.Equal(t, analysis.ExplainableAnomaly, f.Type)
		assert.Equal(t, analysis.SeverityMedium, f.Severity)
		assert.Greater(t, f.Consumption.MedianDeviationPct, cfg.MinDeviation.Consumption, "the deviation stays visible")
		assert.True(t, f.Persistence.Sustained)
		assert.False(t, f.Persistence.Recovery.Recovered, "the new level persists")

		ev, ok := eventOf(f, analysis.EventOperationalChange)
		require.True(t, ok)
		assert.Equal(t, analysis.RoleExplains, ev.Role)
		assert.LessOrEqual(t, ev.Offset.Abs(), cfg.EventWindow, "it begins near the event")
		assert.Equal(t, "New production line activated", ev.Description, "the description is kept as evidence")
		assert.Equal(t, analysis.ActionValidateOperationalChange, f.RecommendedAction)
	})

	t.Run("M-106 is detected, explained and recovered, so not escalated", func(t *testing.T) {
		f := findings["M-106"]
		assert.Equal(t, analysis.FalsePositive, f.Type)
		assert.Equal(t, analysis.SeverityLow, f.Severity)
		assert.Equal(t, analysis.DirectionDown, f.Consumption.Direction)
		assert.Less(t, f.Consumption.MedianDeviationPct, -0.5, "the detector saw a deep drop")
		assert.NotEmpty(t, f.Signals)

		ev, ok := eventOf(f, analysis.EventScheduledOutage)
		require.True(t, ok)
		assert.Equal(t, analysis.RoleExplains, ev.Role)
		assert.True(t, f.Persistence.Recovery.Recovered)
		assert.Less(t, math.Abs(f.Persistence.Recovery.MedianConsumptionDeviationPct), cfg.MinDeviation.Consumption)
		assert.True(t, f.Persistence.Recovery.RecoveredAt.After(f.LastObservedAt))
		assert.GreaterOrEqual(t, f.Confidence, 0.6, "a strong explanation, despite LOW severity")
		assert.Equal(t, analysis.ActionNoEscalationMonitor, f.RecommendedAction)
	})

	t.Run("M-112 is a data-quality problem with consumption on its baseline", func(t *testing.T) {
		f := findings["M-112"]
		assert.Equal(t, analysis.DataQuality, f.Type)
		assert.Equal(t, analysis.SeverityHigh, f.Severity)

		consumption := metricOf(f, analysis.MetricConsumption)
		assert.Zero(t, consumption.TriggeredReadings, "consumption never deviates")
		assert.Less(t, math.Abs(f.Consumption.DeviationPct), 0.1)

		affected := 0
		for _, m := range []analysis.Metric{analysis.MetricVoltage, analysis.MetricCurrent, analysis.MetricPowerFactor, analysis.MetricLoadRatio} {
			if metricOf(f, m).Corroborates {
				affected++
			}
		}
		assert.GreaterOrEqual(t, affected, 3, "several electrical variables are inconsistent")
		assert.Positive(t, metricOf(f, analysis.MetricLoadRatio).TriggeredReadings, "the relationship to consumption breaks")
		assert.GreaterOrEqual(t, f.Persistence.FlaggedReadings, 10, "repeated, not a single glitch")

		ev, ok := eventOf(f, analysis.EventDataQuality)
		require.True(t, ok)
		assert.Equal(t, analysis.RoleCorroborates, ev.Role)
		assert.GreaterOrEqual(t, f.Confidence, 0.85)
		assert.Equal(t, analysis.ActionValidateMeasurementOrSensor, f.RecommendedAction)
	})
}

func TestAcceptance_SuppliedDataset_PrioritizesTheRealAnomalyFirst(t *testing.T) {
	readings, events := loadSupplied(t)

	res := run(t, analysis.DefaultConfig(), readings, events)

	var order []string
	for i, f := range res.Findings {
		order = append(order, f.MeterID)
		assert.Equal(t, i+1, f.Priority)
		if i > 0 {
			assert.LessOrEqual(t, analysis.ComparePriority(res.Findings[i-1], f), 0)
		}
	}
	assert.Equal(t, []string{"M-109", "M-112", "M-104", "M-106"}, order)
}

func TestAcceptance_SuppliedDataset_ControlMetersStayQuiet(t *testing.T) {
	readings, events := loadSupplied(t)

	res := run(t, analysis.DefaultConfig(), readings, events)

	statuses := map[string]analysis.MeterStatus{}
	for _, m := range res.Meters {
		statuses[m.MeterID] = m.Status
		assert.Equal(t, 336, m.Readings)
		assert.Equal(t, 168, m.EvaluatedReadings, "days 8–14 have a mature baseline")
		switch m.MeterID {
		case "M-104", "M-106", "M-109", "M-112":
		default:
			assert.Zero(t, m.FlaggedReadings, "%s: no reading of a stable meter passes the dual gate", m.MeterID)
			assert.Zero(t, m.Findings, m.MeterID)
		}
	}
	assert.Equal(t, analysis.StatusCritical, statuses["M-109"])
	assert.Equal(t, analysis.StatusAlert, statuses["M-112"])
	assert.Equal(t, analysis.StatusAlert, statuses["M-104"])
	assert.Equal(t, analysis.StatusOK, statuses["M-106"])
}

func TestAcceptance_SuppliedDataset_EventCorrelationChangesTheClassification(t *testing.T) {
	readings, events := loadSupplied(t)
	withoutExplanations := slices.DeleteFunc(slices.Clone(events), func(e analysis.Event) bool {
		return e.MeterID == "M-106" || e.MeterID == "M-104"
	})

	findings := byMeter(run(t, analysis.DefaultConfig(), readings, withoutExplanations))

	assert.Equal(t, analysis.RealAnomaly, findings["M-106"].Type, "without the outage the drop is unexplained")
	assert.Equal(t, analysis.RealAnomaly, findings["M-104"].Type, "without the operational change the rise is unexplained")
}

func TestAcceptance_SuppliedDataset_PerturbationKeepsTheResult(t *testing.T) {
	readings, events := loadSupplied(t)
	shift := 37*24*time.Hour + 5*time.Hour
	rename := func(id string) string { return "PX-" + strings.TrimPrefix(id, "M-") + "-Z" }
	perturbedReadings := slices.Clone(readings)
	for i := range perturbedReadings {
		perturbedReadings[i].MeterID = rename(perturbedReadings[i].MeterID)
		perturbedReadings[i].Timestamp = perturbedReadings[i].Timestamp.Add(shift)
	}
	perturbedEvents := slices.Clone(events)
	for i := range perturbedEvents {
		perturbedEvents[i].MeterID = rename(perturbedEvents[i].MeterID)
		perturbedEvents[i].Timestamp = perturbedEvents[i].Timestamp.Add(shift)
	}

	original := run(t, analysis.DefaultConfig(), readings, events)
	perturbed := run(t, analysis.DefaultConfig(), perturbedReadings, perturbedEvents)

	require.Len(t, perturbed.Findings, len(original.Findings))
	for i, o := range original.Findings {
		p := perturbed.Findings[i]
		assert.Equal(t, rename(o.MeterID), p.MeterID)
		assert.Equal(t, o.Type, p.Type)
		assert.Equal(t, o.Severity, p.Severity)
		assert.InDelta(t, o.Confidence, p.Confidence, 1e-12)
		assert.Equal(t, o.StartedAt.Add(shift), p.StartedAt)
	}
}

func TestAcceptance_SuppliedDataset_IsStableUnderThresholdChanges(t *testing.T) {
	// Every calibration value moved by ±20% must give the same outcome; the
	// defaults are not balanced on a knife edge of this dataset.
	readings, events := loadSupplied(t)
	want := outcome(run(t, analysis.DefaultConfig(), readings, events))
	changes := map[string]func(*analysis.Config, float64){
		"consumption":  func(c *analysis.Config, k float64) { c.MinDeviation.Consumption *= k },
		"voltage":      func(c *analysis.Config, k float64) { c.MinDeviation.Voltage *= k },
		"current":      func(c *analysis.Config, k float64) { c.MinDeviation.Current *= k },
		"power factor": func(c *analysis.Config, k float64) { c.MinDeviation.PowerFactor *= k },
		"load ratio":   func(c *analysis.Config, k float64) { c.MinDeviation.LoadRatio *= k },
		"robust z":     func(c *analysis.Config, k float64) { c.MinRobustZ *= k },
		"event window": func(c *analysis.Config, k float64) {
			c.EventWindow = time.Duration(float64(c.EventWindow) * k)
		},
		"sustained duration": func(c *analysis.Config, k float64) {
			c.SustainedDuration = time.Duration(float64(c.SustainedDuration) * k)
		},
		"high deviation": func(c *analysis.Config, k float64) { c.HighDeviation *= k },
	}
	for name, change := range changes {
		for _, k := range []float64{0.8, 1.2} {
			cfg := analysis.DefaultConfig()
			change(&cfg, k)

			assert.Equal(t, want, outcome(run(t, cfg, readings, events)), "%s ×%.1f", name, k)
		}
	}
}

func TestAcceptance_SuppliedDataset_CoherentRelativeThresholdChanges(t *testing.T) {
	readings, events := loadSupplied(t)
	want := outcome(run(t, analysis.DefaultConfig(), readings, events))
	for _, k := range []float64{0.8, 1.2} {
		cfg := analysis.DefaultConfig()
		cfg.MinDeviation.Consumption *= k
		cfg.MinDeviation.Voltage *= k
		cfg.MinDeviation.Current *= k
		cfg.MinDeviation.PowerFactor *= k
		cfg.MinDeviation.LoadRatio *= k
		assert.Equal(t, want, outcome(run(t, cfg, readings, events)), "all relative thresholds ×%.1f", k)
	}
}

func TestAcceptance_SuppliedDataset_GapSensitivityExposesIntermittentPattern(t *testing.T) {
	readings, events := loadSupplied(t)
	for _, k := range []float64{0.8, 1.2} {
		cfg := analysis.DefaultConfig()
		cfg.MaxGap = time.Duration(float64(cfg.MaxGap) * k)
		result := run(t, cfg, readings, events)
		if k < 1 {
			assert.Len(t, result.Findings, 3, "a 2.4h gap splits the every-third-hour inconsistency into singletons")
			_, found := byMeter(result)["M-112"]
			assert.False(t, found)
		} else {
			assert.Equal(t, outcome(run(t, analysis.DefaultConfig(), readings, events)), outcome(result))
		}
	}
}

func TestAcceptance_SuppliedDataset_WholeDayShiftAndRenamePreserveAllEvidence(t *testing.T) {
	readings, events := loadSupplied(t)
	original := run(t, analysis.DefaultConfig(), readings, events)
	shift := 37 * 24 * time.Hour
	rename := func(id string) string { return "FICTIONAL-" + id }
	for i := range readings {
		readings[i].MeterID = rename(readings[i].MeterID)
		readings[i].Timestamp = readings[i].Timestamp.Add(shift)
	}
	for i := range events {
		events[i].MeterID = rename(events[i].MeterID)
		events[i].Timestamp = events[i].Timestamp.Add(shift)
	}
	got := run(t, analysis.DefaultConfig(), readings, events)
	require.Len(t, got.Findings, len(original.Findings))
	for i := range got.Meters {
		got.Meters[i].MeterID = strings.TrimPrefix(got.Meters[i].MeterID, "FICTIONAL-")
	}
	for i := range got.Findings {
		f := &got.Findings[i]
		f.MeterID = strings.TrimPrefix(f.MeterID, "FICTIONAL-")
		undo := func(ts time.Time) time.Time {
			if ts.IsZero() {
				return ts
			}
			unshifted := ts.Add(-shift)
			f.Reason = strings.ReplaceAll(f.Reason, ts.Format("2006-01-02 15:04"), unshifted.Format("2006-01-02 15:04"))
			return unshifted
		}
		f.StartedAt = undo(f.StartedAt)
		f.LastObservedAt = undo(f.LastObservedAt)
		f.Persistence.Recovery.RecoveredAt = undo(f.Persistence.Recovery.RecoveredAt)
		for j := range f.RelatedEvents {
			f.RelatedEvents[j].Timestamp = undo(f.RelatedEvents[j].Timestamp)
		}
		for j := range f.Signals {
			f.Signals[j].Timestamp = undo(f.Signals[j].Timestamp)
		}
	}
	assert.Equal(t, original, got)
}

func outcome(res analysis.Result) []string {
	var out []string
	for _, f := range res.Findings {
		out = append(out, f.MeterID+":"+string(f.Type)+"/"+string(f.Severity))
	}
	return out
}

func TestAcceptance_SuppliedDataset_IsDeterministic(t *testing.T) {
	readings, events := loadSupplied(t)
	want := run(t, analysis.DefaultConfig(), readings, events)

	rng := rand.New(rand.NewPCG(19, 23)) // fixed seed: reproducible permutations
	for range 3 {
		r, e := slices.Clone(readings), slices.Clone(events)
		rng.Shuffle(len(r), func(i, j int) { r[i], r[j] = r[j], r[i] })
		rng.Shuffle(len(e), func(i, j int) { e[i], e[j] = e[j], e[i] })

		assert.Equal(t, want, run(t, analysis.DefaultConfig(), r, e))
	}
}

// BenchmarkAnalyze_SuppliedDataset records the analysis duration of the
// 4,032-reading dataset (evidence only; no performance claim beyond it).
func BenchmarkAnalyze_SuppliedDataset(b *testing.B) {
	readings, events := loadSupplied(b)
	engine, err := analysis.New(analysis.DefaultConfig())
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := engine.Analyze(context.Background(), readings, events); err != nil {
			b.Fatal(err)
		}
	}
}
