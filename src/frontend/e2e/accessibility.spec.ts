import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page, type TestInfo } from "@playwright/test";
import { writeFileSync } from "node:fs";

import { credentials, signIn, watchBrowser } from "./support";

// Automated axe scan of the product's representative pages on the real
// stack (runs after the golden path, so the analysis exists). Policy:
// critical and serious WCAG 2.1 A/AA violations fail the test; moderate and
// minor ones are written to the test output (axe-<page>.json). This
// complements, and does not replace, the manual keyboard and screen-structure
// review; it is not a WCAG certification.

const TAGS = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"];

async function scan(page: Page, testInfo: TestInfo, name: string) {
  const results = await new AxeBuilder({ page }).withTags(TAGS).analyze();
  expect(results.passes.length, `axe evaluated rules on ${name}`).toBeGreaterThan(10);
  const blocking = results.violations.filter((v) => v.impact === "critical" || v.impact === "serious");
  const other = results.violations.filter((v) => !blocking.includes(v));
  // Every scan leaves a summary next to the test output, even when empty.
  const summary = testInfo.outputPath(`axe-${name}.json`);
  writeFileSync(summary, JSON.stringify(other.map((v) => ({ id: v.id, impact: v.impact, nodes: v.nodes.length })), null, 2));
  await testInfo.attach(`axe-${name}`, { path: summary, contentType: "application/json" });
  expect(
    blocking.map((v) => `${v.id} (${v.impact}): ${v.nodes.map((n) => n.target.join(" ")).slice(0, 3).join(" | ")}`),
    `serious/critical axe violations on ${name}`,
  ).toEqual([]);
}

test("representative pages have no serious or critical axe violations", async ({ page }, testInfo) => {
  test.setTimeout(90_000);
  const browser = watchBrowser(page);

  await page.goto("/login");
  await expect(page.getByRole("heading", { level: 1, name: "Sign in" })).toBeVisible();
  await scan(page, testInfo, "login");

  await signIn(page);
  await expect(page.getByRole("list", { name: "Findings in priority order" }).getByRole("listitem")).toHaveCount(4);
  await scan(page, testInfo, "dashboard");

  await page.goto("/meters");
  await expect(page.locator("table tbody tr")).toHaveCount(10);
  await scan(page, testInfo, "meters");

  await page.goto("/meters/M-109");
  await expect(page.getByRole("img", { name: /^Consumption of M-109\./ })).toBeVisible();
  await scan(page, testInfo, "meter-detail");

  await page.goto("/anomalies");
  await expect(page.locator("table tbody tr")).toHaveCount(4);
  await scan(page, testInfo, "anomalies");

  await page.getByRole("link", { name: /^Investigate M-109,/ }).click();
  await expect(page.getByRole("region", { name: "Explanation" })).toBeVisible();
  await scan(page, testInfo, "investigation");

  // Representative narrow layout: the shell switches to the menu sheet.
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/dashboard");
  await expect(page.getByRole("button", { name: "Open menu" })).toBeVisible();
  await scan(page, testInfo, "dashboard-mobile");
  await page.goto("/anomalies");
  await page.getByRole("link", { name: /^Investigate M-109,/ }).click();
  await expect(page.getByRole("region", { name: "Explanation" })).toBeVisible();
  await scan(page, testInfo, "investigation-mobile");

  expect(credentials.username).not.toBe("");
  browser.assertClean();
});
