"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Search } from "lucide-react";
import Link from "next/link";
import { useState } from "react";

import { AnomalyTypeBadge, SeverityBadge, StatusBadge } from "@/components/badges";
import { PageHeader } from "@/components/layout";
import { Segmented } from "@/components/segmented";
import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { api, type MeterListParams, type MeterSort, type SortOrder } from "@/lib/api/endpoints";
import type { ComputedStatus, MeterSummary } from "@/lib/api/types";
import { EMPTY, formatConfidence, formatKwh, formatPercentSigned } from "@/lib/format/numbers";
import { queryKeys } from "@/lib/query/keys";
import { useDebouncedValue } from "@/lib/use-debounced-value";

export const PAGE_SIZE = 10;

type StatusFilter = "ALL" | ComputedStatus;

const STATUS_OPTIONS: ReadonlyArray<{ value: StatusFilter; label: string }> = [
  { value: "ALL", label: "All" },
  { value: "OK", label: "OK" },
  { value: "ALERT", label: "Alert" },
  { value: "CRITICAL", label: "Critical" },
];

/** First click on a column sorts A→Z for the meter ID and largest-first otherwise. */
const DEFAULT_ORDER: Record<MeterSort, SortOrder> = {
  meter_id: "asc",
  consumption: "desc",
  variation: "desc",
  severity: "desc",
};

function SortHeader({
  label,
  column,
  sort,
  order,
  onSort,
  numeric = false,
}: {
  label: string;
  column: MeterSort;
  sort: MeterSort;
  order: SortOrder;
  onSort: (column: MeterSort) => void;
  numeric?: boolean;
}) {
  const active = sort === column;
  return (
    <th scope="col" className={numeric ? "r" : undefined} aria-sort={active ? (order === "asc" ? "ascending" : "descending") : "none"}>
      <span className="min-[641px]:hidden">{label}</span>
      <button type="button" className="hidden min-[641px]:inline-flex" onClick={() => onSort(column)}>
        {label}
        <span aria-hidden="true" className={active ? "" : "opacity-30"}>
          {active ? (order === "asc" ? "▲" : "▼") : "↕"}
        </span>
      </button>
    </th>
  );
}

function MeterRow({ meter }: { meter: MeterSummary }) {
  return (
    <tr>
      <td data-label="Meter">
        <Link href={`/meters/${encodeURIComponent(meter.meter_id)}`}>{meter.meter_id}</Link>
      </td>
      <td data-label="Consumption" className="r">
        {formatKwh(meter.total_consumption_kwh)}
      </td>
      <td data-label="Variation" className="r">
        {formatPercentSigned(meter.variation_pct)}
      </td>
      <td data-label="Status">
        <StatusBadge status={meter.computed_status} />
      </td>
      <td data-label="Anomaly">
        {meter.anomaly_type && meter.severity ? <AnomalyTypeBadge type={meter.anomaly_type} severity={meter.severity} /> : EMPTY}
      </td>
      <td data-label="Severity">{meter.severity ? <SeverityBadge severity={meter.severity} /> : EMPTY}</td>
      <td data-label="Confidence" className="r">
        {formatConfidence(meter.confidence)}
      </td>
    </tr>
  );
}

export function MetersView() {
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState<StatusFilter>("ALL");
  const [sort, setSort] = useState<MeterSort>("meter_id");
  const [order, setOrder] = useState<SortOrder>("asc");
  const [offset, setOffset] = useState(0);
  const debouncedSearch = useDebouncedValue(search.trim(), 250);

  const params: MeterListParams = {
    search: debouncedSearch || undefined,
    status: status === "ALL" ? undefined : status,
    sort,
    order,
    limit: PAGE_SIZE,
    offset,
  };
  const meters = useQuery({
    queryKey: queryKeys.meters.list(params),
    queryFn: ({ signal }) => api.meters(params, signal),
    placeholderData: keepPreviousData,
  });

  function onSort(column: MeterSort) {
    if (column === sort) {
      setOrder(order === "asc" ? "desc" : "asc");
    } else {
      setSort(column);
      setOrder(DEFAULT_ORDER[column]);
    }
    setOffset(0);
  }

  function clearFilters() {
    setSearch("");
    setStatus("ALL");
    setOffset(0);
  }

  const data = meters.data;
  const analyzed = data?.analysis != null;
  const total = data?.pagination.total ?? 0;
  const filtered = debouncedSearch !== "" || status !== "ALL";

  let content;
  if (meters.isPending) {
    content = <LoadingState label="Loading meters…" rows={6} />;
  } else if (meters.isError) {
    content = <ErrorState title="The meters could not be loaded" error={meters.error} onRetry={() => void meters.refetch()} />;
  } else if (data && data.items.length === 0) {
    content = (
      <EmptyState
        title={filtered ? "No meters match" : "No meters"}
        message={
          filtered
            ? status !== "ALL" && !analyzed
              ? "Status filters apply after the first AI analysis; no meter has a computed status yet."
              : "No meter matches the current search and filter."
            : "There are no meters in the dataset."
        }
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
    const first = data.pagination.offset + 1;
    const last = data.pagination.offset + data.items.length;
    content = (
      <section className="panel" aria-labelledby="meters-table-caption">
        <div className="tbl-wrap">
          <table className="tbl tbl-stack">
            <caption id="meters-table-caption">
              {analyzed
                ? "Status and finding from the latest completed analysis. Variation is the consumption deviation of the meter's finding and sorts by magnitude."
                : "Not analyzed yet: run the AI analysis to compute each meter's status."}
            </caption>
            <thead>
              <tr>
                <SortHeader label="Meter" column="meter_id" sort={sort} order={order} onSort={onSort} />
                <SortHeader label="Consumption" column="consumption" sort={sort} order={order} onSort={onSort} numeric />
                <SortHeader label="Variation" column="variation" sort={sort} order={order} onSort={onSort} numeric />
                <th scope="col">Status</th>
                <th scope="col">Anomaly</th>
                <SortHeader label="Severity" column="severity" sort={sort} order={order} onSort={onSort} />
                <th scope="col" className="r">
                  Confidence
                </th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((meter) => (
                <MeterRow key={meter.meter_id} meter={meter} />
              ))}
            </tbody>
          </table>
        </div>
        <nav className="flex flex-wrap items-center justify-between gap-3 border-t border-border px-4 py-3" aria-label="Meters pagination">
          <span className="text-[0.82rem] text-muted-foreground" aria-live="polite">
            Showing {first}–{last} of {total}
            {meters.isFetching && meters.isPlaceholderData ? " · Updating…" : ""}
          </span>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
              Previous
            </Button>
            <Button variant="outline" size="sm" disabled={last >= total} onClick={() => setOffset(offset + PAGE_SIZE)}>
              Next
            </Button>
          </div>
        </nav>
      </section>
    );
  }

  return (
    <>
      <PageHeader title="Meters" description="Fleet consumption and the analytical status of every meter." />
      <div className="mb-4 flex flex-wrap items-end gap-3">
        <div className="flex min-w-[220px] max-w-[320px] flex-1 flex-col gap-1.5">
          <Label htmlFor="meter-search">Search by meter ID</Label>
          <div className="relative">
            <Search className="pointer-events-none absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
            <input
              id="meter-search"
              type="search"
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                setOffset(0);
              }}
              placeholder="Meter ID"
              maxLength={64}
              autoComplete="off"
              className="h-[2.6rem] w-full rounded-full border border-border-strong bg-surface pl-10 pr-4 text-[0.9rem] outline-none transition-[border-color,box-shadow] placeholder:text-muted-foreground focus-visible:border-primary focus-visible:shadow-[0_0_0_0.25rem_var(--focus-ring)] focus-visible:outline-none"
            />
          </div>
        </div>
        <div className="flex flex-col gap-1.5">
          <span className="text-[0.76rem] font-bold text-heading" id="status-filter-label">
            Computed status
          </span>
          <Segmented
            label="Filter by computed status"
            options={STATUS_OPTIONS}
            value={status}
            onChange={(value) => {
              setStatus(value);
              setOffset(0);
            }}
          />
        </div>
      </div>
      <div className="mb-4 flex flex-wrap items-end gap-3 min-[641px]:hidden">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="meter-sort">Sort by</Label>
          <select id="meter-sort" value={sort} className="h-11 rounded-[12px] border border-border-strong bg-surface px-3"
            onChange={(e) => { const value = e.target.value as MeterSort; setSort(value); setOrder(DEFAULT_ORDER[value]); setOffset(0); }}>
            <option value="meter_id">Meter ID</option>
            <option value="consumption">Consumption</option>
            <option value="variation">Variation magnitude</option>
            <option value="severity">Severity</option>
          </select>
        </div>
        <Button variant="outline" aria-label={`Sort direction: ${order === "asc" ? "ascending" : "descending"}`}
          onClick={() => { setOrder(order === "asc" ? "desc" : "asc"); setOffset(0); }}>
          {order === "asc" ? "Ascending ↑" : "Descending ↓"}
        </Button>
      </div>
      {content}
    </>
  );
}
