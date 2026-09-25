import { expect, test } from "@playwright/test";

import { credentials, expectNoHorizontalOverflow, navigate, screenshot, signIn, watchBrowser } from "./support";

test("twenty session cycles never report an intentional logout as expiry", async ({ page }) => {
  test.setTimeout(120_000);
  const browser = watchBrowser(page);
  for (const route of ["/dashboard", "/meters", "/meters/M-109", "/anomalies", "/anomalies/1"]) {
    await page.goto(route);
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.locator(".kpi")).toHaveCount(0);
  }
  for (let i = 0; i < 20; i++) {
    await signIn(page);
    await expect(page.getByRole("list", { name: "Findings in priority order" }).getByRole("listitem")).toHaveCount(4);
    await page.getByRole("button", { name: "Sign out" }).click();
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByText("Your session has ended.", { exact: false })).toHaveCount(0);
  }
  browser.assertClean();
});

test("tampered cookie redirects once and wrong username remains a login error", async ({ page, context, baseURL }) => {
  const browser = watchBrowser(page, [
    { method: "GET", path: "/api/v1/auth/session", status: 401 },
    { method: "GET", path: "/api/v1/dashboard/summary", status: 401 },
    { method: "POST", path: "/api/v1/auth/login", status: 401 },
  ]);
  await context.addCookies([{ name: "bia_session", value: "invalid-audit-session", url: baseURL!, httpOnly: true }]);
  await page.goto("/dashboard");
  await expect(page).toHaveURL(/\/login\?expired=1$/);
  await expect(page.getByRole("status")).toContainText("Your session has ended");
  await page.getByLabel("Username").fill("unknown-operator");
  await page.getByLabel("Password").fill(credentials.password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("alert").filter({ hasText: "Sign-in failed" })).toBeVisible();
  await expect(page.getByLabel("Password")).toBeFocused();
  await expect(page).toHaveURL(/\/login\?expired=1$/);
  browser.assertClean();
});

test("real SQL filters, both sort directions and pagination agree with the displayed rows", async ({ page }) => {
  const browser = watchBrowser(page);
  await signIn(page);
  await navigate(page, "Meters");
  const rows = page.locator("table tbody tr");
  await expect(rows).toHaveCount(10);
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await expect(page.getByText("Showing 11–12 of 12")).toBeVisible();
  await expect(rows).toHaveCount(2);
  await page.getByRole("button", { name: "Previous", exact: true }).click();
  for (const [search, count] of [["M-109", 1], ["m-10", 9], ["M-999", 0], ["%", 0], ["_", 0], ["測定", 0]] as const) {
    await page.getByLabel("Search by meter ID").fill(search);
    await expect(rows).toHaveCount(count);
    if (count === 0) await expect(page.getByRole("heading", { name: "No meters match" })).toBeVisible();
  }
  await page.getByRole("button", { name: "Clear filters" }).click();
  for (const [label, count] of [["OK", 9], ["Alert", 2], ["Critical", 1], ["All", 10]] as const) {
    await page.getByRole("group", { name: "Filter by computed status" }).getByRole("button", { name: label, exact: true }).click();
    await expect(rows).toHaveCount(count);
  }
  for (const [label, column] of [["Consumption", "consumption"], ["Variation", "variation"], ["Severity", "severity"], ["Meter", "meter_id"]] as const) {
    for (const order of column === "meter_id" ? ["asc", "desc"] : ["desc", "asc"]) {
      await page.getByRole("columnheader").getByRole("button", { name: new RegExp(`^${label}`) }).click();
      await expect(page.getByRole("columnheader").filter({ has: page.getByRole("button", { name: new RegExp(`^${label}`) }) })).toHaveAttribute("aria-sort", order === "asc" ? "ascending" : "descending");
      const response = await page.request.get(`/api/v1/meters?sort=${column}&order=${order}&limit=10&offset=0`);
      expect(response.status()).toBe(200);
      const expected = (await response.json()).items.map((m: { meter_id: string }) => m.meter_id);
      await expect(rows.locator("td:first-child")).toHaveText(expected);
    }
  }
  await navigate(page, "AI Anomalies");
  for (const [label, count] of [["High", 2], ["Medium", 1], ["Low", 1], ["All", 4]] as const) {
    await page.getByRole("group", { name: "Filter by severity" }).getByRole("button", { name: label, exact: true }).click();
    await expect(rows).toHaveCount(count);
  }
  for (const type of ["REAL_ANOMALY", "DATA_QUALITY", "EXPLAINABLE_ANOMALY", "FALSE_POSITIVE"]) {
    await page.getByLabel("Type", { exact: true }).selectOption(type);
    await expect(rows).toHaveCount(1);
  }
  await page.getByLabel("Type", { exact: true }).selectOption("ALL");
  await page.getByLabel("Meter ID", { exact: true }).fill("M-106");
  await expect(rows).toHaveCount(1);
  await expect(rows).toContainText("False positive");
  await page.getByLabel("Meter ID", { exact: true }).fill("M-101");
  await expect(page.getByRole("heading", { name: "No anomalies match" })).toBeVisible();
  browser.assertClean();
});

test("all 336 chart samples and metric values preserve source time in opposite time zones", async ({ browser, baseURL }) => {
  test.setTimeout(90_000);
  for (const timezoneId of ["Pacific/Kiritimati", "America/Los_Angeles"]) {
    const context = await browser.newContext({ baseURL, timezoneId, viewport: { width: 1440, height: 900 } });
    try {
      const page = await context.newPage();
      const audit = watchBrowser(page);
      await signIn(page);
      await page.goto("/meters/M-109");
      await expect(page.getByRole("img", { name: /^Consumption of M-109/ })).toBeVisible();
      await expect(page.getByText("Baseline energy (flagged hours)")).toBeVisible();
      const readings = (await (await page.request.get("/api/v1/meters/M-109/readings?limit=1000")).json()).items;
      expect(readings).toHaveLength(336);
      await page.getByText("View data as table", { exact: true }).click();
      const chart = page.getByRole("img", { name: /of M-109/ });
      await expect(chart).toHaveAttribute("_echarts_instance_", /.+/);
      const instance = await chart.getAttribute("_echarts_instance_");
      for (const [label, key, min, max, unit] of [
        ["Consumption", "consumption_kwh", 0, 2, " kWh"], ["Voltage", "voltage_v", 1, 1, " V"],
        ["Current", "current_a", 1, 1, " A"], ["Power factor", "power_factor", 3, 3, ""],
      ] as const) {
        await page.getByRole("group", { name: "Metric shown in the chart" }).getByRole("button", { name: label, exact: true }).click();
        await expect(page.locator("figure th").last()).toHaveText(label);
        const actual = await page.locator("figure tbody tr").evaluateAll((rows) => rows.map((row) => Array.from(row.querySelectorAll("td"), (cell) => cell.textContent)));
        const expected = readings.map((r: Record<string, string | number>) => {
          const t = String(r.timestamp);
          const day = new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric", timeZone: "UTC" }).format(new Date(`${t}Z`));
          const value = new Intl.NumberFormat("en-US", { minimumFractionDigits: min, maximumFractionDigits: max }).format(Number(r[key]));
          return [`${day}, ${t.slice(11, 16)}`, `${value}${unit}`];
        });
        expect(actual).toEqual(expected);
        expect(await chart.getAttribute("_echarts_instance_")).toBe(instance);
      }
      await page.getByRole("link", { name: "Open investigation" }).click();
      await expect(page.getByText(/Episode Sep 12, 2026, 14:00/)).toBeVisible();
      await expect(page.getByText("Technical evidence").locator("..")).not.toHaveAttribute("open", "");
      await expect(page.getByText("Confidence 93% · High")).toBeVisible();
      await expect(page.getByText(/not a probability of equipment failure/)).toBeVisible();
      audit.assertClean();
    } finally { await context.close(); }
  }
});

test("normal meter, missing resources and default investigations remain useful", async ({ page }, testInfo) => {
  const browser = watchBrowser(page, [
    { method: "GET", path: "/api/v1/meters/M-UNKNOWN", status: 404 },
    { method: "GET", path: "/api/v1/anomalies/999999", status: 404 },
  ]);
  await signIn(page);
  await page.goto("/meters/M-101");
  await expect(page.getByRole("heading", { name: "No current finding" })).toBeVisible();
  await expect(page.getByRole("img", { name: /^Consumption of M-101/ })).toBeVisible();
  await screenshot(page, testInfo, "normal-meter");
  for (const [url, title, back] of [["/meters/M-UNKNOWN", "Meter not found", "All meters"], ["/anomalies/999999", "Anomaly not found", "All anomalies"]]) {
    await page.goto(url);
    await expect(page.getByRole("heading", { name: title })).toBeVisible();
    await expect(page.getByRole("link", { name: back })).toBeVisible();
    await screenshot(page, testInfo, title.replaceAll(" ", "-"));
  }
  await page.goto("/anomalies");
  for (const meter of ["M-109", "M-112", "M-104", "M-106"]) {
    await page.getByRole("link", { name: new RegExp(`^Investigate ${meter},`) }).click();
    await expect(page.getByRole("img", { name: new RegExp(`^Consumption of ${meter}`) })).toBeVisible();
    await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
    await expect(page.getByText("Technical evidence").locator("..")).not.toHaveAttribute("open", "");
    await screenshot(page, testInfo, `investigation-${meter}-collapsed`);
    await page.getByRole("link", { name: "All anomalies" }).click();
  }
  const font = await page.evaluate(async () => { await document.fonts.ready; return { family: getComputedStyle(document.body).fontFamily, loaded: [...document.fonts].filter((f) => f.status === "loaded").map((f) => f.family) }; });
  expect(font.family).toContain("Jakarta");
  expect(font.loaded.some((family) => family.includes("Jakarta"))).toBe(true);
  browser.assertClean();
});

test("synthetic long text and large values remain readable at 390 px", async ({ page }, testInfo) => {
  // Layout-only stress probe: alter rendered text, never API responses or the
  // dataset. Functional acceptance above always uses the untouched real API.
  const browser = watchBrowser(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await signIn(page);
  await page.goto("/meters");
  await expect(page.locator("table tbody tr")).toHaveCount(10);
  await page.locator("table tbody tr").first().evaluate((row) => {
    row.querySelector("a")!.textContent = `M-${"X".repeat(62)}`;
    row.querySelector('td[data-label="Consumption"]')!.textContent = "999,999,999,999,999.99 kWh";
  });
  await expectNoHorizontalOverflow(page);
  expect(await page.locator(".tbl-wrap").evaluate((e) => e.scrollWidth - e.clientWidth)).toBeLessThanOrEqual(1);
  await screenshot(page, testInfo, "synthetic-long-meter");
  await page.goto("/anomalies");
  await page.getByRole("link", { name: /^Investigate M-109,/ }).click();
  await expect(page.getByRole("img", { name: /^Consumption of M-109/ })).toBeVisible();
  await page.evaluate(() => {
    document.querySelector("h1")!.textContent = `M-${"X".repeat(62)} · Real anomaly`;
    document.querySelector(".story-block p")!.textContent = "Long event description and supporting evidence. ".repeat(40);
    document.querySelector("#action-title")!.textContent = "Review the very long recommended operational action and its recorded supporting evidence";
    document.querySelector('a[href="/meters/M-109"]')!.textContent = `View meter M-${"X".repeat(62)}`;
    // Explanation fields at their backend maximum (320 / 700 characters),
    // including one unbroken token, as a verbose model could produce.
    const explanation = document.querySelector('section[aria-labelledby="explanation-title"]')!;
    const paragraphs = explanation.querySelectorAll("p");
    paragraphs[0].textContent = ("Summary ".repeat(35) + "X".repeat(40)).slice(0, 320);
    paragraphs[1].textContent = ("Why it matters in detail. ".repeat(26) + "Y".repeat(60)).slice(0, 700);
    paragraphs[2].textContent = ("Evidence narrative sentence. ".repeat(24) + "Z".repeat(60)).slice(0, 700);
    document.querySelector('section[aria-labelledby="action-title"] p')!.textContent = "Recommended action wording. ".repeat(25).slice(0, 700);
  });
  await expect(page.getByRole("heading", { name: "Review the very long recommended operational action and its recorded supporting evidence" })).toBeVisible();
  await expectNoHorizontalOverflow(page);
  expect(await page.locator(".panel").evaluateAll((panels) => Math.max(...panels.map((p) => p.scrollWidth - p.clientWidth)))).toBeLessThanOrEqual(1);
  await screenshot(page, testInfo, "synthetic-long-investigation");
  browser.assertClean();
});
