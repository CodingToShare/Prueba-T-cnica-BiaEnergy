// Modular ECharts: only the pieces the readings chart uses, so the chart
// page does not ship the whole library. Loaded lazily by ReadingsChart.
import { LineChart } from "echarts/charts";
import { GridComponent, MarkAreaComponent, MarkLineComponent, TooltipComponent } from "echarts/components";
import * as echarts from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";

echarts.use([LineChart, GridComponent, TooltipComponent, MarkAreaComponent, MarkLineComponent, CanvasRenderer]);

export { echarts };
