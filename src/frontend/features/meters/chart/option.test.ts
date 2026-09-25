import { describe, expect, it } from "vitest";

import type { Reading } from "@/lib/api/types";

import { buildReadingsOption, describeChart, escapeHtml, type ChartTheme } from "./option";

const theme: ChartTheme = {
  series: "s",
  baseline: "b",
  anomalyRegion: "r",
  anomalyBorder: "rb",
  eventMarker: "e",
  grid: "g",
  axis: "a",
  text: "t",
  tooltipBg: "tb",
  tooltipText: "tt",
  font: "f",
};

function reading(timestamp: string, consumption: number): Reading {
  return { timestamp, consumption_kwh: consumption, voltage_v: 220, current_a: 100, power_factor: 0.95, source_status: "OK" };
}

const readings = [
  reading("2030-01-01T22:00:00", 10),
  reading("2030-01-01T23:00:00", 12),
  reading("2030-01-02T00:00:00", 30),
  reading("2030-01-02T01:00:00", 31),
];

describe("buildReadingsOption", () => {
  it("uses the source timestamps as categories (no time-zone conversion)", () => {
    const option = buildReadingsOption({ readings, metric: "consumption_kwh" }, theme);
    expect(option.xAxis.data).toEqual(readings.map((r) => r.timestamp));
    expect(option.series[0].data).toEqual([10, 12, 30, 31]);
    expect(option.series).toHaveLength(1);
    expect(option.xAxis.axisLabel.interval(0, "2030-01-02T00:00:00")).toBe(true);
    expect(option.xAxis.axisLabel.interval(0, "2030-01-01T23:00:00")).toBe(false);
  });

  it("plots the selected metric", () => {
    const option = buildReadingsOption({ readings, metric: "power_factor" }, theme);
    expect(option.series[0].data).toEqual([0.95, 0.95, 0.95, 0.95]);
    expect(option.yAxis.name).toBe("");
  });

  it("overlays the stored episode, exact-time events and engine baseline points only", () => {
    const option = buildReadingsOption(
      {
        readings,
        metric: "consumption_kwh",
        episode: { start: "2030-01-02T00:00:00", end: "2030-01-02T01:00:00" },
        events: [
          { timestamp: "2030-01-02T00:00:00", label: "OPERATIONAL_CHANGE" },
          { timestamp: "2030-01-02T00:30:00", label: "OFF_GRID" },
        ],
        baseline: [{ timestamp: "2030-01-02T00:00:00", value: 11 }],
      },
      theme,
    );
    const main = option.series[0];
    expect(main.markArea?.data[0][0].xAxis).toBe("2030-01-02T00:00:00");
    expect(main.markLine?.data).toEqual([{ name: "OPERATIONAL_CHANGE", xAxis: "2030-01-02T00:00:00" }]);
    expect(option.series[1].data).toEqual([null, null, 11, null]);
  });

  it("escapes text in the HTML tooltip", () => {
    const option = buildReadingsOption({ readings, metric: "consumption_kwh" }, theme);
    const html = option.tooltip.formatter([{ axisValue: "2030-01-02T00:00:00", seriesName: "<img src=x>", value: 30 }]);
    expect(html).toContain("Jan 2, 00:00");
    expect(html).toContain("&lt;img src=x&gt;");
    expect(html).not.toContain("<img");
    expect(escapeHtml(`"&'`)).toBe("&quot;&amp;&#39;");
  });

  it("describes the chart for assistive technology", () => {
    expect(describeChart({ readings, metric: "consumption_kwh" })).toBe(
      "Consumption, 4 hourly readings from Jan 1, 22:00 to Jan 2, 01:00, ranging 10 kWh to 31 kWh.",
    );
  });
});
