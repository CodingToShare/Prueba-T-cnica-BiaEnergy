import { describe, expect, it } from "vitest";

import { anomalyDetail } from "@/test/fixtures";

import { changedVariables, chartOverlays, formatOffset, hasExplanation, keyFacts, supportingVariables } from "./evidence";

describe("evidence presentation", () => {
  const detail = anomalyDetail();

  it("formats the stored consumption and persistence facts", () => {
    const facts = Object.fromEntries(keyFacts(detail.evidence).map((f) => [f.label, f.value]));
    expect(facts["Baseline energy (flagged hours)"]).toBe("2,000 kWh");
    expect(facts["Observed energy (flagged hours)"]).toBe("3,904 kWh");
    expect(facts["Consumption deviation"]).toBe("+95.2%");
    expect(facts.Duration).toBe("58 h · sustained");
    expect(facts["Flagged readings"]).toBe("58 of 58 (100%)");
    expect(facts.Recovery).toBe("Not observed");
  });

  it("lists variables and which ones support the finding (as stored)", () => {
    const rows = changedVariables(detail.evidence);
    expect(rows.map((r) => r.label)).toEqual(["Consumption", "Current", "Power factor", "Voltage"]);
    expect(supportingVariables(detail.evidence).map((r) => r.label)).toEqual(["Current", "Power factor"]);
    expect(rows.find((r) => r.metric === "power_factor")?.direction).toBe("Down");
    expect(rows.find((r) => r.metric === "voltage_v")?.direction).toBe("—");
  });

  it("does not treat context-only events as an explanation", () => {
    expect(hasExplanation(detail.evidence.related_events)).toBe(false);
    expect(hasExplanation([{ ...detail.evidence.related_events[0], role: "EXPLAINS" }])).toBe(true);
  });

  it("describes event offsets", () => {
    expect(formatOffset(0)).toBe("at the onset");
    expect(formatOffset(-10800)).toBe("3 h before the onset");
    expect(formatOffset(1800)).toBe("30 min after the onset");
  });

  it("derives chart overlays only from stored evidence", () => {
    const overlays = chartOverlays(detail, "consumption_kwh");
    expect(overlays.episode).toEqual({ start: "2030-01-12T14:00:00", end: "2030-01-14T23:00:00" });
    expect(overlays.events).toEqual([{ timestamp: "2030-01-12T14:00:00", label: "UNKNOWN" }]);
    expect(overlays.baseline).toEqual([{ timestamp: "2030-01-12T14:00:00", value: 46 }]);
    expect(chartOverlays(detail, "voltage_v").baseline).toEqual([]);
  });
});
