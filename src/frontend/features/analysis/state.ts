// Pure helpers for presenting an analysis run exactly as the backend reports
// it (ADR-009: QUEUED → LOADING_DATA → ANALYZING → GENERATING_EXPLANATIONS →
// PERSISTING_RESULTS → COMPLETED, or FAILED). Nothing here simulates progress.
import type { AnalysisRun, RunStage, RunStatus } from "@/lib/api/types";
import { stageLabels } from "@/lib/format/labels";

/** Polling interval while a run is QUEUED or RUNNING. */
export const POLL_INTERVAL_MS = 750;

export function isTerminal(status: RunStatus): boolean {
  return status === "COMPLETED" || status === "FAILED";
}

export function isActive(status: RunStatus): boolean {
  return status === "QUEUED" || status === "RUNNING";
}

/** The lifecycle steps shown under the progress bar, in order. */
export const RUN_STEPS: readonly RunStage[] = [
  "QUEUED",
  "LOADING_DATA",
  "ANALYZING",
  "GENERATING_EXPLANATIONS",
  "PERSISTING_RESULTS",
  "COMPLETED",
];

export type StepState = "done" | "current" | "pending";

export interface Step {
  stage: RunStage;
  label: string;
  state: StepState;
}

/** Marks each step done/current/pending from the run's actual stage. */
export function runSteps(run: Pick<AnalysisRun, "stage" | "status">): Step[] {
  const currentIndex = run.status === "COMPLETED" ? RUN_STEPS.length : RUN_STEPS.indexOf(run.stage);
  return RUN_STEPS.map((stage, i) => ({
    stage,
    label: stageLabels[stage],
    state: i < currentIndex ? "done" : i === currentIndex ? "current" : "pending",
  }));
}

/** "4 anomalies detected · 2 require high-priority attention", from real counts. */
export function completionSummary(findings: number, highPriority: number): string {
  const found = `${findings} ${findings === 1 ? "anomaly" : "anomalies"} detected`;
  const high = `${highPriority} ${highPriority === 1 ? "requires" : "require"} high-priority attention`;
  return `${found} · ${high}`;
}
