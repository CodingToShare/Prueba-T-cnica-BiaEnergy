import { afterEach, beforeEach, describe, expect, it } from "vitest";

import {
  formatDurationHours,
  formatSourceDateTime,
  formatSourceDay,
  formatSourcePeriod,
  formatSourceShort,
  formatSystemDate,
  formatSystemDateTime,
  formatSystemTime,
  parseSourceTime,
} from "./time";

describe("source (wall-clock) times", () => {
  const original = process.env.TZ;
  beforeEach(() => {
    // A zone far from UTC: if anything went through Date, the hour would move.
    process.env.TZ = "Pacific/Kiritimati";
  });
  afterEach(() => {
    process.env.TZ = original;
  });

  it("keeps the recorded hour regardless of the browser time zone", () => {
    expect(formatSourceDateTime("2026-09-12T14:00:00")).toBe("Sep 12, 2026, 14:00");
    expect(formatSourceShort("2026-09-12T14:00:00")).toBe("Sep 12, 14:00");
    expect(formatSourceDay("2026-09-01T00:00:00")).toBe("Sep 1");
    expect(formatSourceShort("2026-09-14T23:00:00")).toBe("Sep 14, 23:00");
  });

  it("parses only the exact source layout", () => {
    expect(parseSourceTime("2026-09-12T14:05:09")).toEqual({ year: 2026, month: 9, day: 12, hour: 14, minute: 5, second: 9 });
    for (const bad of ["2026-09-12T14:00:00Z", "2026-09-12T14:00:00+02:00", "2026-09-12 14:00:00", "2026-13-01T00:00:00", "garbage"]) {
      expect(parseSourceTime(bad)).toBeNull();
    }
    expect(formatSourceShort("not-a-time")).toBe("not-a-time");
  });

  it("formats periods", () => {
    expect(formatSourcePeriod("2026-09-01T00:00:00", "2026-09-14T23:00:00")).toBe("Sep 1 – Sep 14, 2026");
    expect(formatSourcePeriod("2025-12-30T00:00:00", "2026-01-02T00:00:00")).toBe("Dec 30, 2025 – Jan 2, 2026");
  });
});

describe("system instants", () => {
  it("are converted to the requested zone", () => {
    expect(formatSystemDateTime("2026-09-24T23:18:00Z", "UTC")).toBe("Sep 24, 2026, 23:18");
    expect(formatSystemDateTime("2026-09-24T23:18:00Z", "America/Bogota")).toBe("Sep 24, 2026, 18:18");
    expect(formatSystemTime("2026-09-24T23:18:00Z", "America/Bogota")).toBe("18:18");
    expect(formatSystemDate("2026-09-25T02:00:00Z", "America/Bogota")).toBe("Sep 24, 2026");
  });

  it("returns invalid input unchanged", () => {
    expect(formatSystemDateTime("nope")).toBe("nope");
  });
});

describe("durations", () => {
  it("uses hours with sensible precision", () => {
    expect(formatDurationHours(58)).toBe("58 h");
    expect(formatDurationHours(1.5)).toBe("1.5 h");
    expect(formatDurationHours(Number.NaN)).toBe("—");
  });
});
