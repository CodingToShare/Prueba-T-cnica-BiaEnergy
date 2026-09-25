import { vi } from "vitest";

// Spies standing in for the Next.js app router in jsdom tests.
export const router = {
  push: vi.fn(),
  replace: vi.fn(),
  refresh: vi.fn(),
  back: vi.fn(),
  forward: vi.fn(),
  prefetch: vi.fn(),
};

export const navigation = { pathname: "/dashboard" };
