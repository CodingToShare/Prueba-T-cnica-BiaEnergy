import { Cpu, FileText } from "lucide-react";

import type { Explanation } from "@/lib/api/types";
import { formatSystemDateTime } from "@/lib/format/time";

/**
 * Provenance in operator language. The deterministic text is described as
 * evidence-based; generated text names the local model. An unknown future
 * source is shown as reported, never as a known one.
 */
export function explanationProvenance(e: Pick<Explanation, "source" | "model" | "fallback_used">): string {
  switch (e.source) {
    case "OLLAMA":
      return `Generated locally with ${e.model ?? "a local model"}`;
    case "DETERMINISTIC":
      return e.fallback_used ? "Evidence-based fallback" : "Evidence-based explanation";
    default:
      return `Explanation source: ${String(e.source)}`;
  }
}

/**
 * The finding's stored explanation (generated once, during the analysis run).
 * It supplements the structured evidence below; it never replaces the
 * deterministic classification, severity, confidence or action. All text is
 * rendered as plain text.
 */
export function ExplanationPanel({ explanation }: { explanation: Explanation | null }) {
  if (explanation === null) {
    return (
      <section className="panel" aria-labelledby="explanation-title">
        <div className="panel-head">
          <h2 id="explanation-title" className="panel-title">
            Explanation
          </h2>
        </div>
        <div className="panel-body">
          <p className="m-0 text-[0.88rem] text-muted-foreground">
            No explanation was stored for this finding. Its evidence and recommended action are shown on this page.
          </p>
        </div>
      </section>
    );
  }

  const generated = explanation.source === "OLLAMA";
  const Icon = generated ? Cpu : FileText;
  return (
    <section className="panel" aria-labelledby="explanation-title">
      <div className="panel-head flex-wrap gap-2">
        <h2 id="explanation-title" className="panel-title">
          Explanation
        </h2>
        <span className="inline-flex items-center gap-1.5 text-[0.78rem] font-semibold text-muted-foreground">
          <Icon className="size-4" aria-hidden="true" />
          {explanationProvenance(explanation)}
        </span>
      </div>
      <div className="panel-body flex min-w-0 flex-col gap-3">
        <p className="m-0 text-[0.95rem] font-semibold text-heading [overflow-wrap:anywhere]">{explanation.summary}</p>
        <div className="story">
          <div className="story-block min-w-0">
            <h3>Why it matters</h3>
            <p className="[overflow-wrap:anywhere]">{explanation.why_it_matters}</p>
          </div>
          <div className="story-block min-w-0">
            <h3>Evidence</h3>
            <p className="[overflow-wrap:anywhere]">{explanation.evidence_narrative}</p>
          </div>
        </div>
        <p className="m-0 text-[0.8rem] text-muted-foreground">
          Classification, severity and confidence come from the deterministic analysis.{" "}
          {generated ? "The language model only helps explain the evidence." : "This text is built from that evidence."}
        </p>
        <details className="rounded-[10px] border border-border">
          <summary className="cursor-pointer px-3.5 py-2.5 text-[0.84rem] font-bold text-primary">About this explanation</summary>
          <dl className="evidence-list border-t border-border px-3.5 py-3">
            <dt>Source</dt>
            <dd>{explanation.source}</dd>
            <dt>Model</dt>
            <dd>{explanation.model ?? "—"}</dd>
            <dt>Prompt version</dt>
            <dd>{explanation.prompt_version}</dd>
            <dt>Generated</dt>
            <dd>{formatSystemDateTime(explanation.generated_at)}</dd>
            <dt>Fallback used</dt>
            <dd>{explanation.fallback_used ? "Yes — the configured model was unavailable or its output was rejected" : "No"}</dd>
          </dl>
        </details>
      </div>
    </section>
  );
}
