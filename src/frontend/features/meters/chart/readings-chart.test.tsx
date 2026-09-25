import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ReadingsChart } from "./readings-chart";

const mocks = vi.hoisted(() => ({
  init: vi.fn(), setOption: vi.fn(), dispose: vi.fn(), resize: vi.fn(),
  observe: vi.fn(), disconnect: vi.fn(),
}));
vi.mock("./echarts-core", () => ({ echarts: { getInstanceByDom: () => undefined, init: mocks.init } }));

describe("ReadingsChart lifecycle", () => {
  it("updates an existing instance, resizes it, and releases both chart and observer on unmount", async () => {
    let resize: () => void = () => undefined;
    vi.stubGlobal("ResizeObserver", class { constructor(callback: () => void) { resize = callback; } observe = mocks.observe; disconnect = mocks.disconnect; });
    mocks.init.mockReturnValue({ setOption: mocks.setOption, dispose: mocks.dispose, resize: mocks.resize });
    const readings = [{ timestamp: "2030-01-01T14:00:00", consumption_kwh: 0, voltage_v: 220, current_a: 1, power_factor: 0.9, source_status: "OK" }];
    const { rerender, unmount } = render(<ReadingsChart readings={readings} metric="consumption_kwh" title="Test" />);
    await waitFor(() => expect(mocks.setOption).toHaveBeenCalledTimes(1));
    expect(mocks.init).toHaveBeenCalledTimes(1);
    expect(screen.getByText("0 kWh")).toBeInTheDocument();
    rerender(<ReadingsChart readings={readings} metric="voltage_v" title="Test" />);
    await waitFor(() => expect(mocks.setOption).toHaveBeenCalledTimes(2));
    expect(mocks.init).toHaveBeenCalledTimes(1);
    resize();
    expect(mocks.resize).toHaveBeenCalledTimes(1);
    unmount();
    expect(mocks.dispose).toHaveBeenCalledTimes(1);
    expect(mocks.disconnect).toHaveBeenCalledTimes(1);
  });
});
