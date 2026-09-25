"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";

import { StatusBadge } from "@/components/badges";
import { Panel } from "@/components/layout";
import { ErrorState, LoadingState } from "@/components/states";
import { api, type MeterListParams } from "@/lib/api/endpoints";
import type { ComputedStatus } from "@/lib/api/types";
import { formatCount } from "@/lib/format/numbers";
import { queryKeys } from "@/lib/query/keys";

/** Every meter of the fleet (the API's maximum page covers this dataset). */
export const FLEET: MeterListParams = { sort: "meter_id", order: "asc", limit: 100, offset: 0 };

const ORDER: ReadonlyArray<ComputedStatus | null> = ["CRITICAL", "ALERT", "OK", null];

/**
 * Answers "how healthy is the fleet?" by counting the computed status the
 * backend reports for each meter (not analyzed until an analysis completes).
 */
export function FleetHealth() {
  const meters = useQuery({
    queryKey: queryKeys.meters.list(FLEET),
    queryFn: ({ signal }) => api.meters(FLEET, signal),
  });

  let body;
  if (meters.isPending) {
    body = <LoadingState label="Loading fleet status…" rows={3} />;
  } else if (meters.isError) {
    body = <ErrorState title="Fleet status could not be loaded" error={meters.error} onRetry={() => void meters.refetch()} />;
  } else {
    const items = meters.data.items;
    const total = items.length;
    const counts = new Map<ComputedStatus | null, number>();
    for (const m of items) {
      counts.set(m.computed_status, (counts.get(m.computed_status) ?? 0) + 1);
    }
    const rows = ORDER.filter((status) => (counts.get(status) ?? 0) > 0);
    body = (
      <>
        <p className="m-0 mb-3 text-[0.82rem] text-muted-foreground">
          {meters.data.analysis
            ? `Computed status of ${formatCount(total)} meters from the latest completed analysis.`
            : `${formatCount(total)} meters, not analyzed yet: run the AI analysis to compute their status.`}
        </p>
        <ul className="m-0 flex list-none flex-col gap-3 p-0" aria-label="Meters by computed status">
          {rows.map((status) => {
            const count = counts.get(status) ?? 0;
            const share = total === 0 ? 0 : count / total;
            return (
              <li key={status ?? "none"} className="grid grid-cols-[7.5rem_minmax(0,1fr)_2rem] items-center gap-3">
                <StatusBadge status={status} />
                <div className="meter-bar m-0" aria-hidden="true">
                  <span style={{ width: `${Math.round(share * 100)}%` }} />
                </div>
                <span className="num text-right text-[0.9rem] font-bold text-heading">{count}</span>
              </li>
            );
          })}
        </ul>
      </>
    );
  }

  return (
    <Panel
      title="Fleet health"
      labelledBy="fleet-title"
      actions={
        <Link href="/meters" className="panel-link">
          All meters →
        </Link>
      }
    >
      {body}
    </Panel>
  );
}
