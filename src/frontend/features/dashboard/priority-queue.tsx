"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";

import { SeverityBadge } from "@/components/badges";
import { Panel } from "@/components/layout";
import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { api, type AnomalyListParams } from "@/lib/api/endpoints";
import { anomalyTypeLabels, shortActionLabels } from "@/lib/format/labels";
import { formatConfidence } from "@/lib/format/numbers";
import { queryKeys } from "@/lib/query/keys";

const TOP: AnomalyListParams = { limit: 5, offset: 0 };

/** The investigation queue: current findings in the backend's priority order. */
export function PriorityQueue({ analyzed }: { analyzed: boolean }) {
  const anomalies = useQuery({
    queryKey: queryKeys.anomalies.list(TOP),
    queryFn: ({ signal }) => api.anomalies(TOP, signal),
    enabled: analyzed,
  });

  let body;
  if (!analyzed) {
    body = (
      <EmptyState
        variant="not-analyzed"
        title="No analysis yet"
        message="Run the AI analysis to detect, classify and prioritize anomalies across the fleet."
      />
    );
  } else if (anomalies.isPending) {
    body = <LoadingState label="Loading priority findings…" rows={3} />;
  } else if (anomalies.isError) {
    body = <ErrorState title="Findings could not be loaded" error={anomalies.error} onRetry={() => void anomalies.refetch()} />;
  } else if (anomalies.data.items.length === 0) {
    body = <EmptyState title="No anomalies" message="The latest analysis found no reportable anomaly." />;
  } else {
    body = (
      <ol className="queue" aria-label="Findings in priority order">
        {anomalies.data.items.map((a) => (
          <li key={a.id}>
            <Link href={`/anomalies/${a.id}`} className="queue-item">
              <span className="queue-rank" aria-label={`Priority ${a.priority}`}>
                {a.priority}
              </span>
              <span className="min-w-0">
                <span className="queue-title block">
                  {a.meter_id} · {anomalyTypeLabels[a.type] ?? a.type}
                </span>
                <span className="queue-meta">
                  <SeverityBadge severity={a.severity} /> Confidence {formatConfidence(a.confidence)}
                </span>
              </span>
              <span className="queue-action">{shortActionLabels[a.recommended_action] ?? a.recommended_action}</span>
            </Link>
          </li>
        ))}
      </ol>
    );
  }

  return (
    <Panel
      title="Investigation queue"
      labelledBy="queue-title"
      bodyClassName={analyzed && anomalies.isSuccess && anomalies.data.items.length > 0 ? "" : "p-4"}
      actions={
        <Link href="/anomalies" className="panel-link">
          All anomalies →
        </Link>
      }
    >
      {body}
    </Panel>
  );
}
