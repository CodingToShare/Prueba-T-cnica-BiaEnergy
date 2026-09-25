import { describe, expect, it } from "vitest";

import {
  actionLabels,
  anomalyTypeLabels,
  anomalyTypeTone,
  confidenceBand,
  eventRoleLabels,
  eventRoleTone,
  severityTone,
  statusTone,
} from "./labels";

describe("enum labels", () => {
  it("keeps readable names that still map to the API terms", () => {
    expect(anomalyTypeLabels).toEqual({
      REAL_ANOMALY: "Real anomaly",
      EXPLAINABLE_ANOMALY: "Explainable anomaly",
      FALSE_POSITIVE: "False positive",
      DATA_QUALITY: "Data quality",
    });
    expect(Object.keys(actionLabels)).toHaveLength(4);
    expect(eventRoleLabels.CONTEXT).toMatch(/does not explain/);
  });
});

describe("semantic tones (design-system.md §4)", () => {
  it("keeps severity, type and status as separate concepts", () => {
    expect(anomalyTypeTone("REAL_ANOMALY", "HIGH")).toBe("critical");
    expect(anomalyTypeTone("REAL_ANOMALY", "MEDIUM")).toBe("warning");
    expect(anomalyTypeTone("DATA_QUALITY", "HIGH")).toBe("data-quality");
    expect(anomalyTypeTone("EXPLAINABLE_ANOMALY", "MEDIUM")).toBe("informational");
    expect(anomalyTypeTone("FALSE_POSITIVE", "LOW")).toBe("informational");
    expect(severityTone("HIGH")).toBe("critical");
    expect(severityTone("LOW")).toBe("neutral");
    expect(statusTone("ALERT")).toBe("warning");
    expect(statusTone(null)).toBe("neutral");
  });

  it("never renders an explanatory-looking tone for context-only events", () => {
    expect(eventRoleTone("EXPLAINS")).toBe("informational");
    expect(eventRoleTone("CORROBORATES")).toBe("data-quality");
    expect(eventRoleTone("CONTEXT")).toBe("neutral");
  });

  it("bands confidence for display only", () => {
    expect(confidenceBand(0.93)).toBe("High");
    expect(confidenceBand(0.73)).toBe("Moderate");
    expect(confidenceBand(0.4)).toBe("Low");
  });
});
