"use client";

import Link from "next/link";

import { errorMessage } from "@/components/states";
import { Button } from "@/components/ui/button";
import type { AnalysisRun } from "@/lib/api/types";
import { stageLabels } from "@/lib/format/labels";

import { completionSummary, isActive, runSteps } from "./state";

/**
 * Truthful status of the current run: live progress while it is active, a
 * short completion summary with the real counts, or a safe failure message
 * with a retry. Renders nothing when no run is being followed.
 */
export function AnalysisStatus({
  run,
  startError,
  onRetry,
  retrying,
}: {
  run: AnalysisRun | undefined;
  startError: unknown;
  onRetry: () => void;
  retrying: boolean;
}) {
  if (startError) {
    return (
      <div className="alert alert-error" role="alert">
        <span>
          <strong>The analysis could not be started.</strong> {errorMessage(startError)}
        </span>
        <button type="button" className="alert-link" onClick={onRetry} disabled={retrying}>
          Retry analysis
        </button>
      </div>
    );
  }
  if (run === undefined) {
    return null;
  }

  if (isActive(run.status)) {
    const steps = runSteps(run);
    const position = steps.findIndex((s) => s.state === "current") + 1;
    return (
      <section className="panel" aria-label="Analysis progress">
        <div className="panel-body" role="status" aria-live="polite">
          <div className="flex justify-between gap-4 text-[0.88rem] font-bold text-heading">
            <span>{stageLabels[run.stage] ?? run.stage}…</span>
            <span className="num">
              Step {position} of {steps.length}
            </span>
          </div>
          <div
            className="progress-track"
            role="progressbar"
            aria-label="Analysis progress"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={run.progress}
          >
            <span style={{ width: `${run.progress}%` }} />
          </div>
          <ol className="steps" aria-label="Analysis stages">
            {steps.map((step) => (
              <li key={step.stage} data-state={step.state} aria-current={step.state === "current" ? "step" : undefined}>
                {step.label}
              </li>
            ))}
          </ol>
        </div>
      </section>
    );
  }

  if (run.status === "FAILED") {
    return (
      <div className="alert alert-error" role="alert">
        <span>
          <strong>The analysis failed.</strong>{" "}
          {run.error?.message ?? "The analysis did not complete."} Previous results, if any, are still shown.
        </span>
        <button type="button" className="alert-link" onClick={onRetry} disabled={retrying}>
          Retry analysis
        </button>
      </div>
    );
  }

  if (run.status !== "COMPLETED") {
    return <div className="alert alert-warning" role="status">Analysis status: {run.status}. Review the run before starting another analysis.</div>;
  }
  return (
    <div className="alert alert-informational" role="status">
      <span>
        <strong>Analysis completed.</strong> {completionSummary(run.findings_count ?? 0, run.high_priority_count ?? 0)}.
      </span>
      <Button asChild variant="ghost" size="sm" className="h-auto px-0 text-informational underline underline-offset-[3px] hover:bg-transparent">
        <Link href="/anomalies">View AI anomalies</Link>
      </Button>
    </div>
  );
}
