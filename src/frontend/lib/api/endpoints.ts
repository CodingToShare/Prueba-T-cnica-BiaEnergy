// One function per API operation (docs/api/openapi.yaml).
import { apiRequest, withQuery } from "./client";
import type {
  AnalysisRun,
  AnalyzeResponse,
  AnomalyDetail,
  AnomalyList,
  AnomalyType,
  ComputedStatus,
  DashboardSummary,
  MeterDetail,
  MeterList,
  ReadingList,
  Session,
  Severity,
} from "./types";

export type MeterSort = "meter_id" | "consumption" | "variation" | "severity";
export type SortOrder = "asc" | "desc";

export interface MeterListParams {
  search?: string;
  status?: ComputedStatus;
  sort: MeterSort;
  order: SortOrder;
  limit: number;
  offset: number;
}

export interface AnomalyListParams {
  meterId?: string;
  type?: AnomalyType;
  severity?: Severity;
  limit: number;
  offset: number;
}

export interface ReadingsParams {
  from?: string;
  to?: string;
  limit?: number;
}

export const api = {
  login: (username: string, password: string) =>
    apiRequest<Session>("/api/v1/auth/login", { method: "POST", body: { username, password } }),
  logout: () => apiRequest<undefined>("/api/v1/auth/logout", { method: "POST" }),
  session: (signal?: AbortSignal) => apiRequest<Session>("/api/v1/auth/session", { signal }),

  dashboard: (signal?: AbortSignal) => apiRequest<DashboardSummary>("/api/v1/dashboard/summary", { signal }),

  meters: (p: MeterListParams, signal?: AbortSignal) =>
    apiRequest<MeterList>(
      withQuery("/api/v1/meters", {
        search: p.search,
        status: p.status,
        sort: p.sort,
        order: p.order,
        limit: p.limit,
        offset: p.offset,
      }),
      { signal },
    ),
  meter: (meterId: string, signal?: AbortSignal) =>
    apiRequest<MeterDetail>(`/api/v1/meters/${encodeURIComponent(meterId)}`, { signal }),
  readings: (meterId: string, p: ReadingsParams, signal?: AbortSignal) =>
    apiRequest<ReadingList>(
      withQuery(`/api/v1/meters/${encodeURIComponent(meterId)}/readings`, { from: p.from, to: p.to, limit: p.limit }),
      { signal },
    ),

  anomalies: (p: AnomalyListParams, signal?: AbortSignal) =>
    apiRequest<AnomalyList>(
      withQuery("/api/v1/anomalies", {
        meter_id: p.meterId,
        type: p.type,
        severity: p.severity,
        limit: p.limit,
        offset: p.offset,
      }),
      { signal },
    ),
  anomaly: (id: number, signal?: AbortSignal) => apiRequest<AnomalyDetail>(`/api/v1/anomalies/${id}`, { signal }),

  analyze: () => apiRequest<AnalyzeResponse>("/api/v1/ai/analyze", { method: "POST" }),
  analysis: (id: number, signal?: AbortSignal) => apiRequest<AnalysisRun>(`/api/v1/ai/analysis/${id}`, { signal }),
};
