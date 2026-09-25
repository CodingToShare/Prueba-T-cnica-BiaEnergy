import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { installApiFake, type Handler } from "@/test/api-fake";
import { meter, meterList } from "@/test/fixtures";
import { renderWithClient } from "@/test/render";

import { MetersView } from "./meters-view";

function lastMetersQuery(fake: ReturnType<typeof installApiFake>) {
  const calls = fake.calls.filter((c) => c.url.pathname === "/api/v1/meters");
  return calls[calls.length - 1].url.searchParams;
}

describe("MetersView", () => {
  it("shows unanalyzed meters honestly: status 'Not analyzed' and '—', never 0%", async () => {
    installApiFake({ "GET /api/v1/meters": { body: meterList([meter()]) } });
    renderWithClient(<MetersView />);

    const row = (await screen.findByRole("link", { name: "TST-1" })).closest("tr")!;
    const cells = within(row);
    expect(cells.getByText("1,000.25 kWh")).toBeInTheDocument();
    expect(cells.getByText("Not analyzed")).toBeInTheDocument();
    expect(cells.getAllByText("—")).toHaveLength(4);
    expect(row).not.toHaveTextContent("0%");
    expect(screen.getByText(/Not analyzed yet: run the AI analysis/)).toBeInTheDocument();
  });

  it("renders the backend's status, finding and signed variation", async () => {
    installApiFake({
      "GET /api/v1/meters": {
        body: meterList(
          [meter({ variation_pct: -79.7, computed_status: "ALERT", anomaly_id: 5, anomaly_type: "DATA_QUALITY", severity: "MEDIUM", confidence: 0.74, priority: 2 })],
          1,
          0,
          true,
        ),
      },
    });
    renderWithClient(<MetersView />);

    const row = (await screen.findByRole("link", { name: "TST-1" })).closest("tr")!;
    expect(within(row).getByText("−79.7%")).toBeInTheDocument();
    expect(within(row).getByText("Alert")).toBeInTheDocument();
    expect(within(row).getByText("Data quality")).toBeInTheDocument();
    expect(within(row).getByText("74%")).toBeInTheDocument();
  });

  it("filters by computed status and sorts through the API", async () => {
    const fake = installApiFake({ "GET /api/v1/meters": { body: meterList([meter()], 1, 0, true) } });
    const user = userEvent.setup();
    renderWithClient(<MetersView />);
    await screen.findByRole("link", { name: "TST-1" });

    const group = screen.getByRole("group", { name: "Filter by computed status" });
    await user.click(within(group).getByRole("button", { name: "Critical" }));
    expect(within(group).getByRole("button", { name: "Critical" })).toHaveAttribute("aria-pressed", "true");
    await waitFor(() => expect(lastMetersQuery(fake).get("status")).toBe("CRITICAL"));

    const variation = screen.getByRole("columnheader", { name: /Variation/ });
    expect(variation).toHaveAttribute("aria-sort", "none");
    await user.click(within(variation).getByRole("button"));
    expect(variation).toHaveAttribute("aria-sort", "descending");
    await waitFor(() => {
      const query = lastMetersQuery(fake);
      expect(query.get("sort")).toBe("variation");
      expect(query.get("order")).toBe("desc");
    });
    await user.click(within(variation).getByRole("button"));
    expect(variation).toHaveAttribute("aria-sort", "ascending");
  });

  it("debounces the search and offers to clear an empty result", async () => {
    const handler: Handler = ({ url }) =>
      ({ body: url.searchParams.get("search") ? meterList([], 0, 0, true) : meterList([meter()], 1, 0, true) });
    const fake = installApiFake({ "GET /api/v1/meters": handler });
    const user = userEvent.setup();
    renderWithClient(<MetersView />);
    await screen.findByRole("link", { name: "TST-1" });

    await user.type(screen.getByLabelText("Search by meter ID"), "ZZZ");
    expect(await screen.findByText("No meters match")).toBeInTheDocument();
    expect(lastMetersQuery(fake).get("search")).toBe("ZZZ");
    expect(fake.calls.filter((c) => c.url.searchParams.get("search") === "Z")).toHaveLength(0);

    await user.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(await screen.findByRole("link", { name: "TST-1" })).toBeInTheDocument();
  });

  it("paginates with offsets", async () => {
    const items = Array.from({ length: 10 }, (_, i) => meter({ meter_id: `TST-${i + 1}` }));
    const fake = installApiFake({
      "GET /api/v1/meters": ({ url }) => {
        const offset = Number(url.searchParams.get("offset"));
        return { body: meterList(offset === 0 ? items : [meter({ meter_id: "TST-11" })], 11, offset) };
      },
    });
    const user = userEvent.setup();
    renderWithClient(<MetersView />);

    expect(await screen.findByText("Showing 1–10 of 11")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Next" }));
    expect(await screen.findByText("Showing 11–11 of 11")).toBeInTheDocument();
    expect(lastMetersQuery(fake).get("offset")).toBe("10");
    expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
  });
});
