// All TanStack Query keys, in one place. Prefix keys (`meters.all`, ...)
// are what a completed analysis invalidates.
import type { AnomalyListParams, MeterListParams, ReadingsParams } from "@/lib/api/endpoints";

export const queryKeys = {
  session: ["session"] as const,
  dashboard: ["dashboard"] as const,
  meters: {
    all: ["meters"] as const,
    list: (params: MeterListParams) => ["meters", "list", params] as const,
    detail: (meterId: string) => ["meters", "detail", meterId] as const,
    readings: (meterId: string, params: ReadingsParams) => ["meters", "readings", meterId, params] as const,
  },
  anomalies: {
    all: ["anomalies"] as const,
    list: (params: AnomalyListParams) => ["anomalies", "list", params] as const,
    detail: (id: number) => ["anomalies", "detail", id] as const,
  },
  analysis: (id: number) => ["analysis", id] as const,
};
