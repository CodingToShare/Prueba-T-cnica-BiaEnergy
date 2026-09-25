// Readable labels for API enums, and their visual tone. The mapping from
// domain values to the five Energy Management semantics is the one defined
// in docs/design/design-system.md §4; it only decides presentation, never
// the classification, severity or status themselves (those come from the API).
import type { AnomalyType, ComputedStatus, RecommendedAction, RunStage, Severity } from "@/lib/api/types";

export type Tone = "normal" | "informational" | "warning" | "critical" | "data-quality" | "neutral";

export const anomalyTypeLabels: Record<AnomalyType, string> = {
  REAL_ANOMALY: "Real anomaly",
  EXPLAINABLE_ANOMALY: "Explainable anomaly",
  FALSE_POSITIVE: "False positive",
  DATA_QUALITY: "Data quality",
};

export const severityLabels: Record<Severity, string> = {
  HIGH: "High",
  MEDIUM: "Medium",
  LOW: "Low",
};

export const statusLabels: Record<ComputedStatus, string> = {
  OK: "OK",
  ALERT: "Alert",
  CRITICAL: "Critical",
};

export const NOT_ANALYZED = "Not analyzed";

export const actionLabels: Record<RecommendedAction, string> = {
  INVESTIGATE_METER_AND_INSTALLATION: "Investigate meter and installation",
  VALIDATE_MEASUREMENT_OR_SENSOR: "Validate measurement or sensor",
  VALIDATE_OPERATIONAL_CHANGE: "Validate the operational change",
  NO_ESCALATION_MONITOR: "No escalation — keep monitoring",
};

/** What each recommended action asks the operator to do (guidance only). */
export const actionDescriptions: Record<RecommendedAction, string> = {
  INVESTIGATE_METER_AND_INSTALLATION:
    "Inspect the meter and its installation on site and confirm whether new loads, faults or equipment changes explain the deviation.",
  VALIDATE_MEASUREMENT_OR_SENSOR:
    "Check the meter's sensors, wiring and communication before trusting its electrical readings; consumption itself looks normal.",
  VALIDATE_OPERATIONAL_CHANGE:
    "Confirm with operations that the recorded change is expected, and review whether the new level should become the reference.",
  NO_ESCALATION_MONITOR:
    "The deviation is explained by a known event and has recovered. No action is needed beyond normal monitoring.",
};

/** Why a finding of each type matters (interpretation of the classification). */
export const typeMeaning: Record<AnomalyType, string> = {
  REAL_ANOMALY:
    "A persistent deviation that no known operational event explains. It may indicate a fault, an unexpected load or a problem in the installation.",
  EXPLAINABLE_ANOMALY:
    "A real, persistent change in consumption that a known operational event accounts for. It needs validation, not escalation as a fault.",
  FALSE_POSITIVE:
    "The detector saw a genuine deviation, but a scheduled event explains it and the readings recovered afterwards, so it is not escalated.",
  DATA_QUALITY:
    "The electrical measurements contradict each other while consumption stays near its baseline: the readings cannot be trusted, rather than the energy use being abnormal.",
};

export const stageLabels: Record<RunStage, string> = {
  QUEUED: "Queued",
  LOADING_DATA: "Loading data",
  ANALYZING: "Analyzing",
  GENERATING_EXPLANATIONS: "Writing explanations",
  PERSISTING_RESULTS: "Saving results",
  COMPLETED: "Completed",
  FAILED: "Failed",
};

export const metricLabels: Record<string, string> = {
  consumption_kwh: "Consumption",
  voltage_v: "Voltage",
  current_a: "Current",
  power_factor: "Power factor",
  consumption_to_load_proxy_ratio: "Consumption-to-load ratio",
};

export const eventRoleLabels: Record<string, string> = {
  EXPLAINS: "Explains the deviation",
  CORROBORATES: "Corroborates the finding",
  CONTEXT: "Context only — does not explain it",
};

export function metricLabel(metric: string): string {
  return metricLabels[metric] ?? metric;
}

/** Anomaly type tone: a high real anomaly is critical, otherwise a warning. */
export function anomalyTypeTone(type: AnomalyType, severity: Severity): Tone {
  switch (type) {
    case "REAL_ANOMALY":
      return severity === "HIGH" ? "critical" : "warning";
    case "DATA_QUALITY":
      return "data-quality";
    case "EXPLAINABLE_ANOMALY":
    case "FALSE_POSITIVE":
      return "informational";
    default:
      return "neutral";
  }
}

export function severityTone(severity: Severity): Tone {
  switch (severity) {
    case "HIGH":
      return "critical";
    case "MEDIUM":
      return "warning";
    default:
      return "neutral";
  }
}

export function statusTone(status: ComputedStatus | null): Tone {
  switch (status) {
    case "OK":
      return "normal";
    case "ALERT":
      return "warning";
    case "CRITICAL":
      return "critical";
    default:
      return "neutral";
  }
}

export function eventRoleTone(role: string): Tone {
  switch (role) {
    case "EXPLAINS":
      return "informational";
    case "CORROBORATES":
      return "data-quality";
    default:
      return "neutral";
  }
}

/** Confidence band shown next to the value (design-system.md §4): presentation only. */
export function confidenceBand(confidence: number): "High" | "Moderate" | "Low" {
  if (confidence >= 0.8) {
    return "High";
  }
  return confidence >= 0.6 ? "Moderate" : "Low";
}

/** Short action wording for compact lists (the full label stays on detail views). */
export const shortActionLabels: Record<RecommendedAction, string> = {
  INVESTIGATE_METER_AND_INSTALLATION: "Investigate",
  VALIDATE_MEASUREMENT_OR_SENSOR: "Validate readings",
  VALIDATE_OPERATIONAL_CHANGE: "Validate operation",
  NO_ESCALATION_MONITOR: "No escalation",
};
