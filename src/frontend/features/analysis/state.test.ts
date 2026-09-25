import { describe, expect, it } from "vitest";

import { completionSummary, isActive, isTerminal, runSteps } from "./state";

describe("analysis state", () => {
  it("knows terminal and active statuses", () => {
    expect(isTerminal("COMPLETED")).toBe(true);
    expect(isTerminal("FAILED")).toBe(true);
    expect(isTerminal("RUNNING")).toBe(false);
    expect(isActive("QUEUED")).toBe(true);
    expect(isActive("COMPLETED")).toBe(false);
  });

  it("marks steps from the actual stage only", () => {
    expect(runSteps({ status: "RUNNING", stage: "ANALYZING" }).map((s) => s.state)).toEqual(["done", "done", "current", "pending", "pending"]);
    expect(runSteps({ status: "QUEUED", stage: "QUEUED" }).map((s) => s.state)).toEqual(["current", "pending", "pending", "pending", "pending"]);
    expect(runSteps({ status: "COMPLETED", stage: "COMPLETED" }).every((s) => s.state === "done")).toBe(true);
  });

  it("summarizes completion with the real counts", () => {
    expect(completionSummary(4, 2)).toBe("4 anomalies detected · 2 require high-priority attention");
    expect(completionSummary(1, 1)).toBe("1 anomaly detected · 1 requires high-priority attention");
  });
});
