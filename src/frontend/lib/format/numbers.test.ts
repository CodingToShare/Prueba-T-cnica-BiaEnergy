import { describe, expect, it } from "vitest";

import {
  formatConfidence,
  formatCount,
  formatCurrent,
  formatKwh,
  formatMeasurement,
  formatNumber,
  formatPercentSigned,
  formatPowerFactor,
  formatVoltage,
} from "./numbers";

describe("number formatting", () => {
  it("formats energy and electrical measurements with units", () => {
    expect(formatKwh(155250.85)).toBe("155,250.85 kWh");
    expect(formatKwh(12)).toBe("12 kWh");
    expect(formatVoltage(221.9)).toBe("221.9 V");
    expect(formatCurrent(101.28)).toBe("101.3 A");
    expect(formatPowerFactor(0.954)).toBe("0.954");
    expect(formatMeasurement("voltage_v", 220)).toBe("220.0 V");
  });

  it("signs deviations and never shows false precision", () => {
    expect(formatPercentSigned(109.83061925)).toBe("+109.8%");
    expect(formatPercentSigned(-79.689)).toBe("−79.7%");
    expect(formatPercentSigned(0.01)).toBe("0.0%");
  });

  it("shows confidence as a whole percentage", () => {
    expect(formatConfidence(0.933333333333)).toBe("93%");
    expect(formatConfidence(0.756)).toBe("76%");
  });

  it("shows missing values as an em dash, never as zero", () => {
    for (const fmt of [formatKwh, formatVoltage, formatCurrent, formatPowerFactor, formatPercentSigned, formatConfidence, formatCount, formatNumber]) {
      expect(fmt(null)).toBe("—");
      expect(fmt(undefined)).toBe("—");
      expect(fmt(Number.NaN)).toBe("—");
    }
  });

  it("formats counts and plain numbers", () => {
    expect(formatCount(4032)).toBe("4,032");
    expect(formatNumber(155250.8512)).toBe("155,250.85");
    expect(formatNumber(3.8, 1)).toBe("3.8");
  });
});
