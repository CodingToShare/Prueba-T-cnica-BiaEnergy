package httpapi

import (
	"time"

	"bia-energy.local/backend/internal/analysisrun"
	"bia-energy.local/backend/internal/anomaly"
	"bia-energy.local/backend/internal/auth"
	"bia-energy.local/backend/internal/dashboard"
	"bia-energy.local/backend/internal/meter"
	"bia-energy.local/backend/internal/platform/jsontime"
)

// Response bodies (snake_case). Source times use jsontime.Source (no
// offset); system instants use jsontime.System (RFC 3339 UTC). A missing
// analytical value is null, never a fabricated zero.

type paginationDTO struct {
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
	Total  int64 `json:"total"`
}

type analysisRefDTO struct {
	ID          int64           `json:"id"`
	Status      string          `json:"status"`
	CompletedAt jsontime.System `json:"completed_at"`
}

func analysisRef(c *analysisrun.CompletedRun) *analysisRefDTO {
	if c == nil {
		return nil
	}
	return &analysisRefDTO{ID: c.ID, Status: string(analysisrun.StatusCompleted), CompletedAt: jsontime.System(c.CompletedAt)}
}

type periodDTO struct {
	FirstReadingAt jsontime.Source `json:"first_reading_at"`
	LastReadingAt  jsontime.Source `json:"last_reading_at"`
}

func period(first, last *time.Time) *periodDTO {
	if first == nil || last == nil {
		return nil
	}
	return &periodDTO{FirstReadingAt: jsontime.Source(*first), LastReadingAt: jsontime.Source(*last)}
}

type sessionDTO struct {
	Authenticated bool            `json:"authenticated"`
	Username      string          `json:"username"`
	ExpiresAt     jsontime.System `json:"expires_at"`
}

func sessionBody(s auth.Session) sessionDTO {
	return sessionDTO{Authenticated: true, Username: s.Subject, ExpiresAt: jsontime.System(s.ExpiresAt)}
}

type meterSummaryDTO struct {
	MeterID             string   `json:"meter_id"`
	TotalConsumptionKWh float64  `json:"total_consumption_kwh"`
	VariationPct        *float64 `json:"variation_pct"`
	ComputedStatus      *string  `json:"computed_status"`
	AnomalyID           *int64   `json:"anomaly_id"`
	AnomalyType         *string  `json:"anomaly_type"`
	Severity            *string  `json:"severity"`
	Confidence          *float64 `json:"confidence"`
	Priority            *int     `json:"priority"`
}

type meterListDTO struct {
	Items      []meterSummaryDTO `json:"items"`
	Pagination paginationDTO     `json:"pagination"`
	Analysis   *analysisRefDTO   `json:"analysis"`
}

func meterList(p meter.Page, limit, offset int) meterListDTO {
	out := meterListDTO{
		Items:      make([]meterSummaryDTO, len(p.Items)),
		Pagination: paginationDTO{Limit: limit, Offset: offset, Total: p.Total},
		Analysis:   analysisRef(p.Analysis),
	}
	for i, m := range p.Items {
		out.Items[i] = meterSummaryDTO{
			MeterID: m.MeterID, TotalConsumptionKWh: m.TotalConsumptionKWh, VariationPct: m.VariationPct,
			ComputedStatus: m.ComputedStatus, AnomalyID: m.AnomalyID, AnomalyType: m.AnomalyType,
			Severity: m.Severity, Confidence: m.Confidence, Priority: m.Priority,
		}
	}
	return out
}

type meterFindingDTO struct {
	AnomalyID         int64           `json:"anomaly_id"`
	Type              string          `json:"type"`
	Severity          string          `json:"severity"`
	Confidence        float64         `json:"confidence"`
	Priority          int             `json:"priority"`
	VariationPct      float64         `json:"variation_pct"`
	StartedAt         jsontime.Source `json:"started_at"`
	LastObservedAt    jsontime.Source `json:"last_observed_at"`
	RecommendedAction string          `json:"recommended_action"`
	Reason            string          `json:"reason"`
}

type meterDetailDTO struct {
	MeterID             string           `json:"meter_id"`
	TotalConsumptionKWh float64          `json:"total_consumption_kwh"`
	ReadingsCount       int64            `json:"readings_count"`
	Period              *periodDTO       `json:"period"`
	ComputedStatus      *string          `json:"computed_status"`
	FindingsCount       int              `json:"findings_count"`
	CurrentFinding      *meterFindingDTO `json:"current_finding"`
	Analysis            *analysisRefDTO  `json:"analysis"`
}

func meterDetail(d meter.Detail) meterDetailDTO {
	out := meterDetailDTO{
		MeterID: d.MeterID, TotalConsumptionKWh: d.TotalConsumptionKWh, ReadingsCount: d.ReadingsCount,
		Period: period(d.FirstReadingAt, d.LastReadingAt), ComputedStatus: d.ComputedStatus,
		FindingsCount: d.FindingsCount, Analysis: analysisRef(d.Analysis),
	}
	if f := d.CurrentFinding; f != nil {
		out.CurrentFinding = &meterFindingDTO{
			AnomalyID: f.AnomalyID, Type: f.Type, Severity: f.Severity, Confidence: f.Confidence,
			Priority: f.Priority, VariationPct: f.VariationPct,
			StartedAt: jsontime.Source(f.StartedAt), LastObservedAt: jsontime.Source(f.LastObservedAt),
			RecommendedAction: f.RecommendedAction, Reason: f.Reason,
		}
	}
	return out
}

type readingDTO struct {
	Timestamp      jsontime.Source `json:"timestamp"`
	ConsumptionKWh float64         `json:"consumption_kwh"`
	VoltageV       float64         `json:"voltage_v"`
	CurrentA       float64         `json:"current_a"`
	PowerFactor    float64         `json:"power_factor"`
	SourceStatus   string          `json:"source_status"`
}

type readingsDTO struct {
	MeterID string           `json:"meter_id"`
	From    *jsontime.Source `json:"from"`
	To      *jsontime.Source `json:"to"`
	Limit   int              `json:"limit"`
	HasMore bool             `json:"has_more"`
	Items   []readingDTO     `json:"items"`
}

func readings(meterID string, p meter.ReadingsParams, rs []meter.Reading, more bool) readingsDTO {
	out := readingsDTO{
		MeterID: meterID, From: jsontime.SourcePtr(p.From), To: jsontime.SourcePtr(p.To),
		Limit: p.Limit, HasMore: more, Items: make([]readingDTO, len(rs)),
	}
	for i, r := range rs {
		out.Items[i] = readingDTO{
			Timestamp: jsontime.Source(r.Timestamp), ConsumptionKWh: r.ConsumptionKWh, VoltageV: r.VoltageV,
			CurrentA: r.CurrentA, PowerFactor: r.PowerFactor, SourceStatus: r.SourceStatus,
		}
	}
	return out
}

type anomalySummaryDTO struct {
	ID                int64           `json:"id"`
	AnalysisID        int64           `json:"analysis_id"`
	MeterID           string          `json:"meter_id"`
	Priority          int             `json:"priority"`
	Type              string          `json:"type"`
	Severity          string          `json:"severity"`
	Confidence        float64         `json:"confidence"`
	Status            string          `json:"status"`
	StartedAt         jsontime.Source `json:"started_at"`
	LastObservedAt    jsontime.Source `json:"last_observed_at"`
	DurationHours     float64         `json:"duration_hours"`
	VariationPct      float64         `json:"variation_pct"`
	Reason            string          `json:"reason"`
	RecommendedAction string          `json:"recommended_action"`
}

func anomalySummary(a anomaly.Summary) anomalySummaryDTO {
	return anomalySummaryDTO{
		ID: a.ID, AnalysisID: a.AnalysisID, MeterID: a.MeterID, Priority: a.Priority, Type: a.Type,
		Severity: a.Severity, Confidence: a.Confidence, Status: a.Status,
		StartedAt: jsontime.Source(a.StartedAt), LastObservedAt: jsontime.Source(a.LastObservedAt),
		DurationHours: float64(a.DurationSeconds) / 3600, VariationPct: a.VariationPct,
		Reason: a.Reason, RecommendedAction: a.RecommendedAction,
	}
}

type anomalyListDTO struct {
	Items      []anomalySummaryDTO `json:"items"`
	Pagination paginationDTO       `json:"pagination"`
	Analysis   *analysisRefDTO     `json:"analysis"`
}

func anomalyList(p anomaly.Page, limit, offset int) anomalyListDTO {
	out := anomalyListDTO{
		Items:      make([]anomalySummaryDTO, len(p.Items)),
		Pagination: paginationDTO{Limit: limit, Offset: offset, Total: p.Total},
		Analysis:   analysisRef(p.Analysis),
	}
	for i, a := range p.Items {
		out.Items[i] = anomalySummary(a)
	}
	return out
}

type anomalyDetailDTO struct {
	anomalySummaryDTO
	Rule        string               `json:"rule"`
	CreatedAt   jsontime.System      `json:"created_at"`
	Evidence    analysisrun.Evidence `json:"evidence"`
	Explanation *explanationDTO      `json:"explanation"`
}

// explanationDTO is the operator-facing explanation and its provenance.
type explanationDTO struct {
	Source                string          `json:"source"`
	Summary               string          `json:"summary"`
	WhyItMatters          string          `json:"why_it_matters"`
	EvidenceNarrative     string          `json:"evidence_narrative"`
	RecommendedActionText string          `json:"recommended_action_text"`
	Model                 *string         `json:"model"`
	PromptVersion         string          `json:"prompt_version"`
	GeneratedAt           jsontime.System `json:"generated_at"`
	FallbackUsed          bool            `json:"fallback_used"`
}

func anomalyDetail(d anomaly.Detail) anomalyDetailDTO {
	out := anomalyDetailDTO{
		anomalySummaryDTO: anomalySummary(d.Summary), Rule: d.Rule,
		CreatedAt: jsontime.System(d.CreatedAt), Evidence: d.Evidence,
	}
	if e := d.Explanation; e != nil {
		out.Explanation = &explanationDTO{
			Source: e.Source, Summary: e.Summary, WhyItMatters: e.WhyItMatters,
			EvidenceNarrative: e.EvidenceNarrative, RecommendedActionText: e.RecommendedActionText,
			Model: e.Model, PromptVersion: e.PromptVersion,
			GeneratedAt: jsontime.System(e.GeneratedAt), FallbackUsed: e.FallbackUsed,
		}
	}
	return out
}

type runErrorDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type runDTO struct {
	AnalysisID          int64            `json:"analysis_id"`
	Status              string           `json:"status"`
	Stage               string           `json:"stage"`
	Progress            int              `json:"progress"`
	EngineVersion       string           `json:"engine_version"`
	CreatedAt           jsontime.System  `json:"created_at"`
	StartedAt           *jsontime.System `json:"started_at"`
	CompletedAt         *jsontime.System `json:"completed_at"`
	MetersCount         *int             `json:"meters_count"`
	ReadingsCount       *int             `json:"readings_count"`
	EventsCount         *int             `json:"events_count"`
	FindingsCount       *int             `json:"findings_count"`
	HighPriorityCount   *int             `json:"high_priority_count"`
	AggregateConfidence *float64         `json:"aggregate_confidence"`
	Error               *runErrorDTO     `json:"error"`
}

func runBody(r analysisrun.Run) runDTO {
	out := runDTO{
		AnalysisID: r.ID, Status: string(r.Status), Stage: string(r.Stage), Progress: r.Progress,
		EngineVersion: r.EngineVersion, CreatedAt: jsontime.System(r.CreatedAt),
		StartedAt: jsontime.SystemPtr(r.StartedAt), CompletedAt: jsontime.SystemPtr(r.CompletedAt),
		MetersCount: r.MetersCount, ReadingsCount: r.ReadingsCount, EventsCount: r.EventsCount,
		FindingsCount: r.FindingsCount, HighPriorityCount: r.HighPriorityCount,
		AggregateConfidence: r.AggregateConfidence,
	}
	if r.ErrorCode != nil && r.ErrorMessage != nil {
		out.Error = &runErrorDTO{Code: *r.ErrorCode, Message: *r.ErrorMessage}
	}
	return out
}

type analyzeDTO struct {
	runDTO
	// Created is false when an already active run was returned.
	Created bool `json:"created"`
}

type activeAnalysisDTO struct {
	ID       int64  `json:"id"`
	Status   string `json:"status"`
	Stage    string `json:"stage"`
	Progress int    `json:"progress"`
}

type dashboardDTO struct {
	Meters              int64              `json:"meters"`
	Readings            int64              `json:"readings"`
	TotalConsumptionKWh float64            `json:"total_consumption_kwh"`
	Period              *periodDTO         `json:"period"`
	Anomalies           int                `json:"anomalies"`
	HighPriority        int                `json:"high_priority"`
	AggregateConfidence *float64           `json:"aggregate_confidence"`
	LatestAnalysis      *analysisRefDTO    `json:"latest_analysis"`
	ActiveAnalysis      *activeAnalysisDTO `json:"active_analysis"`
}

func dashboardBody(s dashboard.Summary) dashboardDTO {
	out := dashboardDTO{
		Meters: s.Meters, Readings: s.Readings, TotalConsumptionKWh: s.TotalConsumptionKWh,
		Period: period(s.FirstReadingAt, s.LastReadingAt), Anomalies: s.Anomalies, HighPriority: s.HighPriority,
		AggregateConfidence: s.AggregateConfidence, LatestAnalysis: analysisRef(s.LatestAnalysis),
	}
	if a := s.ActiveAnalysis; a != nil {
		out.ActiveAnalysis = &activeAnalysisDTO{ID: a.ID, Status: string(a.Status), Stage: string(a.Stage), Progress: a.Progress}
	}
	return out
}
