"use client";

import { useQuery } from "@tanstack/react-query";
import { Play } from "lucide-react";

import { Badge } from "@/components/badges";
import { KpiCard, MeterBar, PageHeader } from "@/components/layout";
import { ErrorState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { AnalysisStatus } from "@/features/analysis/analysis-status";
import { useAnalysis } from "@/features/analysis/use-analysis";
import { api } from "@/lib/api/endpoints";
import type { DashboardSummary } from "@/lib/api/types";
import { confidenceBand } from "@/lib/format/labels";
import { EMPTY, formatConfidence, formatCount, formatNumber } from "@/lib/format/numbers";
import { formatSourcePeriod, formatSystemDate, formatSystemTime } from "@/lib/format/time";
import { queryKeys } from "@/lib/query/keys";

import { FleetHealth } from "./fleet-health";
import { PriorityQueue } from "./priority-queue";

const NOT_ANALYZED_YET = "Not analyzed yet";

function Kpis({ summary }: { summary: DashboardSummary }) {
  const latest = summary.latest_analysis;
  const analyzed = latest !== null;
  const confidence = summary.aggregate_confidence;
  return (
    <div className="grid grid-cols-2 gap-3.5 min-[721px]:grid-cols-3">
      <KpiCard label="Meters" value={formatCount(summary.meters)} context={`${formatCount(summary.readings)} hourly readings`} />
      <KpiCard
        label="Total consumption"
        value={formatNumber(summary.total_consumption_kwh)}
        unit="kWh"
        context="Sum of all readings in the period"
      />
      <KpiCard
        label="AI anomalies"
        value={analyzed ? formatCount(summary.anomalies) : EMPTY}
        context={analyzed ? "Findings of the latest analysis" : NOT_ANALYZED_YET}
      />
      <KpiCard
        featured
        label="High priority"
        value={analyzed ? formatCount(summary.high_priority) : EMPTY}
        context={analyzed ? "Findings with high severity" : NOT_ANALYZED_YET}
      />
      <KpiCard
        label="AI confidence"
        value={confidence !== null ? formatConfidence(confidence) : EMPTY}
        context={
          confidence !== null
            ? `${confidenceBand(confidence)} · mean of current findings`
            : analyzed
              ? "No findings to score"
              : NOT_ANALYZED_YET
        }
      >
        {confidence !== null ? <MeterBar value={confidence} label={`Confidence ${formatConfidence(confidence)}`} /> : null}
      </KpiCard>
      <KpiCard
        label="Latest analysis"
        value={latest ? formatSystemTime(latest.completed_at) : EMPTY}
        context={
          latest ? (
            <span className="mt-1 flex flex-wrap items-center gap-1.5">
              {formatSystemDate(latest.completed_at)} <Badge tone="normal">Completed</Badge>
            </span>
          ) : (
            NOT_ANALYZED_YET
          )
        }
      />
    </div>
  );
}

function KpiSkeleton() {
  return (
    <div className="grid grid-cols-2 gap-3.5 min-[721px]:grid-cols-3" aria-busy="true" role="status">
      <span className="sr-only">Loading summary…</span>
      {Array.from({ length: 6 }, (_, i) => (
        <div key={i} className="kpi">
          <Skeleton className="h-3 w-1/2" />
          <Skeleton className="mt-3 h-7 w-2/3" />
          <Skeleton className="mt-3 h-2.5 w-3/4" />
        </div>
      ))}
    </div>
  );
}

export function DashboardView() {
  const dashboard = useQuery({
    queryKey: queryKeys.dashboard,
    queryFn: ({ signal }) => api.dashboard(signal),
  });
  const analysis = useAnalysis(dashboard.data);
  const summary = dashboard.data;
  const analyzed = summary?.latest_analysis != null;

  const description = summary
    ? [summary.period ? formatSourcePeriod(summary.period.first_reading_at, summary.period.last_reading_at) : null, `${formatCount(summary.meters)} meters`, "hourly readings"]
        .filter(Boolean)
        .join(" · ")
    : "Fleet overview";

  return (
    <>
      <PageHeader
        title="Dashboard"
        description={description}
        actions={
          <Button onClick={analysis.start} disabled={analysis.busy || !dashboard.isSuccess} aria-busy={analysis.busy}>
            {analysis.busy ? (
              <>
                <span className="size-4 animate-spin rounded-full border-2 border-white/40 border-t-white" aria-hidden="true" />
                Analyzing…
              </>
            ) : (
              <>
                <Play className="size-4" aria-hidden="true" />
                {analyzed ? "Run analysis again" : "Run AI Analysis"}
              </>
            )}
          </Button>
        }
      />

      <div className="flex flex-col gap-4">
        {analysis.runError ? (
          <ErrorState title="Analysis status could not be loaded" error={analysis.runError}
            action={<Button variant="secondary" size="sm" onClick={analysis.retryStatus} disabled={analysis.retryingStatus}>Retry status</Button>} />
        ) : (
          <AnalysisStatus run={analysis.run} startError={analysis.startError} onRetry={analysis.start} retrying={analysis.busy} />
        )}

        {dashboard.isPending ? <KpiSkeleton /> : null}
        {dashboard.isError ? (
          <ErrorState title="The dashboard could not be loaded" error={dashboard.error} onRetry={() => void dashboard.refetch()} />
        ) : null}
        {summary ? <Kpis summary={summary} /> : null}

        {summary ? (
          <div className="grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)]">
            <PriorityQueue analyzed={analyzed} />
            <FleetHealth />
          </div>
        ) : null}
      </div>
    </>
  );
}
