import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import type { AnomalyList } from "@/lib/api/types";
import { installApiFake } from "@/test/api-fake";
import { anomalyList } from "@/test/fixtures";
import { renderWithClient } from "@/test/render";

import { AnomaliesView } from "./anomalies-view";

const twoFindings: AnomalyList = {
  ...anomalyList,
  items: [
    anomalyList.items[0],
    {
      ...anomalyList.items[0],
      id: 42,
      meter_id: "TST-3",
      priority: 2,
      type: "FALSE_POSITIVE",
      severity: "LOW",
      confidence: 0.756,
      recommended_action: "NO_ESCALATION_MONITOR",
    },
  ],
  pagination: { limit: 20, offset: 0, total: 2 },
};

function lastQuery(fake: ReturnType<typeof installApiFake>) {
  const calls = fake.calls.filter((c) => c.url.pathname === "/api/v1/anomalies");
  return calls[calls.length - 1].url.searchParams;
}

describe("AnomaliesView", () => {
  it("lists findings in the backend's priority order, false positives included", async () => {
    installApiFake({ "GET /api/v1/anomalies": { body: twoFindings } });
    renderWithClient(<AnomaliesView />);

    const first = await screen.findByRole("link", { name: "Investigate TST-7, Real anomaly" });
    expect(first).toHaveAttribute("href", "/anomalies/41");
    const rows = screen.getAllByRole("row").slice(1);
    expect(within(rows[0]).getByText("#1")).toBeInTheDocument();
    expect(within(rows[1]).getByText("False positive")).toBeInTheDocument();
    expect(within(rows[1]).getByText("No escalation — keep monitoring")).toBeInTheDocument();
    expect(within(rows[1]).getByText("76%")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Priority" })).toHaveAttribute("aria-sort", "ascending");
  });

  it("says when no analysis has run yet", async () => {
    installApiFake({ "GET /api/v1/anomalies": { body: { items: [], pagination: { limit: 20, offset: 0, total: 0 }, analysis: null } } });
    renderWithClient(<AnomaliesView />);

    expect(await screen.findByText("No analysis yet")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Go to dashboard" })).toHaveAttribute("href", "/dashboard");
  });

  it("filters by severity, type and a valid meter ID through the API", async () => {
    const fake = installApiFake({ "GET /api/v1/anomalies": { body: twoFindings } });
    const user = userEvent.setup();
    renderWithClient(<AnomaliesView />);
    await screen.findByRole("link", { name: "Investigate TST-7, Real anomaly" });

    await user.click(within(screen.getByRole("group", { name: "Filter by severity" })).getByRole("button", { name: "High" }));
    await waitFor(() => expect(lastQuery(fake).get("severity")).toBe("HIGH"));

    await user.selectOptions(screen.getByLabelText("Type"), "REAL_ANOMALY");
    await waitFor(() => expect(lastQuery(fake).get("type")).toBe("REAL_ANOMALY"));

    const meter = screen.getByLabelText("Meter ID");
    await user.type(meter, "bad id");
    expect(await screen.findByText(/Use letters, digits/)).toBeInTheDocument();
    expect(meter).toHaveAttribute("aria-invalid", "true");
    expect(fake.calls.some((c) => c.url.searchParams.has("meter_id"))).toBe(false);

    await user.clear(meter);
    await user.type(meter, "TST-7");
    await waitFor(() => expect(lastQuery(fake).get("meter_id")).toBe("TST-7"));
  });
});
