import { screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { AnomalyDetail } from "@/lib/api/types";
import { apiError, installApiFake } from "@/test/api-fake";
import { anomalyDetail, explanation } from "@/test/fixtures";
import { renderWithClient } from "@/test/render";

import { InvestigationView } from "./investigation-view";

// The chart has its own tests; here it is replaced by a marker.
vi.mock("@/features/meters/meter-readings", () => ({
  MeterReadings: ({ meterId }: { meterId: string }) => <div data-testid="readings">{meterId}</div>,
}));

function show(detail: AnomalyDetail) {
  installApiFake({ [`GET /api/v1/anomalies/${detail.id}`]: { body: detail } });
  renderWithClient(<InvestigationView id={detail.id} />);
}

describe("InvestigationView", () => {
  it("renders script-like evidence and long API text as plain text", async () => {
    const text = '<script>window.injected=true</script><img src=x onerror="window.injected=true">';
    const reason = `${text} ${"Long explanation supported by recorded evidence. ".repeat(25)}`;
    show(anomalyDetail({ reason }));
    expect(await screen.findByText(reason.trim())).toBeInTheDocument();
    expect(document.querySelector("script, img[src=x]")).toBeNull();
  });
  it("keeps future enum values readable without inventing a classification or crashing", async () => {
    show(anomalyDetail({ type: "FUTURE_TYPE", recommended_action: "FUTURE_ACTION", severity: "FUTURE_SEVERITY" } as unknown as Partial<AnomalyDetail>));
    expect(await screen.findByRole("heading", { level: 1, name: "TST-7 · FUTURE_TYPE" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "FUTURE_ACTION" })).toBeInTheDocument();
    expect(screen.getByText("Severity FUTURE_SEVERITY")).toBeInTheDocument();
  });
  it("answers what happened, why it matters, what supports it and what to do", async () => {
    show(anomalyDetail());

    expect(await screen.findByRole("heading", { level: 1, name: "TST-7 · Real anomaly" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "What happened" }).nextSibling).toHaveTextContent("Consumption rose well above its baseline.");
    expect(screen.getByRole("heading", { name: "Why it matters" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "What supports it" }).nextSibling).toHaveTextContent(
      "+94.1% median consumption deviation over 58 h; current, power factor also changed.",
    );
    expect(screen.getByRole("heading", { name: "Investigate meter and installation" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View meter TST-7" })).toHaveAttribute("href", "/meters/TST-7");
    expect(screen.getByText("Confidence 88% · High")).toBeInTheDocument();
    expect(screen.getByTestId("readings")).toHaveTextContent("TST-7");
  });

  it("never presents a context-only event as an explanation", async () => {
    show(anomalyDetail());

    const callout = await screen.findByRole("note");
    expect(callout).toHaveTextContent("Unexplained deviation.");
    expect(callout).toHaveTextContent("context only; none explains this change");
    const events = screen.getByRole("heading", { name: "Operational context" }).closest("section")!;
    expect(within(events).getByText("Context only — does not explain it")).toBeInTheDocument();
    // API text is rendered as text, not HTML.
    expect(within(events).getByText("Nothing was reported <b>here</b>")).toBeInTheDocument();
    expect(events.querySelector("b")).toBeNull();
  });

  it("shows the explaining event of an explainable anomaly", async () => {
    show(
      anomalyDetail({
        type: "EXPLAINABLE_ANOMALY",
        severity: "MEDIUM",
        recommended_action: "VALIDATE_OPERATIONAL_CHANGE",
        evidence: {
          ...anomalyDetail().evidence,
          related_events: [
            { timestamp: "2030-01-12T11:00:00", type: "OPERATIONAL_CHANGE", description: "New line", offset_seconds: -10800, role: "EXPLAINS" },
          ],
        },
      }),
    );

    const callout = await screen.findByRole("note");
    expect(callout).toHaveTextContent("Explained change. OPERATIONAL_CHANGE recorded 3 h before the onset (Jan 12, 11:00).");
    expect(screen.getByText("Explains the deviation")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Validate the operational change" })).toBeInTheDocument();
  });

  it("marks a recovered false positive as not needing escalation", async () => {
    const base = anomalyDetail();
    show(
      anomalyDetail({
        type: "FALSE_POSITIVE",
        severity: "LOW",
        recommended_action: "NO_ESCALATION_MONITOR",
        evidence: {
          ...base.evidence,
          persistence: {
            ...base.evidence.persistence,
            recovery: { recovered: true, recovered_at: "2030-01-13T02:00:00", readings_observed: 6, median_consumption_deviation_pct: 1.2 },
          },
          related_events: [
            { timestamp: "2030-01-12T14:00:00", type: "SCHEDULED_OUTAGE", description: "Planned work", offset_seconds: 0, role: "EXPLAINS" },
          ],
        },
      }),
    );

    const callout = await screen.findByRole("note");
    expect(callout).toHaveTextContent(
      "Detected, explained and recovered. SCHEDULED_OUTAGE recorded at the onset. Readings recovered from Jan 13, 02:00; no escalation is needed.",
    );
    expect(screen.getByText("False positive")).toBeInTheDocument();
  });

  it("describes a data-quality finding from its corroborating variables", async () => {
    show(anomalyDetail({ type: "DATA_QUALITY", severity: "MEDIUM", recommended_action: "VALIDATE_MEASUREMENT_OR_SENSOR" }));

    const callout = await screen.findByRole("note");
    expect(callout).toHaveTextContent("Measurement inconsistency. Inconsistent current, power factor while consumption stayed within ±120% of its baseline.");
    expect(screen.getByRole("heading", { name: "Validate measurement or sensor" })).toBeInTheDocument();
  });

  it("keeps technical evidence collapsed by default", async () => {
    show(anomalyDetail());
    const summary = await screen.findByText("Technical evidence");
    expect(summary.closest("details")).not.toHaveAttribute("open");
  });

  it("shows a not-found state for unknown findings", async () => {
    installApiFake({ "GET /api/v1/anomalies/999": apiError(404, "anomaly_not_found", "Anomaly was not found.") });
    renderWithClient(<InvestigationView id={999} />);
    expect(await screen.findByText("Anomaly not found")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "All anomalies" })).toHaveAttribute("href", "/anomalies");
  });
});

describe("InvestigationView — explanation", () => {
  function explanationPanel() {
    return screen.getByRole("heading", { level: 2, name: "Explanation" }).closest("section") as HTMLElement;
  }

  it("presents a locally generated explanation with its model and the transparency note", async () => {
    show(anomalyDetail({ explanation: explanation({ source: "OLLAMA", model: "llama3.2:3b", prompt_version: "energy-explanation-v1", summary: "Generated summary." }) }));

    await screen.findByRole("heading", { level: 2, name: "Explanation" });
    const panel = explanationPanel();
    expect(within(panel).getByText("Generated locally with llama3.2:3b")).toBeInTheDocument();
    expect(within(panel).getByText("Generated summary.")).toBeInTheDocument();
    expect(within(panel).getByRole("heading", { level: 3, name: "Why it matters" })).toBeInTheDocument();
    expect(within(panel).getByRole("heading", { level: 3, name: "Evidence" })).toBeInTheDocument();
    expect(panel).toHaveTextContent("Classification, severity and confidence come from the deterministic analysis. The language model only helps explain the evidence.");
    // The explanation is page content, not a live announcement.
    expect(panel.querySelector("[aria-live]")).toBeNull();
    // It supplements the deterministic summary: "Why it matters" appears once, in the explanation.
    expect(screen.getAllByRole("heading", { name: "Why it matters" })).toHaveLength(1);
    expect(screen.getByRole("heading", { name: "What happened" })).toBeInTheDocument();
  });

  it("keeps provenance details collapsed and shows model, prompt version and fallback status", async () => {
    show(anomalyDetail({ explanation: explanation({ source: "OLLAMA", model: "llama3.2:3b", prompt_version: "energy-explanation-v1" }) }));

    await screen.findByRole("heading", { level: 2, name: "Explanation" });
    const details = within(explanationPanel()).getByText("About this explanation").closest("details") as HTMLElement;
    expect(details).not.toHaveAttribute("open");
    expect(details).toHaveTextContent("SourceOLLAMA");
    expect(details).toHaveTextContent("Modelllama3.2:3b");
    expect(details).toHaveTextContent("Prompt versionenergy-explanation-v1");
    expect(details).toHaveTextContent("Fallback usedNo");
  });

  it("labels deterministic text as evidence-based, without a model", async () => {
    show(anomalyDetail({ explanation: explanation() }));

    await screen.findByRole("heading", { level: 2, name: "Explanation" });
    const panel = explanationPanel();
    expect(within(panel).getByText("Evidence-based explanation")).toBeInTheDocument();
    expect(panel).toHaveTextContent("This text is built from that evidence.");
    expect(panel).not.toHaveTextContent("language model");
    expect(panel).toHaveTextContent("Model—");
  });

  it("shows a fallback subtly, as a label, with no error banner", async () => {
    show(anomalyDetail({ explanation: explanation({ fallback_used: true }) }));

    await screen.findByRole("heading", { level: 2, name: "Explanation" });
    const panel = explanationPanel();
    expect(within(panel).getByText("Evidence-based fallback")).toBeInTheDocument();
    expect(within(panel).queryByRole("alert")).toBeNull();
    expect(panel).toHaveTextContent("Fallback usedYes");
  });

  it("words the action card with the stored text while the action itself stays the deterministic code", async () => {
    show(anomalyDetail({ explanation: explanation({ recommended_action_text: "Check the installation on site today." }) }));

    const action = (await screen.findByRole("heading", { name: "Investigate meter and installation" })).closest("section") as HTMLElement;
    expect(action).toHaveTextContent("Check the installation on site today.");
  });

  it("renders generated HTML-like content as text", async () => {
    show(anomalyDetail({ explanation: explanation({ source: "OLLAMA", model: "m", summary: "<script>alert(1)</script>", why_it_matters: "<b>bold</b>" }) }));

    await screen.findByRole("heading", { level: 2, name: "Explanation" });
    expect(within(explanationPanel()).getByText("<script>alert(1)</script>")).toBeInTheDocument();
    expect(within(explanationPanel()).getByText("<b>bold</b>")).toBeInTheDocument();
    expect(document.querySelector("main script, section script, b")).toBeNull();
  });

  it("handles a missing model and an unknown future source without inventing provenance", async () => {
    show(anomalyDetail({ explanation: explanation({ source: "OLLAMA", model: null }) }));
    expect(await screen.findByText("Generated locally with a local model")).toBeInTheDocument();
  });

  it("shows an unknown source as reported", async () => {
    show(anomalyDetail({ explanation: explanation({ source: "FUTURE_PROVIDER" as unknown as "OLLAMA" }) }));
    await screen.findByRole("heading", { level: 2, name: "Explanation" });
    expect(within(explanationPanel()).getByText("Explanation source: FUTURE_PROVIDER")).toBeInTheDocument();
    expect(explanationPanel()).not.toHaveTextContent("language model");
  });

  it("keeps the page usable with maximum-length text", async () => {
    const long = "word ".repeat(140).trim();
    show(anomalyDetail({ explanation: explanation({ summary: long.slice(0, 320), why_it_matters: long, evidence_narrative: long, recommended_action_text: long }) }));

    await screen.findByRole("heading", { level: 2, name: "Explanation" });
    expect(screen.getByRole("heading", { name: "Investigate meter and installation" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View meter TST-7" })).toBeInTheDocument();
  });

  it("tells when a finding has no stored explanation", async () => {
    show(anomalyDetail({ explanation: null }));
    await screen.findByRole("heading", { level: 2, name: "Explanation" });
    expect(explanationPanel()).toHaveTextContent("No explanation was stored for this finding.");
    expect(screen.getByRole("heading", { name: "Why it matters" })).toBeInTheDocument();
  });
});
