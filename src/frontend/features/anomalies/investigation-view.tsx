"use client";

import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, ClipboardCheck, SearchCheck, ShieldCheck, Wrench, type LucideIcon } from "lucide-react";
import Link from "next/link";

import { AnomalyTypeBadge, Badge, ConfidenceBadge, SeverityBadge } from "@/components/badges";
import { MeterBar, PageHeader, Panel } from "@/components/layout";
import { ErrorState, LoadingState, NotFoundState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { MeterReadings } from "@/features/meters/meter-readings";
import { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/endpoints";
import type { AnomalyDetail, RecommendedAction } from "@/lib/api/types";
import {
  actionDescriptions,
  actionLabels,
  anomalyTypeLabels,
  eventRoleLabels,
  eventRoleTone,
  typeMeaning,
} from "@/lib/format/labels";
import { formatNumber, formatPercentSigned } from "@/lib/format/numbers";
import { formatDurationHours, formatSourceDateTime, formatSourceShort } from "@/lib/format/time";
import { queryKeys } from "@/lib/query/keys";

import { CONFIDENCE_COMPONENTS, changedVariables, formatOffset, hasExplanation, keyFacts, supportingVariables } from "./evidence";

const ACTION_ICONS: Record<RecommendedAction, LucideIcon> = {
  INVESTIGATE_METER_AND_INSTALLATION: Wrench,
  VALIDATE_MEASUREMENT_OR_SENSOR: SearchCheck,
  VALIDATE_OPERATIONAL_CHANGE: ClipboardCheck,
  NO_ESCALATION_MONITOR: ShieldCheck,
};

function BackToAnomalies() {
  return (
    <Button asChild variant="outline" size="sm">
      <Link href="/anomalies">
        <ArrowLeft className="size-4" aria-hidden="true" />
        All anomalies
      </Link>
    </Button>
  );
}

/** Type-specific headline built only from the stored evidence. */
function ContextCallout({ detail }: { detail: AnomalyDetail }) {
  const { evidence } = detail;
  const events = evidence.related_events;
  const explaining = events.find((e) => e.role === "EXPLAINS");
  const recovery = evidence.persistence.recovery;

  switch (detail.type) {
    case "REAL_ANOMALY":
      return (
        <div className={`alert ${detail.severity === "HIGH" ? "alert-critical" : "alert-warning"}`} role="note">
          <span>
            <strong>Unexplained deviation.</strong>{" "}
            {hasExplanation(events)
              ? "A correlated event exists but does not account for this change."
              : events.length > 0
                ? "The events recorded near the onset are context only; none explains this change."
                : "No operational event was recorded near the onset."}
          </span>
        </div>
      );
    case "EXPLAINABLE_ANOMALY":
      return (
        <div className="alert alert-informational" role="note">
          <span>
            <strong>Explained change.</strong>{" "}
            {explaining
              ? `${explaining.type} recorded ${formatOffset(explaining.offset_seconds)} (${formatSourceShort(explaining.timestamp)}). The deviation is real; validate the new operating level.`
              : "A correlated operational event explains the deviation."}
          </span>
        </div>
      );
    case "FALSE_POSITIVE":
      return (
        <div className="alert alert-informational" role="note">
          <span>
            <strong>Detected, explained and recovered.</strong>{" "}
            {explaining ? `${explaining.type} recorded ${formatOffset(explaining.offset_seconds)}. ` : ""}
            {recovery.recovered && recovery.recovered_at
              ? `Readings recovered from ${formatSourceShort(recovery.recovered_at)}; no escalation is needed.`
              : "No escalation is needed."}
          </span>
        </div>
      );
    case "DATA_QUALITY": {
      const consumption = evidence.metrics.find((m) => m.metric === "consumption_kwh");
      const inconsistent = supportingVariables(evidence).map((v) => v.label.toLowerCase());
      return (
        <div className="alert alert-data-quality" role="note">
          <span>
            <strong>Measurement inconsistency.</strong>{" "}
            {inconsistent.length > 0 ? `Inconsistent ${inconsistent.join(", ")}` : "Inconsistent electrical readings"}
            {consumption ? ` while consumption stayed within ±${formatNumber(consumption.max_abs_deviation_pct, 1)}% of its baseline.` : "."}
          </span>
        </div>
      );
    }
  }
}

function ActionCard({ detail }: { detail: AnomalyDetail }) {
  const Icon = ACTION_ICONS[detail.recommended_action] ?? ClipboardCheck;
  return (
    <section className="panel" aria-labelledby="action-title">
      <div className="panel-body flex flex-col gap-3">
        <div className="flex items-center gap-3">
          <span className="grid size-11 shrink-0 place-items-center rounded-[13px] bg-primary-soft text-primary" aria-hidden="true">
            <Icon className="size-5" />
          </span>
          <div>
            <div className="text-[0.72rem] font-bold uppercase tracking-[0.06em] text-muted-foreground">What to do</div>
            <h2 id="action-title" className="m-0 text-[1.05rem] font-extrabold text-heading">
              {actionLabels[detail.recommended_action] ?? detail.recommended_action}
            </h2>
          </div>
        </div>
        <p className="m-0 text-[0.88rem] text-foreground">{actionDescriptions[detail.recommended_action] ?? "Review the stored evidence and the recommended action above."}</p>
        <div>
          <Button asChild variant="secondary" size="sm" className="h-auto min-h-[2.3rem] max-w-full whitespace-normal text-left">
            <Link href={`/meters/${encodeURIComponent(detail.meter_id)}`}>View meter {detail.meter_id}</Link>
          </Button>
        </div>
      </div>
    </section>
  );
}

function RelatedEvents({ detail }: { detail: AnomalyDetail }) {
  const events = detail.evidence.related_events;
  if (events.length === 0) {
    return <p className="m-0 text-[0.88rem] text-muted-foreground">No operational or data event was recorded near the onset of this episode.</p>;
  }
  return (
    <ul className="m-0 flex list-none flex-col gap-3 p-0">
      {events.map((e) => (
        <li key={`${e.timestamp}-${e.type}-${e.description}`} className="rounded-[14px] border border-border p-3.5">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-[0.9rem] font-extrabold text-heading">{e.type}</span>
            <Badge tone={eventRoleTone(e.role)}>{eventRoleLabels[e.role] ?? e.role}</Badge>
          </div>
          <p className="m-0 mt-1.5 text-[0.88rem] text-foreground">{e.description}</p>
          <p className="m-0 mt-1 text-[0.78rem] text-muted-foreground">
            {formatSourceDateTime(e.timestamp)} · {formatOffset(e.offset_seconds)}
          </p>
        </li>
      ))}
    </ul>
  );
}

function ChangedVariables({ detail }: { detail: AnomalyDetail }) {
  const rows = changedVariables(detail.evidence);
  return (
    <div className="tbl-wrap">
      <table className="tbl tbl-stack">
        <caption>Median deviation over the flagged readings; &ldquo;supports&rdquo; marks variables that changed consistently.</caption>
        <thead>
          <tr>
            <th scope="col">Variable</th>
            <th scope="col" className="r">
              Change
            </th>
            <th scope="col">Direction</th>
            <th scope="col" className="r">
              Flagged readings
            </th>
            <th scope="col">Supports finding</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.metric}>
              <td data-label="Variable" className="font-bold text-heading">
                {row.label}
              </td>
              <td data-label="Change" className="r">
                {row.change}
              </td>
              <td data-label="Direction">{row.direction}</td>
              <td data-label="Flagged readings" className="r">
                {row.flagged}
              </td>
              <td data-label="Supports finding">{row.supports ? <Badge tone="informational">Supports</Badge> : "—"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function TechnicalEvidence({ detail }: { detail: AnomalyDetail }) {
  const { evidence } = detail;
  return (
    <details className="panel">
      <summary className="cursor-pointer px-5 py-4 font-bold text-primary">Technical evidence</summary>
      <div className="flex flex-col gap-3 border-t border-border px-5 py-4 text-[0.84rem]">
        <dl className="evidence-list">
          <dt>Decision rule</dt>
          <dd>{detail.rule}</dd>
          <dt>Evidence strength</dt>
          <dd>{formatNumber(evidence.evidence_strength)}× the detection threshold</dd>
          <dt>Evidence schema</dt>
          <dd>v{evidence.schema_version}</dd>
          <dt>Signals</dt>
          <dd>{evidence.signals.length} metric readings passed both detection gates</dd>
        </dl>
        <div className="max-h-80 overflow-auto rounded-[10px] border border-border">
          <table className="tbl">
            <thead>
              <tr>
                <th scope="col">Time (source)</th>
                <th scope="col">Metric</th>
                <th scope="col" className="r">
                  Observed
                </th>
                <th scope="col" className="r">
                  Baseline
                </th>
                <th scope="col" className="r">
                  Deviation
                </th>
                <th scope="col" className="r">
                  Robust Z
                </th>
              </tr>
            </thead>
            <tbody>
              {evidence.signals.map((s) => (
                <tr key={`${s.timestamp}-${s.metric}`}>
                  <td>{formatSourceShort(s.timestamp)}</td>
                  <td>{s.metric}</td>
                  <td className="r">{formatNumber(s.observed, 3)}</td>
                  <td className="r">{formatNumber(s.baseline, 3)}</td>
                  <td className="r">{formatPercentSigned(s.deviation_pct)}</td>
                  <td className="r">{formatNumber(s.robust_z, 1)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </details>
  );
}

export function InvestigationView({ id }: { id: number }) {
  const anomaly = useQuery({
    queryKey: queryKeys.anomalies.detail(id),
    queryFn: ({ signal }) => api.anomaly(id, signal),
    enabled: Number.isSafeInteger(id) && id > 0,
  });

  if (!Number.isSafeInteger(id) || id <= 0 || (anomaly.error instanceof ApiError && (anomaly.error.isNotFound || anomaly.error.status === 400))) {
    return <><PageHeader title="Investigation" /><NotFoundState title="Anomaly not found" message="This finding does not exist or is no longer available." action={<BackToAnomalies />} /></>;
  }
  if (anomaly.isPending) {
    return (
      <>
        <PageHeader title="Investigation" actions={<BackToAnomalies />} />
        <LoadingState label="Loading investigation…" rows={6} />
      </>
    );
  }
  if (anomaly.isError) {
    return <><PageHeader title="Investigation" actions={<BackToAnomalies />} /><ErrorState title="The investigation could not be loaded" error={anomaly.error} onRetry={() => void anomaly.refetch()} /></>;
  }

  const d = anomaly.data;
  const facts = keyFacts(d.evidence);
  const supporting = supportingVariables(d.evidence);

  return (
    <>
      <PageHeader
        title={`${d.meter_id} · ${anomalyTypeLabels[d.type] ?? d.type}`}
        description={`Priority ${d.priority} · Episode ${formatSourceDateTime(d.started_at)} – ${formatSourceDateTime(d.last_observed_at)} (${formatDurationHours(d.duration_hours)})`}
        actions={<BackToAnomalies />}
      />

      <div className="flex flex-col gap-4">
        <div className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)] lg:items-start">
          <section className="panel" aria-labelledby="summary-title">
            <div className="panel-head">
              <div className="flex flex-col gap-2">
                <h2 id="summary-title" className="panel-title">
                  Finding summary
                </h2>
                <div className="flex flex-wrap gap-1.5">
                  <AnomalyTypeBadge type={d.type} severity={d.severity} />
                  <SeverityBadge severity={d.severity} prefix="Severity " />
                  <ConfidenceBadge confidence={d.confidence} />
                  <Badge tone="neutral">Priority {d.priority}</Badge>
                </div>
              </div>
            </div>
            <div className="panel-body flex flex-col gap-3">
              <ContextCallout detail={d} />
              <div className="story">
                <div className="story-block">
                  <h3>What happened</h3>
                  <p>{d.reason}</p>
                </div>
                <div className="story-block">
                  <h3>Why it matters</h3>
                  <p>{typeMeaning[d.type] ?? "Review the classification and evidence returned by the analysis."}</p>
                </div>
                <div className="story-block">
                  <h3>What supports it</h3>
                  <p>
                    {formatPercentSigned(d.evidence.consumption.median_deviation_pct)} median consumption deviation over{" "}
                    {formatDurationHours(d.evidence.persistence.duration_hours)}
                    {supporting.length > 0 ? `; ${supporting.map((v) => v.label.toLowerCase()).join(", ")} also changed` : ""}.
                  </p>
                </div>
              </div>
            </div>
          </section>
          <ActionCard detail={d} />
        </div>

        <div className="grid gap-4 lg:grid-cols-2">
          <Panel title="Consumption evidence" labelledBy="facts-title">
            <dl className="evidence-list">
              {facts.map((f) => (
                <div key={f.label} className="contents">
                  <dt>{f.label}</dt>
                  <dd>{f.value}</dd>
                </div>
              ))}
            </dl>
          </Panel>
          <Panel title="Operational context" labelledBy="events-title">
            <RelatedEvents detail={d} />
          </Panel>
        </div>

        <Panel title="Changed variables" labelledBy="variables-title" bodyClassName="">
          <ChangedVariables detail={d} />
        </Panel>

        <div className="grid gap-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
          <MeterReadings meterId={d.meter_id} finding={d} />
          <Panel title="Confidence composition" labelledBy="confidence-title">
            <p className="m-0 mb-3 text-[0.82rem] text-muted-foreground">
              Confidence ({Math.round(d.confidence * 100)}%) reflects how strongly the evidence supports this classification. It is not a
              probability of equipment failure.
            </p>
            {CONFIDENCE_COMPONENTS.map(({ key, label }) => {
              const value = d.evidence.confidence_breakdown[key];
              return (
                <div key={key} className="conf-row">
                  <span>{label}</span>
                  <MeterBar value={value} label={`${label} ${Math.round(value * 100)}%`} />
                  <span>{Math.round(value * 100)}%</span>
                </div>
              );
            })}
          </Panel>
        </div>

        <TechnicalEvidence detail={d} />
      </div>
    </>
  );
}
