import "@testing-library/jest-dom/vitest";

import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

import { navigation, router } from "./navigation";

vi.mock("next/navigation", async () => {
  const nav = await import("./navigation");
  return {
    useRouter: () => nav.router,
    usePathname: () => nav.navigation.pathname,
    useSearchParams: () => new URLSearchParams(),
  };
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  for (const fn of Object.values(router)) {
    fn.mockReset();
  }
  navigation.pathname = "/dashboard";
});
