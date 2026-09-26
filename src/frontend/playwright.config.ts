import { defineConfig } from "@playwright/test";

// Real-stack E2E. Run through `pnpm test:e2e` (e2e/run-e2e.mjs), which starts
// a disposable database, the Go API and a production Next.js build, then sets
// E2E_BASE_URL and the test-only credentials.
const baseURL = process.env.E2E_BASE_URL;
if (!baseURL) {
  throw new Error("E2E_BASE_URL is not set: run the suite with `pnpm test:e2e`.");
}

export default defineConfig({
  testDir: "./e2e",
  outputDir: "./test-results",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  forbidOnly: true,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: [["list"]],
  use: {
    baseURL,
    browserName: "chromium",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    locale: "en-US",
    timezoneId: "America/Bogota",
  },
  projects: [
    // The golden path runs first on a fresh database: it performs the first
    // analysis that the other projects then read.
    { name: "desktop", testMatch: /golden-path\.spec\.ts/, use: { viewport: { width: 1440, height: 900 } } },
    { name: "audit", testMatch: /audit\.spec\.ts/, dependencies: ["desktop"], use: { viewport: { width: 1440, height: 900 } } },
    {
      name: "a11y",
      testMatch: /accessibility\.spec\.ts/,
      dependencies: ["desktop"],
      use: { viewport: { width: 1440, height: 900 } },
    },
    {
      name: "mobile",
      testMatch: /responsive\.spec\.ts/,
      dependencies: ["desktop"],
      use: { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 2 },
    },
    {
      name: "tablet",
      testMatch: /responsive\.spec\.ts/,
      dependencies: ["desktop"],
      use: { viewport: { width: 768, height: 1024 }, hasTouch: true },
    },
  ],
});
