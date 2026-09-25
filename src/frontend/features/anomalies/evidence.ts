// Presentation of the structured evidence the backend stored for a finding
// (GET /api/v1/anomalies/{id}). These functions only select and format
// values; they never recompute baselines, deviations or classifications.
import type { AnomalyDetail, ConfidenceBreakdown, Evidence, MetricEvidence, ReadingMetric, RelatedEvent } from "@/lib/api/types";
import { metricLabel } from "@/lib/format/labels";
import { formatCount, formatKwh, formatPercentSigned } from "@/lib/format/numbers";
import { formatDurationHours, formatSourceShort } from "@/lib/format/time";

import type { BaselinePoint, ChartEpisode, ChartEvent } from "@/features/meters/chart/option";

export interface Fact {
  label: string;
  value: string;
}

/** Consumption, persistence and recovery facts for the evidence list. */
export function keyFacts(evidence: Evidence): Fact[] {
  const { consumption, persistence } = evidence;
  const recovery = persistence.recovery;
  const density = Math.round(persistence.density_fraction * 100);
  return [
    { label: "Baseline energy (flagged hours)", value: formatKwh(consumption.baseline_kwh) },
    { label: "Observed energy (flagged hours)", value: formatKwh(consumption.observed_kwh) },
    { label: "Consumption deviation", value: formatPercentSigned(consumption.deviation_pct) },
    { label: "Median hourly deviation", value: formatPercentSigned(consumption.median_deviation_pct) },
    { label: "Duration", value: `${formatDurationHours(persistence.duration_hours)}${persistence.sustained ? " · sustained" : ""}` },
    {
      label: "Flagged readings",
      value: `${formatCount(persistence.flagged_readings)} of ${formatCount(persistence.span_readings)} (${density}%)`,
    },
    {
      label: "Recovery",
      value:
        recovery.recovered && recovery.recovered_at
          ? `Recovered from ${formatSourceShort(recovery.recovered_at)} (${formatPercentSigned(recovery.median_consumption_deviation_pct)} vs baseline)`
          : "Not observed",
    },
  ];
}

export interface VariableRow {
  metric: string;
  label: string;
  change: string;
  flagged: string;
  direction: string;
  supports: boolean;
}

const DIRECTION_LABELS: Record<string, string> = { UP: "Up", DOWN: "Down", MIXED: "Mixed" };

/** Every analyzed variable with its median deviation and whether it supports the finding. */
export function changedVariables(evidence: Evidence): VariableRow[] {
  return evidence.metrics.map((m: MetricEvidence) => ({
    metric: m.metric,
    label: metricLabel(m.metric),
    change: formatPercentSigned(m.median_deviation_pct),
    flagged: `${formatCount(m.triggered_readings)} of ${formatCount(m.evaluated_readings)}`,
    direction: m.direction ? (DIRECTION_LABELS[m.direction] ?? m.direction) : "—",
    supports: m.corroborates,
  }));
}

/** Variables that changed consistently enough to support the finding. */
export function supportingVariables(evidence: Evidence): VariableRow[] {
  return changedVariables(evidence).filter((row) => row.supports);
}

/** True when a correlated event explains the deviation (and none merely context). */
export function hasExplanation(events: RelatedEvent[]): boolean {
  return events.some((e) => e.role === "EXPLAINS");
}

/** "3 h before onset", "at onset", "2 h after onset". */
export function formatOffset(seconds: number): string {
  if (seconds === 0) {
    return "at the onset";
  }
  const hours = Math.abs(seconds) / 3600;
  const amount = Number.isInteger(hours) ? `${hours} h` : `${Math.round(Math.abs(seconds) / 60)} min`;
  return `${amount} ${seconds < 0 ? "before" : "after"} the onset`;
}

export const CONFIDENCE_COMPONENTS: ReadonlyArray<{ key: keyof ConfidenceBreakdown; label: string }> = [
  { key: "signal_strength", label: "Signal strength" },
  { key: "persistence", label: "Persistence" },
  { key: "multivariate_support", label: "Multivariate support" },
  { key: "event_context", label: "Event context" },
  { key: "pattern_support", label: "Supporting pattern" },
];

/** Chart overlays for a finding: its episode, its correlated events and the engine's baseline for `metric`. */
export function chartOverlays(
  detail: AnomalyDetail,
  metric: ReadingMetric,
): { episode: ChartEpisode; events: ChartEvent[]; baseline: BaselinePoint[] } {
  return {
    episode: { start: detail.started_at, end: detail.last_observed_at },
    events: detail.evidence.related_events.map((e) => ({ timestamp: e.timestamp, label: e.type })),
    baseline: detail.evidence.signals
      .filter((s) => s.metric === metric)
      .map((s) => ({ timestamp: s.timestamp, value: s.baseline })),
  };
}
