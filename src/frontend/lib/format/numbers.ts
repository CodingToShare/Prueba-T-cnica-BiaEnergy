// Domain number formatting (en-US), used by every screen. Missing values
// (null) are shown as an em dash, never as 0.

export const EMPTY = "—";

const format = (value: number, min: number, max: number) =>
  new Intl.NumberFormat("en-US", { minimumFractionDigits: min, maximumFractionDigits: max }).format(value);

const isNumber = (value: number | null | undefined): value is number =>
  typeof value === "number" && Number.isFinite(value);

/** "155,250.85 kWh" */
export function formatKwh(value: number | null | undefined): string {
  return isNumber(value) ? `${format(value, 0, 2)} kWh` : EMPTY;
}

/** "221.9 V" */
export function formatVoltage(value: number | null | undefined): string {
  return isNumber(value) ? `${format(value, 1, 1)} V` : EMPTY;
}

/** "101.3 A" */
export function formatCurrent(value: number | null | undefined): string {
  return isNumber(value) ? `${format(value, 1, 1)} A` : EMPTY;
}

/** "0.954" (dimensionless) */
export function formatPowerFactor(value: number | null | undefined): string {
  return isNumber(value) ? format(value, 3, 3) : EMPTY;
}

/** Signed percent value (the API sends percent numbers): "+109.8%", "−79.7%". */
export function formatPercentSigned(value: number | null | undefined): string {
  if (!isNumber(value)) {
    return EMPTY;
  }
  const rounded = Math.round(value * 10) / 10;
  if (rounded === 0) {
    return "0.0%";
  }
  return `${rounded > 0 ? "+" : "−"}${format(Math.abs(rounded), 1, 1)}%`;
}

/** Confidence fraction (0–1) as a whole percentage: "93%". */
export function formatConfidence(value: number | null | undefined): string {
  return isNumber(value) ? `${Math.round(value * 100)}%` : EMPTY;
}

/** Plain integer with separators: "4,032". */
export function formatCount(value: number | null | undefined): string {
  return isNumber(value) ? format(value, 0, 0) : EMPTY;
}

/** Formats one reading measurement by metric. */
export function formatMeasurement(metric: string, value: number | null | undefined): string {
  switch (metric) {
    case "consumption_kwh":
      return formatKwh(value);
    case "voltage_v":
      return formatVoltage(value);
    case "current_a":
      return formatCurrent(value);
    case "power_factor":
      return formatPowerFactor(value);
    default:
      return isNumber(value) ? format(value, 0, 3) : EMPTY;
  }
}

/** A number with separators and at most `maxFraction` decimals, no unit. */
export function formatNumber(value: number | null | undefined, maxFraction = 2): string {
  return isNumber(value) ? format(value, 0, maxFraction) : EMPTY;
}
