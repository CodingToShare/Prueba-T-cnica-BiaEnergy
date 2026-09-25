"use client";

import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, ArrowRight } from "lucide-react";
import Link from "next/link";

import { AnomalyTypeBadge, ConfidenceBadge, SeverityBadge, StatusBadge } from "@/components/badges";
import { KpiCard, PageHeader, Panel } from "@/components/layout";
import { EmptyState, ErrorState, LoadingState, NotFoundState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/endpoints";
import type { MeterDetail } from "@/lib/api/types";
import { actionLabels } from "@/lib/format/labels";
import { EMPTY, formatCount, formatKwh, formatNumber, formatPercentSigned } from "@/lib/format/numbers";
import { formatSourcePeriod, formatSourceShort } from "@/lib/format/time";
import { queryKeys } from "@/lib/query/keys";

import { MeterReadings } from "./meter-readings";

function BackToMeters() {
  return (
    <Button asChild variant="outline" size="sm">
      <Link href="/meters">
        <ArrowLeft className="size-4" aria-hidden="true" />
        All meters
      </Link>
    </Button>
  );
}

function CurrentFinding({ meter }: { meter: MeterDetail }) {
  const finding = meter.current_finding;
  if (meter.analysis === null) {
    return (
      <EmptyState
        variant="not-analyzed"
        title="Not analyzed yet"
        message="Run the AI analysis from the dashboard to evaluate this meter."
      />
    );
  }
  if (finding === null) {
    return (
      <EmptyState
        title="No current finding"
        message="This meter showed no reportable anomaly in the latest completed analysis."
      />
    );
  }
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-1.5">
        <AnomalyTypeBadge type={finding.type} severity={finding.severity} />
        <SeverityBadge severity={finding.severity} prefix="Severity " />
        <ConfidenceBadge confidence={finding.confidence} />
        <span className="badge badge-neutral">Priority {finding.priority}</span>
      </div>
      <p className="m-0 text-[0.92rem] font-semibold text-heading">{finding.reason}</p>
      <dl className="evidence-list">
        <dt>Episode</dt>
        <dd>
          {formatSourceShort(finding.started_at)} – {formatSourceShort(finding.last_observed_at)}
        </dd>
        <dt>Variation</dt>
        <dd>{formatPercentSigned(finding.variation_pct)} vs baseline</dd>
        <dt>Recommended action</dt>
        <dd>{actionLabels[finding.recommended_action] ?? finding.recommended_action}</dd>
      </dl>
      <div>
        <Button asChild variant="secondary" size="sm">
          <Link href={`/anomalies/${finding.anomaly_id}`}>
            Open investigation
            <ArrowRight className="size-4" aria-hidden="true" />
          </Link>
        </Button>
      </div>
    </div>
  );
}

export function MeterDetailView({ meterId }: { meterId: string }) {
  const meter = useQuery({
    queryKey: queryKeys.meters.detail(meterId),
    queryFn: ({ signal }) => api.meter(meterId, signal),
  });
  const findingId = meter.data?.current_finding?.anomaly_id;
  const finding = useQuery({
    queryKey: queryKeys.anomalies.detail(findingId ?? 0),
    queryFn: ({ signal }) => api.anomaly(findingId as number, signal),
    enabled: findingId !== undefined,
  });

  if (meter.isPending) {
    return (
      <>
        <PageHeader title={meterId} description="Meter detail" actions={<BackToMeters />} />
        <LoadingState label="Loading meter…" rows={5} />
      </>
    );
  }
  if (meter.isError) {
    if (meter.error instanceof ApiError && (meter.error.isNotFound || meter.error.status === 400)) {
      return (
        <><PageHeader title="Meter detail" />
        <NotFoundState
          title="Meter not found"
          message={`There is no meter with the ID “${meterId}”.`}
          action={<BackToMeters />}
        />
        </>
      );
    }
    return <><PageHeader title={meterId} actions={<BackToMeters />} /><ErrorState title="The meter could not be loaded" error={meter.error} onRetry={() => void meter.refetch()} /></>;
  }

  const m = meter.data;
  const status = m.computed_status;
  return (
    <>
      <PageHeader
        title={m.meter_id}
        description={m.period ? `Hourly readings · ${formatSourcePeriod(m.period.first_reading_at, m.period.last_reading_at)}` : "No readings yet"}
        actions={<BackToMeters />}
      />
      <div className="flex flex-col gap-4">
        <div className="grid grid-cols-2 gap-3.5 lg:grid-cols-4">
          <KpiCard
            label="Computed status"
            value={<StatusBadge status={status} />}
            context={status === null ? "Run the AI analysis to compute it" : "From the latest completed analysis"}
          />
          <KpiCard label="Period consumption" value={formatNumber(m.total_consumption_kwh)} unit="kWh" context="Sum of hourly readings" />
          <KpiCard label="Readings" value={formatCount(m.readings_count)} context="Hourly measurements" />
          <KpiCard
            label="Variation"
            value={m.current_finding ? formatPercentSigned(m.current_finding.variation_pct) : EMPTY}
            context={m.current_finding ? "Of the current finding vs baseline" : "No reportable variation"}
          />
        </div>
        <Panel title="Current finding" labelledBy="finding-title">
          <CurrentFinding meter={m} />
          {findingId !== undefined && finding.isPending ? <LoadingState label="Loading baseline evidence…" rows={2} /> : null}
          {finding.isError ? <ErrorState title="Baseline evidence could not be loaded" error={finding.error} onRetry={() => void finding.refetch()} /> : null}
          {finding.data ? (
            <dl className="evidence-list mt-4">
              <dt>Baseline energy (flagged hours)</dt><dd>{formatKwh(finding.data.evidence.consumption.baseline_kwh)}</dd>
              <dt>Observed energy (flagged hours)</dt><dd>{formatKwh(finding.data.evidence.consumption.observed_kwh)}</dd>
            </dl>
          ) : null}
        </Panel>
        <MeterReadings meterId={m.meter_id} finding={finding.data} />
      </div>
    </>
  );
}
