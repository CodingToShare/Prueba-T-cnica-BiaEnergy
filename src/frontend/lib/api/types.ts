// Aliases over the types generated from docs/api/openapi.yaml
// (lib/api/schema.d.ts, `pnpm generate:api`). The OpenAPI document is the
// single source of truth for API shapes; nothing here redefines them.
import type { components } from "./schema";

type Schemas = components["schemas"];

export type Session = Schemas["Session"];
export type Pagination = Schemas["Pagination"];
export type AnalysisRef = Schemas["AnalysisRef"];
export type Period = Schemas["Period"];
export type ComputedStatus = Schemas["ComputedStatus"];
export type AnomalyType = Schemas["AnomalyType"];
export type Severity = Schemas["Severity"];
export type RecommendedAction = Schemas["RecommendedAction"];
export type MeterSummary = Schemas["MeterSummary"];
export type MeterList = Schemas["MeterList"];
export type MeterFinding = Schemas["MeterFinding"];
export type MeterDetail = Schemas["MeterDetail"];
export type Reading = Schemas["Reading"];
export type ReadingList = Schemas["ReadingList"];
export type AnomalySummary = Schemas["AnomalySummary"];
export type AnomalyList = Schemas["AnomalyList"];
export type AnomalyDetail = Schemas["AnomalyDetail"];
export type Evidence = Schemas["Evidence"];
export type MetricEvidence = Evidence["metrics"][number];
export type RelatedEvent = Evidence["related_events"][number];
export type Signal = Evidence["signals"][number];
export type ConfidenceBreakdown = Evidence["confidence_breakdown"];
export type AnalysisRun = Schemas["AnalysisRun"];
export type AnalyzeResponse = Schemas["AnalyzeResponse"];
export type RunStatus = AnalysisRun["status"];
export type RunStage = AnalysisRun["stage"];
export type DashboardSummary = Schemas["DashboardSummary"];

/** Metric names used by readings and evidence. */
export type ReadingMetric = "consumption_kwh" | "voltage_v" | "current_a" | "power_factor";
