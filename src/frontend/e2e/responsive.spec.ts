import { expect, test } from "@playwright/test";

import { expectNoHorizontalOverflow, navigate, screenshot, signIn, watchBrowser } from "./support";

// Narrow layouts (mobile 390 px, tablet 768 px) on the analysed database left
// by the desktop golden path. Checks navigation, stacked tables, charts and
// the absence of horizontal scrolling on every screen.

test("narrow layouts keep the full journey usable", async ({ page }, testInfo) => {
  test.setTimeout(90_000);
  const browser = watchBrowser(page);

  await page.goto("/login");
  await expectNoHorizontalOverflow(page);
  await screenshot(page, testInfo, "login");
  await signIn(page);

  // Below 901 px the sidebar becomes a top bar with a menu sheet.
  await expect(page.getByRole("button", { name: "Open menu" })).toBeVisible();
  const menu = page.getByRole("button", { name: "Open menu" });
  await menu.focus();
  await page.keyboard.press("Enter");
  // Modal navigation correctly hides the background trigger from the accessibility tree.
  await expect(page.locator('button[aria-label="Open menu"]')).toHaveAttribute("aria-expanded", "true");
  await page.keyboard.press("Tab");
  expect(await page.getByRole("dialog").evaluate((el) => el.contains(document.activeElement))).toBe(true);
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toBeHidden();
  await expect(menu).toBeFocused();
  expect(await page.evaluate(() => document.body.hasAttribute("data-scroll-locked"))).toBe(false);
  await expect(page.getByRole("list", { name: "Findings in priority order" }).getByRole("listitem")).toHaveCount(4);
  await expectNoHorizontalOverflow(page);
  await screenshot(page, testInfo, "dashboard");

  await navigate(page, "Meters");
  await expect(page.getByText("Showing 1–10 of 12")).toBeVisible();
  if (testInfo.project.name === "mobile") {
    await page.getByLabel("Sort by", { exact: true }).selectOption("consumption");
    await expect(page.getByRole("button", { name: "Sort direction: descending" })).toBeVisible();
    await page.getByRole("button", { name: "Sort direction: descending" }).click();
    await expect(page.getByRole("button", { name: "Sort direction: ascending" })).toBeVisible();
  }
  await expectNoHorizontalOverflow(page);
  await screenshot(page, testInfo, "meters");

  await navigate(page, "AI Anomalies");
  await expect(page.locator("table tbody tr")).toHaveCount(4);
  await expectNoHorizontalOverflow(page);
  await screenshot(page, testInfo, "anomalies");

  for (const [meter, heading, callout] of [
    ["M-112", "M-112 · Data quality", "Measurement inconsistency."],
    ["M-106", "M-106 · False positive", "Detected, explained and recovered."],
    ["M-104", "M-104 · Explainable anomaly", "Explained change."],
    ["M-109", "M-109 · Real anomaly", "Unexplained deviation."],
  ] as const) {
    await page.getByRole("link", { name: new RegExp(`^Investigate ${meter},`) }).click();
    await expect(page.getByRole("heading", { level: 1, name: heading })).toBeVisible();
    await expect(page.getByRole("note")).toContainText(callout);
    await expect(page.getByRole("heading", { name: "What happened" })).toBeVisible();
    await expect(page.getByText("What to do")).toBeVisible();
    await expect(page.getByRole("img", { name: new RegExp(`^Consumption of ${meter}\\.`) })).toBeVisible();
    await expectNoHorizontalOverflow(page);
    if (meter === "M-109" || meter === "M-112") {
      await screenshot(page, testInfo, `investigation-${meter}`);
    }
    await page.getByRole("link", { name: "All anomalies" }).click();
    await expect(page.locator("table tbody tr")).toHaveCount(4);
  }

  await page.goto("/meters/M-109");
  await expect(page.getByRole("heading", { level: 1, name: "M-109" })).toBeVisible();
  await expect(page.getByRole("img", { name: /^Consumption of M-109\./ })).toBeVisible();
  await expectNoHorizontalOverflow(page);
  await screenshot(page, testInfo, "meter-detail-M-109");

  await page.getByRole("button", { name: "Open menu" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Sign out" }).click();
  await expect(page).toHaveURL(/\/login$/);

  browser.assertClean();
});
