import type { ReactNode } from "react";

import type { AnomalyType, ComputedStatus, Severity } from "@/lib/api/types";
import {
  NOT_ANALYZED,
  anomalyTypeLabels,
  anomalyTypeTone,
  confidenceBand,
  severityLabels,
  severityTone,
  statusLabels,
  statusTone,
  type Tone,
} from "@/lib/format/labels";
import { formatConfidence } from "@/lib/format/numbers";

/** Reference badge: tone color + text + shape (color is never the only cue). */
export function Badge({ tone, children, title }: { tone: Tone; children: ReactNode; title?: string }) {
  return (
    <span className={`badge badge-${tone}`} title={title}>
      {children}
    </span>
  );
}

export function AnomalyTypeBadge({ type, severity }: { type: AnomalyType; severity: Severity }) {
  return <Badge tone={anomalyTypeTone(type, severity)}>{anomalyTypeLabels[type] ?? type}</Badge>;
}

export function SeverityBadge({ severity, prefix = "" }: { severity: Severity; prefix?: string }) {
  return (
    <Badge tone={severityTone(severity)}>
      {prefix}
      {severityLabels[severity] ?? severity}
    </Badge>
  );
}

/** Computed meter status; null means the meter has not been analyzed yet. */
export function StatusBadge({ status }: { status: ComputedStatus | null }) {
  return <Badge tone={statusTone(status)}>{status === null ? NOT_ANALYZED : (statusLabels[status] ?? status)}</Badge>;
}

export function ConfidenceBadge({ confidence }: { confidence: number }) {
  return (
    <Badge
      tone="informational"
      title="Confidence reflects the strength of the evidence supporting this classification; it is not a failure probability."
    >
      Confidence {formatConfidence(confidence)} · {confidenceBand(confidence)}
    </Badge>
  );
}
