"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";

import { AnomalyTypeBadge, SeverityBadge } from "@/components/badges";
import { PageHeader } from "@/components/layout";
import { Segmented } from "@/components/segmented";
import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { api, type AnomalyListParams } from "@/lib/api/endpoints";
import type { AnomalyType, Severity } from "@/lib/api/types";
import { actionLabels, anomalyTypeLabels } from "@/lib/format/labels";
import { formatConfidence } from "@/lib/format/numbers";
import { formatSourceShort } from "@/lib/format/time";
import { queryKeys } from "@/lib/query/keys";
import { useDebouncedValue } from "@/lib/use-debounced-value";

const PAGE_SIZE = 20;

type SeverityFilter = "ALL" | Severity;
const SEVERITY_OPTIONS: ReadonlyArray<{ value: SeverityFilter; label: string }> = [
  { value: "ALL", label: "All" },
  { value: "HIGH", label: "High" },
  { value: "MEDIUM", label: "Medium" },
  { value: "LOW", label: "Low" },
];

const TYPE_OPTIONS = Object.entries(anomalyTypeLabels) as Array<[AnomalyType, string]>;

// Mirrors the API's meter-ID rule so an incomplete ID is simply not applied
// yet; the backend still validates every request.
const METER_ID = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/;

/** AI Anomalies: the latest completed analysis, in the backend's priority order. */
export function AnomaliesView() {
  const [severity, setSeverity] = useState<SeverityFilter>("ALL");
  const [type, setType] = useState<"ALL" | AnomalyType>("ALL");
  const [meterInput, setMeterInput] = useState("");
  const [offset, setOffset] = useState(0);
  const meterId = useDebouncedValue(meterInput.trim(), 300);
  const meterFilter = METER_ID.test(meterId) ? meterId : undefined;
  const meterInvalid = meterId !== "" && meterFilter === undefined;

  const params: AnomalyListParams = {
    meterId: meterFilter,
    type: type === "ALL" ? undefined : type,
    severity: severity === "ALL" ? undefined : severity,
    limit: PAGE_SIZE,
    offset,
  };
  const anomalies = useQuery({
    queryKey: queryKeys.anomalies.list(params),
    queryFn: ({ signal }) => api.anomalies(params, signal),
    placeholderData: keepPreviousData,
  });
  const filtered = severity !== "ALL" || type !== "ALL" || meterFilter !== undefined;

  function clearFilters() {
    setSeverity("ALL");
    setType("ALL");
    setMeterInput("");
    setOffset(0);
  }

  const data = anomalies.data;
  let content;
  if (anomalies.isPending) {
    content = <LoadingState label="Loading anomalies…" rows={4} />;
  } else if (anomalies.isError) {
    content = <ErrorState title="The anomalies could not be loaded" error={anomalies.error} onRetry={() => void anomalies.refetch()} />;
  } else if (data && data.analysis === null) {
    content = (
      <EmptyState
        variant="not-analyzed"
        title="No analysis yet"
        message="Run the AI analysis from the dashboard to detect and prioritize anomalies."
        action={
          <Button asChild size="sm">
            <Link href="/dashboard">Go to dashboard</Link>
          </Button>
        }
      />
    );
  } else if (data && data.items.length === 0) {
    content = (
      <EmptyState
        title={filtered ? "No anomalies match" : "No anomalies"}
        message={filtered ? "No finding of the latest analysis matches these filters." : "The latest analysis found no reportable anomaly."}
        action={
          filtered ? (
            <Button variant="outline" size="sm" onClick={clearFilters}>
              Clear filters
            </Button>
          ) : undefined
        }
      />
    );
  } else if (data) {
    const last = data.pagination.offset + data.items.length;
    content = (
      <section className="panel" aria-labelledby="anomalies-caption">
        <div className="tbl-wrap">
          <table className="tbl tbl-stack">
            <caption id="anomalies-caption">
              Findings of the latest completed analysis, most important first. False positives stay visible: they show
              deviations that operational context explains.
            </caption>
            <thead>
              <tr>
                <th scope="col" aria-sort="ascending">
                  Priority
                </th>
                <th scope="col">Meter</th>
                <th scope="col">Type</th>
                <th scope="col">Severity</th>
                <th scope="col" className="r">
                  Confidence
                </th>
                <th scope="col">Recommended action</th>
                <th scope="col">Episode start</th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((a) => (
                <tr key={a.id}>
                  <td data-label="Priority" className="num font-extrabold text-heading">
                    #{a.priority}
                  </td>
                  <td data-label="Meter">
                    <Link href={`/anomalies/${a.id}`} aria-label={`Investigate ${a.meter_id}, ${anomalyTypeLabels[a.type] ?? a.type}`}>
                      {a.meter_id}
                    </Link>
                  </td>
                  <td data-label="Type">
                    <AnomalyTypeBadge type={a.type} severity={a.severity} />
                  </td>
                  <td data-label="Severity">
                    <SeverityBadge severity={a.severity} />
                  </td>
                  <td data-label="Confidence" className="r">
                    {formatConfidence(a.confidence)}
                  </td>
                  <td data-label="Recommended action">{actionLabels[a.recommended_action] ?? a.recommended_action}</td>
                  <td data-label="Episode start" className="whitespace-nowrap">
                    {formatSourceShort(a.started_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <nav className="flex flex-wrap items-center justify-between gap-3 border-t border-border px-4 py-3" aria-label="Anomalies pagination">
          <span className="text-[0.82rem] text-muted-foreground" aria-live="polite">
            Showing {data.pagination.offset + 1}–{last} of {data.pagination.total}
          </span>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
              Previous
            </Button>
            <Button variant="outline" size="sm" disabled={last >= data.pagination.total} onClick={() => setOffset(offset + PAGE_SIZE)}>
              Next
            </Button>
          </div>
        </nav>
      </section>
    );
  }

  return (
    <>
      <PageHeader title="AI Anomalies" description="Classified findings with severity, confidence and a recommended action." />
      <div className="mb-4 flex flex-wrap items-end gap-3">
        <div className="flex flex-col gap-1.5">
          <span className="text-[0.76rem] font-bold text-heading">Severity</span>
          <Segmented
            label="Filter by severity"
            options={SEVERITY_OPTIONS}
            value={severity}
            onChange={(v) => {
              setSeverity(v);
              setOffset(0);
            }}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="type-filter">Type</Label>
          <select
            id="type-filter"
            value={type}
            onChange={(e) => {
              setType(e.target.value as "ALL" | AnomalyType);
              setOffset(0);
            }}
            className="h-[2.6rem] cursor-pointer rounded-[12px] border border-border-strong bg-surface px-3 text-[0.9rem] outline-none focus-visible:border-primary focus-visible:shadow-[0_0_0_0.25rem_var(--focus-ring)]"
          >
            <option value="ALL">All types</option>
            {TYPE_OPTIONS.map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </div>
        <div className="flex min-w-[180px] flex-col gap-1.5">
          <Label htmlFor="meter-filter">Meter ID</Label>
          <input
            id="meter-filter"
            type="search"
            value={meterInput}
            onChange={(e) => {
              setMeterInput(e.target.value);
              setOffset(0);
            }}
            placeholder="Meter ID"
            maxLength={64}
            autoComplete="off"
            aria-invalid={meterInvalid || undefined}
            aria-describedby={meterInvalid ? "meter-filter-hint" : undefined}
            className="h-[2.6rem] rounded-[12px] border border-border-strong bg-surface px-3 text-[0.9rem] outline-none placeholder:text-muted-foreground focus-visible:border-primary focus-visible:shadow-[0_0_0_0.25rem_var(--focus-ring)] aria-invalid:border-destructive"
          />
          {meterInvalid ? (
            <span id="meter-filter-hint" className="text-[0.78rem] font-semibold text-destructive-text">
              Use letters, digits, &ldquo;.&rdquo;, &ldquo;_&rdquo; or &ldquo;-&rdquo;.
            </span>
          ) : null}
        </div>
      </div>
      {content}
    </>
  );
}
