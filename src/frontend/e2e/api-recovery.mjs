import { chromium, expect } from "@playwright/test";
import { join } from "node:path";

/** An actual stopped API, not a fulfilled/intercepted browser request. */
export async function verifyAPIRecovery({ baseUrl, username, password, frontendDir, stopAPI, restartAPI }) {
  const browser = await chromium.launch();
  try {
    const page = await browser.newPage({ baseURL: baseUrl, viewport: { width: 1440, height: 900 } });
    const errors = [];
    const unexpected = [];
    const outageResponses = [];
    let outage = false;
    page.on("pageerror", (e) => errors.push(e.message));
    page.on("response", (response) => {
      if (response.status() < 400) return;
      const path = new URL(response.url()).pathname;
      if (outage && ["/api/v1/auth/session", "/api/v1/dashboard/summary"].includes(path) && [500, 502, 503, 504].includes(response.status())) {
        outageResponses.push(response.status());
      } else unexpected.push(`${response.status()} ${path}`);
    });
    await page.goto("/login");
    await page.getByLabel("Username").fill(username);
    await page.getByLabel("Password").fill(password);
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/dashboard$/);
    await expect(page.locator(".kpi-value")).toHaveCount(6);
    const values = await page.locator(".kpi-value").allTextContents();
    outage = true;
    await stopAPI();
    await page.reload();
    await expect(page.getByRole("heading", { name: "The dashboard could not be loaded" })).toBeVisible({ timeout: 20_000 });
    // Next's connection-refused proxy response is a non-JSON 500. The API
    // client must hide that raw response behind its safe generic message.
    await expect(page.getByText("The service returned an unexpected response. Please try again.")).toBeVisible();
    await page.screenshot({ path: join(frontendDir, "test-results", "api-down-desktop.png"), fullPage: true });
    await restartAPI();
    outage = false;
    await page.getByRole("button", { name: "Retry", exact: true }).click();
    await expect(page.locator(".kpi-value")).toHaveText(values);
    await page.getByRole("button", { name: "Retry session" }).click();
    await expect(page.getByText(username, { exact: true })).toBeVisible();
    await page.screenshot({ path: join(frontendDir, "test-results", "api-recovered-desktop.png"), fullPage: true });
    expect(errors).toEqual([]);
    expect(unexpected).toEqual([]);
    expect(outageResponses.length).toBeGreaterThan(0);
  } finally { await browser.close(); }
}
