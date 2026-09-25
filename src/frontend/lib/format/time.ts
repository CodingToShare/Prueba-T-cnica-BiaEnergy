// Two different kinds of time (docs/api/openapi.yaml, ADR-008):
//
// - Source times ("2026-09-12T14:00:00") are wall-clock values from the
//   dataset with no time zone. They are parsed as text and never passed
//   through Date, so no browser time zone can shift the displayed hour.
// - System times ("2026-09-24T23:18:00Z") are real instants and are shown in
//   the viewer's local time.

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"] as const;
const SOURCE_PATTERN = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})$/;

export interface SourceTimeParts {
  year: number;
  month: number; // 1–12
  day: number;
  hour: number;
  minute: number;
  second: number;
}

/** Parses a source wall-clock time; returns null for anything else. */
export function parseSourceTime(value: string): SourceTimeParts | null {
  const match = SOURCE_PATTERN.exec(value);
  if (match === null) {
    return null;
  }
  const [year, month, day, hour, minute, second] = match.slice(1).map(Number);
  if (month < 1 || month > 12 || day < 1 || day > 31 || hour > 23 || minute > 59 || second > 59) {
    return null;
  }
  return { year, month, day, hour, minute, second };
}

const pad = (n: number) => String(n).padStart(2, "0");

/** "Sep 12, 2026, 14:00" — the wall clock exactly as recorded. */
export function formatSourceDateTime(value: string): string {
  const p = parseSourceTime(value);
  if (p === null) {
    return value;
  }
  return `${MONTHS[p.month - 1]} ${p.day}, ${p.year}, ${pad(p.hour)}:${pad(p.minute)}`;
}

/** "Sep 12, 14:00" — compact form for tables and chart tooltips. */
export function formatSourceShort(value: string): string {
  const p = parseSourceTime(value);
  if (p === null) {
    return value;
  }
  return `${MONTHS[p.month - 1]} ${p.day}, ${pad(p.hour)}:${pad(p.minute)}`;
}

/** "Sep 12" — chart axis day label. */
export function formatSourceDay(value: string): string {
  const p = parseSourceTime(value);
  return p === null ? value : `${MONTHS[p.month - 1]} ${p.day}`;
}

/** "Sep 1 – Sep 14, 2026" for a source-time period. */
export function formatSourcePeriod(from: string, to: string): string {
  const a = parseSourceTime(from);
  const b = parseSourceTime(to);
  if (a === null || b === null) {
    return `${from} – ${to}`;
  }
  const start = a.year === b.year ? `${MONTHS[a.month - 1]} ${a.day}` : `${MONTHS[a.month - 1]} ${a.day}, ${a.year}`;
  return `${start} – ${MONTHS[b.month - 1]} ${b.day}, ${b.year}`;
}

/**
 * Formats a system instant (RFC 3339) in the viewer's local time, or in
 * `timeZone` when given (used by tests).
 */
export function formatSystemDateTime(value: string, timeZone?: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
    timeZone,
  }).format(date);
}

/** "58 h", "12 h", "1.5 h". */
export function formatDurationHours(hours: number): string {
  if (!Number.isFinite(hours)) {
    return "—";
  }
  const rounded = Math.round(hours * 10) / 10;
  return `${Number.isInteger(rounded) ? rounded.toFixed(0) : rounded.toFixed(1)} h`;
}

/** Time of day of a system instant in the viewer's zone: "18:50". */
export function formatSystemTime(value: string, timeZone?: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("en-US", { hour: "2-digit", minute: "2-digit", hour12: false, timeZone }).format(date);
}

/** Date of a system instant in the viewer's zone: "Sep 24, 2026". */
export function formatSystemDate(value: string, timeZone?: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric", year: "numeric", timeZone }).format(date);
}
