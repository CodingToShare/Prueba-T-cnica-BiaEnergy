import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { apiError, installApiFake } from "@/test/api-fake";
import { anomalyList, completedRun, dashboardAfter, dashboardBefore, meter, meterList, run } from "@/test/fixtures";
import { renderWithClient } from "@/test/render";

import { DashboardView } from "./dashboard-view";

const ANALYSIS = "/api/v1/ai/analysis/9";

function kpi(label: string) {
  const card = screen.getByText(label, { selector: ".kpi-label" }).closest(".kpi");
  if (!(card instanceof HTMLElement)) {
    throw new Error(`KPI ${label} not found`);
  }
  return within(card);
}

function pause(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

const lists = {
  "GET /api/v1/anomalies": { body: anomalyList },
  "GET /api/v1/meters": { body: meterList([meter({ computed_status: "CRITICAL" }), meter({ meter_id: "TST-2", computed_status: "OK" })], 2, 0, true) },
};

describe("DashboardView", () => {
  it("sends one analysis POST for rapid clicks while the start request is pending", async () => {
    let release: () => void = () => undefined;
    const gate = new Promise<void>((resolve) => { release = resolve; });
    const fake = installApiFake({
      "GET /api/v1/dashboard/summary": { body: dashboardBefore },
      "POST /api/v1/ai/analyze": { status: 202, body: { ...completedRun, created: true } },
      [`GET ${ANALYSIS}`]: { body: completedRun }, ...lists,
    });
    const fetchImpl = vi.mocked(fetch).getMockImplementation()!;
    vi.mocked(fetch).mockImplementation(async (...args) => {
      if (args[1]?.method === "POST") await gate;
      return fetchImpl(...args);
    });
    const user = userEvent.setup();
    renderWithClient(<DashboardView />);
    await user.dblClick(await screen.findByRole("button", { name: "Run AI Analysis" }));
    expect(screen.getByRole("button", { name: "Analyzing…" })).toBeDisabled();
    release();
    await screen.findByText(/Analysis completed/);
    expect(fake.count("POST", "/api/v1/ai/analyze")).toBe(1);
  });
  it("distinguishes a completed analysis with no findings from a fleet not yet analyzed", async () => {
    installApiFake({
      "GET /api/v1/dashboard/summary": { body: { ...dashboardAfter, anomalies: 0, high_priority: 0, aggregate_confidence: null } },
      "GET /api/v1/anomalies": { body: { ...anomalyList, items: [], pagination: { limit: 5, offset: 0, total: 0 } } },
      "GET /api/v1/meters": lists["GET /api/v1/meters"],
    });
    renderWithClient(<DashboardView />);
    await screen.findByRole("button", { name: "Run analysis again" });
    expect(kpi("AI anomalies").getByText("0")).toBeInTheDocument();
    expect(kpi("AI confidence").getByText("No findings to score")).toBeInTheDocument();
    expect(screen.queryByText("No analysis yet")).not.toBeInTheDocument();
  });
  it("shows a monitoring error, stops automatic polls and retries the same run without starting another", async () => {
    const fake = installApiFake({
      "GET /api/v1/dashboard/summary": { body: { ...dashboardAfter, active_analysis: { id: 9, status: "RUNNING", stage: "ANALYZING", progress: 45 } } },
      [`GET ${ANALYSIS}`]: apiError(503, "service_unavailable", "The service is unavailable."),
      ...lists,
    });
    const user = userEvent.setup();
    renderWithClient(<DashboardView />);
    expect(await screen.findByRole("alert")).toHaveTextContent("Analysis status could not be loaded");
    expect(kpi("AI anomalies").getByText("3")).toBeInTheDocument();
    const polls = fake.count("GET", ANALYSIS);
    await pause(1600);
    expect(fake.count("GET", ANALYSIS)).toBe(polls);
    fake.set(`GET ${ANALYSIS}`, { body: completedRun });
    await user.click(screen.getByRole("button", { name: "Retry status" }));
    expect(await screen.findByText(/Analysis completed/)).toBeInTheDocument();
    expect(fake.count("POST", "/api/v1/ai/analyze")).toBe(0);
  });
  it("is honest before the first analysis", async () => {
    installApiFake({
      "GET /api/v1/dashboard/summary": { body: dashboardBefore },
      "GET /api/v1/meters": { body: meterList([meter(), meter({ meter_id: "TST-2" })]) },
    });
    renderWithClient(<DashboardView />);

    await screen.findByText("4,321.5");
    expect(await screen.findByText("2 meters, not analyzed yet: run the AI analysis to compute their status.")).toBeInTheDocument();
    expect(kpi("Meters").getByText("3")).toBeInTheDocument();
    for (const label of ["AI anomalies", "High priority", "AI confidence", "Latest analysis"]) {
      expect(kpi(label).getByText("—")).toBeInTheDocument();
      expect(kpi(label).getByText("Not analyzed yet")).toBeInTheDocument();
    }
    expect(screen.getByText("No analysis yet")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Run AI Analysis" })).toBeEnabled();
  });

  it("shows the backend's values after an analysis", async () => {
    installApiFake({ "GET /api/v1/dashboard/summary": { body: dashboardAfter }, ...lists });
    renderWithClient(<DashboardView />);

    await screen.findByRole("button", { name: "Run analysis again" });
    expect(kpi("AI anomalies").getByText("3")).toBeInTheDocument();
    expect(kpi("High priority").getByText("1")).toBeInTheDocument();
    expect(kpi("AI confidence").getByText("71%")).toBeInTheDocument();
    const queue = await screen.findByRole("list", { name: "Findings in priority order" });
    expect(within(queue).getByText("TST-7 · Real anomaly")).toBeInTheDocument();
  });

  it("offers a retry when the summary cannot be loaded", async () => {
    const fake = installApiFake({ "GET /api/v1/dashboard/summary": apiError(503, "database_unavailable", "The database is unavailable.") });
    const user = userEvent.setup();
    renderWithClient(<DashboardView />);

    expect(await screen.findByRole("alert")).toHaveTextContent("The database is unavailable.");
    fake.set("GET /api/v1/dashboard/summary", { body: dashboardBefore });
    await user.click(screen.getByRole("button", { name: "Retry" }));
    await screen.findByText("4,321.5");
  });

  it("handles a run that is already COMPLETED on the first poll", async () => {
    let completed = false;
    const fake = installApiFake({
      "GET /api/v1/dashboard/summary": () => ({ body: completed ? dashboardAfter : dashboardBefore }),
      "POST /api/v1/ai/analyze": { status: 202, body: { ...run(), created: true } },
      [`GET ${ANALYSIS}`]: () => {
        completed = true;
        return { body: completedRun };
      },
      ...lists,
    });
    const user = userEvent.setup();
    renderWithClient(<DashboardView />);

    await user.click(await screen.findByRole("button", { name: "Run AI Analysis" }));

    expect(await screen.findByText(/3 anomalies detected · 1 requires high-priority attention/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View AI anomalies" })).toHaveAttribute("href", "/anomalies");
    // Dashboard, meters and anomalies were refreshed after completion.
    await waitFor(() => expect(kpi("AI anomalies").getByText("3")).toBeInTheDocument());
    expect(fake.count("GET", "/api/v1/dashboard/summary")).toBe(2);
    expect(screen.getByRole("button", { name: "Run analysis again" })).toBeEnabled();

    const polls = fake.count("GET", ANALYSIS);
    await pause(1600);
    expect(fake.count("GET", ANALYSIS)).toBe(polls);
  });

  it("follows progress, stops polling on FAILED and offers a retry", async () => {
    const statuses = [run({ status: "RUNNING", stage: "ANALYZING", progress: 45 })];
    const fake = installApiFake({
      "GET /api/v1/dashboard/summary": { body: dashboardAfter },
      "POST /api/v1/ai/analyze": { status: 202, body: { ...run(), created: true } },
      [`GET ${ANALYSIS}`]: () => ({ body: statuses[0] }),
      ...lists,
    });
    const user = userEvent.setup();
    renderWithClient(<DashboardView />);

    await user.click(await screen.findByRole("button", { name: "Run analysis again" }));
    const progress = await screen.findByRole("progressbar", { name: "Analysis progress" });
    await waitFor(() => expect(progress).toHaveAttribute("aria-valuenow", "45"));
    expect(screen.getByText("Step 3 of 5")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Analyzing…" })).toBeDisabled();
    // Previous results stay visible while the new run is active.
    expect(kpi("AI anomalies").getByText("3")).toBeInTheDocument();

    statuses[0] = run({ status: "FAILED", stage: "FAILED", progress: 45, error: { code: "analysis_failed", message: "The analysis could not be completed." } });
    const alert = await screen.findByRole("alert", {}, { timeout: 3000 });
    expect(alert).toHaveTextContent("The analysis could not be completed. Previous results, if any, are still shown.");

    const polls = fake.count("GET", ANALYSIS);
    await pause(1600);
    expect(fake.count("GET", ANALYSIS)).toBe(polls);

    await user.click(within(alert).getByRole("button", { name: "Retry analysis" }));
    await waitFor(() => expect(fake.count("POST", "/api/v1/ai/analyze")).toBe(2));
  });

  it("recovers an analysis that is already active after a refresh", async () => {
    const fake = installApiFake({
      "GET /api/v1/dashboard/summary": {
        body: { ...dashboardBefore, active_analysis: { id: 9, status: "RUNNING", stage: "LOADING_DATA", progress: 20 } },
      },
      [`GET ${ANALYSIS}`]: { body: run({ status: "RUNNING", stage: "LOADING_DATA", progress: 20 }) },
      ...lists,
    });
    renderWithClient(<DashboardView />);

    expect(await screen.findByText("Step 2 of 5")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Analyzing…" })).toBeDisabled();
    expect(fake.count("POST", "/api/v1/ai/analyze")).toBe(0);
  });

  it("shows a safe message when the analysis cannot be started", async () => {
    installApiFake({
      "GET /api/v1/dashboard/summary": { body: dashboardBefore },
      "POST /api/v1/ai/analyze": apiError(500, "internal_error", "An unexpected error occurred."),
      ...lists,
    });
    const user = userEvent.setup();
    renderWithClient(<DashboardView />);

    await user.click(await screen.findByRole("button", { name: "Run AI Analysis" }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("The analysis could not be started. An unexpected error occurred.");
    expect(within(alert).getByRole("button", { name: "Retry analysis" })).toBeEnabled();
  });
});
