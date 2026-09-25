import { expect, test } from "@playwright/test";

import { credentials, kpi, navigate, screenshot, signIn, watchBrowser } from "./support";

// Runs against the real stack on a freshly seeded database. The expected
// dataset outcome below is the one documented as Phase 02/03 acceptance
// evidence (docs/phases/phase-03-backend-api.md), asserted here from the UI.

test.describe.configure({ mode: "serial" });

test("invalid credentials are rejected without leaving the login page", async ({ page }) => {
  const browser = watchBrowser(page, [{ method: "POST", path: "/api/v1/auth/login", status: 401 }]);
  await page.goto("/login");
  await page.getByLabel("Username").fill(credentials.username);
  await page.getByLabel("Password").fill("definitely-not-the-password");
  await page.keyboard.press("Enter"); // keyboard submit

  // (Next.js also renders an empty role="alert" route announcer.)
  await expect(page.getByRole("alert").filter({ hasText: "Sign-in failed." })).toContainText("Invalid");
  await expect(page.getByLabel("Password")).toHaveValue("");
  await expect(page.getByLabel("Password")).toBeFocused();
  await expect(page).toHaveURL(/\/login$/);
  browser.assertClean();
});

test("golden path: login → dashboard → AI analysis → investigation → meter → logout", async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  const browser = watchBrowser(page);

  // Protected routes send an anonymous visitor to the login page.
  await page.goto("/dashboard");
  await expect(page).toHaveURL(/\/login$/);
  await screenshot(page, testInfo, "login");

  await signIn(page);

  // Keyboard: the first Tab stop is the skip link to the main content.
  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "Skip to content" })).toBeFocused();

  // Honest dashboard before the first analysis.
  await expect(kpi(page, "Meters").locator(".kpi-value")).toHaveText("12");
  await expect(kpi(page, "Total consumption").locator(".kpi-value")).toHaveText("155,250.85kWh");
  await expect(kpi(page, "AI anomalies").locator(".kpi-value")).toHaveText("—");
  await expect(kpi(page, "AI anomalies")).toContainText("Not analyzed yet");
  await expect(page.getByText("No analysis yet")).toBeVisible();
  await screenshot(page, testInfo, "dashboard-before");

  await navigate(page, "Meters");
  await expect(page.locator("table tbody tr")).toHaveCount(10);
  await expect(page.locator("table tbody tr").first()).toContainText("Not analyzed");
  await page.getByRole("link", { name: "M-109", exact: true }).click();
  await expect(page.getByText("Not analyzed yet", { exact: true })).toBeVisible();
  await navigate(page, "Dashboard");

  let analysisPosts = 0;
  page.on("request", (request) => { if (request.method() === "POST" && request.url().endsWith("/api/v1/ai/analyze")) analysisPosts++; });
  await page.getByRole("button", { name: "Run AI Analysis" }).click();
  const completed = page.getByRole("status").filter({ hasText: "Analysis completed." });
  await expect(completed).toContainText("4 anomalies detected · 2 require high-priority attention", { timeout: 60_000 });
  expect(analysisPosts).toBe(1);

  // The dashboard refreshed with the backend's results.
  await expect(kpi(page, "AI anomalies").locator(".kpi-value")).toHaveText("4");
  await expect(kpi(page, "High priority").locator(".kpi-value")).toHaveText("2");
  await expect(kpi(page, "AI confidence").locator(".kpi-value")).toHaveText("83%");
  const queue = page.getByRole("list", { name: "Findings in priority order" }).getByRole("listitem");
  await expect(queue).toHaveCount(4);
  await expect(queue.nth(0)).toContainText("M-109 · Real anomaly");
  await expect(queue.nth(1)).toContainText("M-112 · Data quality");
  await expect(queue.nth(2)).toContainText("M-104 · Explainable anomaly");
  await expect(queue.nth(3)).toContainText("M-106 · False positive");
  await expect(page.getByRole("button", { name: "Run analysis again" })).toBeEnabled();
  await screenshot(page, testInfo, "dashboard-after");

  // AI Anomalies in priority order.
  await navigate(page, "AI Anomalies");
  await expect(page.getByRole("heading", { level: 1, name: "AI Anomalies" })).toBeVisible();
  const rows = page.locator("table tbody tr");
  await expect(rows).toHaveCount(4);
  await expect(rows.nth(0)).toContainText("M-109");
  await expect(rows.nth(0)).toContainText("Investigate meter and installation");
  await screenshot(page, testInfo, "anomalies");

  // Investigation of the top finding.
  await page.getByRole("link", { name: "Investigate M-109, Real anomaly" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "M-109 · Real anomaly" })).toBeVisible();
  // Source wall-clock time, not shifted by the browser's time zone.
  await expect(page.getByText(/Episode Sep 12, 2026, 14:00/)).toBeVisible();
  await expect(page.getByRole("note")).toContainText("Unexplained deviation.");
  await expect(page.getByRole("heading", { name: "What happened" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Investigate meter and installation" })).toBeVisible();
  await expect(page.getByText("Context only — does not explain it")).toBeVisible();
  await expect(page.getByRole("img", { name: /^Consumption of M-109\./ })).toBeVisible();
  await page.getByText("Technical evidence").click();
  await expect(page.getByText("Decision rule")).toBeVisible();
  await screenshot(page, testInfo, "investigation-M-109");

  // Meters: search, then the meter detail with its reading history.
  await navigate(page, "Meters");
  await expect(page.getByText("Showing 1–10 of 12")).toBeVisible();
  await screenshot(page, testInfo, "meters");
  await page.getByLabel("Search by meter ID").fill("M-109");
  await expect(page.locator("table tbody tr")).toHaveCount(1);
  await expect(page.locator("table tbody tr")).toContainText("Critical");
  await page.getByRole("link", { name: "M-109" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "M-109" })).toBeVisible();
  await expect(kpi(page, "Readings").locator(".kpi-value")).toHaveText("336");
  await expect(page.getByRole("img", { name: /^Consumption of M-109\./ })).toBeVisible();
  await page.getByRole("group", { name: "Metric shown in the chart" }).getByRole("button", { name: "Voltage" }).click();
  await expect(page.getByRole("img", { name: /^Voltage of M-109\./ })).toBeVisible();
  await page.getByRole("group", { name: "Metric shown in the chart" }).getByRole("button", { name: "Consumption" }).click();
  await screenshot(page, testInfo, "meter-detail-M-109");

  // Logout ends the session: protected pages are no longer reachable.
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page).toHaveURL(/\/login$/);
  await page.goto("/meters/M-109");
  await expect(page).toHaveURL(/\/login$/);
  const session = await page.request.get("/api/v1/auth/session");
  expect(session.status()).toBe(401);

  browser.assertClean();
});
