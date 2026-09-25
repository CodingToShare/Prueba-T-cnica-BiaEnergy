// Builds the ECharts option for a meter's readings. Pure and deterministic:
// it only arranges backend data. No baseline is computed here — the
// optional baseline points are the engine's own evidence (signals) for the
// flagged hours, and the episode and event markers come from the finding.
import type { Reading, ReadingMetric } from "@/lib/api/types";
import { metricLabel } from "@/lib/format/labels";
import { formatMeasurement } from "@/lib/format/numbers";
import { formatSourceDay, formatSourceShort, parseSourceTime } from "@/lib/format/time";

export interface ChartTheme {
  series: string;
  baseline: string;
  anomalyRegion: string;
  anomalyBorder: string;
  eventMarker: string;
  grid: string;
  axis: string;
  text: string;
  tooltipBg: string;
  tooltipText: string;
  font: string;
}

export interface ChartEpisode {
  start: string; // source time
  end: string; // source time
}

export interface ChartEvent {
  timestamp: string; // source time
  label: string;
}

export interface BaselinePoint {
  timestamp: string; // source time
  value: number;
}

export interface ReadingsChartInput {
  readings: Reading[];
  metric: ReadingMetric;
  episode?: ChartEpisode;
  events?: ChartEvent[];
  baseline?: BaselinePoint[];
}

export const METRIC_UNITS: Record<ReadingMetric, string> = {
  consumption_kwh: "kWh",
  voltage_v: "V",
  current_a: "A",
  power_factor: "",
};

/** Escapes text placed in ECharts' HTML tooltip. */
export function escapeHtml(text: string): string {
  return text.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c] ?? c);
}

/** One-sentence text alternative for the chart. */
export function describeChart({ readings, metric, episode }: ReadingsChartInput): string {
  if (readings.length === 0) {
    return `No ${metricLabel(metric).toLowerCase()} readings.`;
  }
  const values = readings.map((r) => r[metric]);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const first = readings[0].timestamp;
  const last = readings[readings.length - 1].timestamp;
  const range = `${formatMeasurement(metric, min)} to ${formatMeasurement(metric, max)}`;
  const period = `${formatSourceShort(first)} to ${formatSourceShort(last)}`;
  const flagged = episode ? ` Highlighted anomaly episode: ${formatSourceShort(episode.start)} to ${formatSourceShort(episode.end)}.` : "";
  return `${metricLabel(metric)}, ${readings.length} hourly readings from ${period}, ranging ${range}.${flagged}`;
}

interface TooltipParam {
  axisValue?: string;
  seriesName?: string;
  value?: unknown;
}

export function buildReadingsOption(input: ReadingsChartInput, theme: ChartTheme) {
  const { readings, metric, episode, events = [], baseline = [] } = input;
  const categories = readings.map((r) => r.timestamp);
  const known = new Set(categories);
  const unit = METRIC_UNITS[metric];
  const label = metricLabel(metric);

  const baselineByTime = new Map(baseline.map((p) => [p.timestamp, p.value]));
  const baselineData = baseline.length > 0 ? categories.map((t) => baselineByTime.get(t) ?? null) : [];

  const markArea =
    episode && known.has(episode.start) && known.has(episode.end)
      ? {
          silent: true,
          itemStyle: { color: theme.anomalyRegion, borderColor: theme.anomalyBorder, borderWidth: 1, borderType: "dashed" as const },
          // Named in the legend; an in-chart label collides with event labels.
          label: { show: false },
          data: [[{ name: "Anomaly episode", xAxis: episode.start }, { xAxis: episode.end }]],
        }
      : undefined;

  const markLine =
    events.filter((e) => known.has(e.timestamp)).length > 0
      ? {
          silent: true,
          symbol: ["none", "none"],
          lineStyle: { color: theme.eventMarker, type: "dashed" as const, width: 1.5 },
          label: { color: theme.eventMarker, fontWeight: 700, fontSize: 11, fontFamily: theme.font, formatter: "{b}" },
          data: events.filter((e) => known.has(e.timestamp)).map((e) => ({ name: e.label, xAxis: e.timestamp })),
        }
      : undefined;

  return {
    animation: false,
    textStyle: { fontFamily: theme.font, color: theme.text },
    grid: { left: 8, right: 16, top: 28, bottom: 8, containLabel: true },
    tooltip: {
      trigger: "axis" as const,
      backgroundColor: theme.tooltipBg,
      borderWidth: 0,
      textStyle: { color: theme.tooltipText, fontFamily: theme.font, fontSize: 12 },
      formatter: (params: TooltipParam | TooltipParam[]) => {
        const list = Array.isArray(params) ? params : [params];
        const time = typeof list[0]?.axisValue === "string" ? formatSourceShort(list[0].axisValue) : "";
        const rows = list
          .filter((p) => typeof p.value === "number")
          .map((p) => `${escapeHtml(p.seriesName ?? "")}: <b>${escapeHtml(formatMeasurement(metric, p.value as number))}</b>`);
        return [escapeHtml(time), ...rows].join("<br/>");
      },
    },
    xAxis: {
      type: "category" as const,
      data: categories,
      boundaryGap: false,
      axisLine: { lineStyle: { color: theme.grid } },
      axisTick: { show: false },
      axisLabel: {
        color: theme.axis,
        fontSize: 11,
        hideOverlap: true,
        interval: (_index: number, value: string) => parseSourceTime(value)?.hour === 0,
        formatter: (value: string) => formatSourceDay(value),
      },
    },
    yAxis: {
      type: "value" as const,
      scale: true,
      name: unit,
      nameTextStyle: { color: theme.axis, fontSize: 11, align: "left" as const },
      splitLine: { lineStyle: { color: theme.grid } },
      axisLabel: { color: theme.axis, fontSize: 11 },
    },
    series: [
      {
        name: label,
        type: "line" as const,
        data: readings.map((r) => r[metric]),
        showSymbol: false,
        lineStyle: { color: theme.series, width: 2 },
        itemStyle: { color: theme.series },
        markArea,
        markLine,
      },
      ...(baselineData.length > 0
        ? [
            {
              name: "Baseline (engine evidence)",
              type: "line" as const,
              data: baselineData,
              connectNulls: false,
              showSymbol: true,
              symbolSize: 4,
              lineStyle: { color: theme.baseline, width: 1.5, type: "dashed" as const },
              itemStyle: { color: theme.baseline },
            },
          ]
        : []),
    ],
  };
}
