"use client";

import type { ECharts } from "echarts/core";
import { useEffect, useRef, useState } from "react";

import { metricLabel } from "@/lib/format/labels";
import { formatMeasurement } from "@/lib/format/numbers";
import { formatSourceShort } from "@/lib/format/time";

import { buildReadingsOption, describeChart, type ChartTheme, type ReadingsChartInput } from "./option";

/** Chart colors come from the design tokens (CSS variables), not literals. */
function readTheme(): ChartTheme {
  const css = getComputedStyle(document.documentElement);
  const v = (name: string) => css.getPropertyValue(name).trim();
  return {
    series: v("--chart-series-1"),
    baseline: v("--chart-baseline"),
    anomalyRegion: v("--chart-anomaly-region"),
    anomalyBorder: v("--chart-anomaly-border"),
    eventMarker: v("--chart-event-marker"),
    grid: v("--chart-grid"),
    axis: v("--chart-axis"),
    text: v("--text-body"),
    tooltipBg: v("--text-heading"),
    tooltipText: v("--text-inverse"),
    font: getComputedStyle(document.body).fontFamily,
  };
}

/**
 * ECharts lifecycle, kept explicit: one instance per mounted element,
 * created on the client only (ECharts is imported lazily), resized with its
 * container, updated in place when data or metric changes, and disposed on
 * unmount so navigation leaks nothing.
 */
export function ReadingsChart(props: ReadingsChartInput & { title: string }) {
  const { title, ...input } = props;
  const elementRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<ECharts | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const element = elementRef.current;
    if (element === null) {
      return;
    }
    let disposed = false;
    const observer = new ResizeObserver(() => chartRef.current?.resize());
    void import("./echarts-core").then(({ echarts }) => {
      if (disposed) {
        return;
      }
      chartRef.current = echarts.getInstanceByDom(element) ?? echarts.init(element, undefined, { renderer: "canvas" });
      observer.observe(element);
      setReady(true);
    });
    return () => {
      disposed = true;
      observer.disconnect();
      chartRef.current?.dispose();
      chartRef.current = null;
    };
  }, []);

  const { readings, metric, episode, events, baseline } = input;
  useEffect(() => {
    if (ready) {
      chartRef.current?.setOption(buildReadingsOption({ readings, metric, episode, events, baseline }, readTheme()), { notMerge: true });
    }
  }, [ready, readings, metric, episode, events, baseline]);

  const description = describeChart(input);

  return (
    <figure className="m-0">
      <div ref={elementRef} className="h-[260px] w-full sm:h-[320px]" role="img" aria-label={`${title}. ${description}`} />
      <figcaption className="sr-only">{description}</figcaption>
      <details className="mt-2 text-[0.82rem]">
        <summary className="cursor-pointer font-bold text-primary">View data as table</summary>
        <div className="mt-2 max-h-72 overflow-auto rounded-[10px] border border-border">
          <table className="tbl">
            <thead>
              <tr>
                <th scope="col">Time (source)</th>
                <th scope="col" className="r">
                  {metricLabel(metric)}
                </th>
              </tr>
            </thead>
            <tbody>
              {readings.map((r) => (
                <tr key={r.timestamp}>
                  <td>{formatSourceShort(r.timestamp)}</td>
                  <td className="r">{formatMeasurement(metric, r[metric])}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </details>
    </figure>
  );
}
