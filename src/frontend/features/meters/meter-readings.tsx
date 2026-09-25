"use client";

import { useQuery } from "@tanstack/react-query";
import dynamic from "next/dynamic";
import { useMemo, useState } from "react";

import { Panel } from "@/components/layout";
import { Segmented } from "@/components/segmented";
import { ErrorState, LoadingState } from "@/components/states";
import { chartOverlays } from "@/features/anomalies/evidence";
import { api, type ReadingsParams } from "@/lib/api/endpoints";
import type { AnomalyDetail, ReadingMetric } from "@/lib/api/types";
import { queryKeys } from "@/lib/query/keys";

// ECharts is only loaded where a chart is shown, and only in the browser.
const ReadingsChart = dynamic(() => import("./chart/readings-chart").then((m) => m.ReadingsChart), {
  ssr: false,
  loading: () => <div className="h-[260px] animate-pulse rounded-[12px] bg-surface-muted sm:h-[320px]" aria-hidden="true" />,
});

const METRICS: ReadonlyArray<{ value: ReadingMetric; label: string }> = [
  { value: "consumption_kwh", label: "Consumption" },
  { value: "voltage_v", label: "Voltage" },
  { value: "current_a", label: "Current" },
  { value: "power_factor", label: "Power factor" },
];

const ALL_READINGS: ReadingsParams = { limit: 1000 };

/**
 * Reading history of one meter with a metric selector. When the meter has a
 * finding, its episode, correlated events and the engine's baseline values
 * for the flagged hours are overlaid (all from the stored evidence).
 */
export function MeterReadings({ meterId, finding }: { meterId: string; finding?: AnomalyDetail }) {
  const [metric, setMetric] = useState<ReadingMetric>("consumption_kwh");
  const readings = useQuery({
    queryKey: queryKeys.meters.readings(meterId, ALL_READINGS),
    queryFn: ({ signal }) => api.readings(meterId, ALL_READINGS, signal),
    staleTime: 5 * 60_000,
  });
  const overlays = useMemo(() => (finding ? chartOverlays(finding, metric) : undefined), [finding, metric]);
  const label = METRICS.find((m) => m.value === metric)?.label ?? metric;

  let body;
  if (readings.isPending) {
    body = <LoadingState label="Loading readings…" rows={5} />;
  } else if (readings.isError) {
    body = <ErrorState title="Readings could not be loaded" error={readings.error} onRetry={() => void readings.refetch()} />;
  } else {
    body = (
      <>
        <ul className="m-0 mb-2 flex list-none flex-wrap gap-x-4 gap-y-1 p-0 text-[0.78rem] text-muted-foreground" aria-label="Chart legend">
          <li className="flex items-center gap-1.5">
            <i className="inline-block h-[3px] w-3.5 rounded-[3px] bg-navy" aria-hidden="true" /> Observed readings
          </li>
          {overlays ? (
            <>
              <li className="flex items-center gap-1.5">
                <i className="inline-block h-2.5 w-3.5 rounded-[3px] border border-critical bg-critical-soft" aria-hidden="true" /> Anomaly episode
              </li>
              {overlays.baseline.length > 0 ? (
                <li className="flex items-center gap-1.5">
                  <i className="inline-block h-0 w-3.5 border-t-2 border-dashed border-neutral" aria-hidden="true" /> Baseline at flagged hours
                </li>
              ) : null}
              {overlays.events.length > 0 ? (
                <li className="flex items-center gap-1.5">
                  <i className="inline-block h-2.5 w-0 border-l-2 border-dashed border-navy" aria-hidden="true" /> Correlated event
                </li>
              ) : null}
            </>
          ) : null}
        </ul>
        <ReadingsChart
          title={`${label} of ${meterId}`}
          readings={readings.data.items}
          metric={metric}
          episode={overlays?.episode}
          events={overlays?.events}
          baseline={overlays?.baseline}
        />
        <p className="m-0 mt-2 text-[0.78rem] text-muted-foreground">
          Times are the meter&apos;s local wall-clock times, as recorded in the source data.
        </p>
      </>
    );
  }

  return (
    <Panel
      title="Reading history"
      labelledBy="readings-title"
      actions={<Segmented label="Metric shown in the chart" options={METRICS} value={metric} onChange={setMetric} />}
    >
      {body}
    </Panel>
  );
}
