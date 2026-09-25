import { expect, type Page, type TestInfo } from "@playwright/test";

export const credentials = {
  username: process.env.E2E_USERNAME ?? "",
  password: process.env.E2E_PASSWORD ?? "",
};

/**
 * Records browser console errors, uncaught page errors, failed requests and
 * unexpected HTTP errors. `allow` lists patterns that are expected in a test
 * (for example the 401 of an invalid login).
 */
export function watchBrowser(page: Page, expected: Array<{ method: string; path: string; status: number }> = []) {
  const problems: string[] = [];
  page.on("console", (msg) => {
    const expectedResourceError = expected.some((e) =>
      msg.location().url.endsWith(e.path) && msg.text().startsWith(`Failed to load resource: the server responded with a status of ${e.status} (`));
    if ((msg.type() === "error" || msg.type() === "warning") && !expectedResourceError) {
      problems.push(`console.${msg.type()}: ${msg.text()}`);
    }
  });
  page.on("pageerror", (error) => problems.push(`pageerror: ${error.message}`));
  page.on("requestfailed", (request) => {
    const failure = request.failure()?.errorText ?? "";
    // Navigations cancel in-flight RSC prefetches; that is not an error.
    if (failure.includes("ERR_ABORTED") && request.method() === "GET" &&
      (request.url().includes("_rsc=") || new URL(request.url()).pathname.startsWith("/api/v1/"))) {
      return;
    }
    const text = `requestfailed: ${request.method()} ${request.url()} ${failure}`;
    problems.push(text);
  });
  page.on("response", (response) => {
    const text = `http ${response.status()}: ${response.request().method()} ${response.url()}`;
    const allowed = expected.some((e) => e.method === response.request().method() && e.status === response.status() && e.path === new URL(response.url()).pathname);
    if (response.status() >= 400 && !allowed) {
      problems.push(text);
    }
  });
  return {
    problems,
    assertClean: () => expect(problems, "browser console / network problems").toEqual([]),
  };
}

export async function signIn(page: Page) {
  await page.goto("/login");
  await page.getByLabel("Username").fill(credentials.username);
  await page.getByLabel("Password").fill(credentials.password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);
}

/** The shell switches to a top bar with a menu sheet at 900 px and below. */
export function isNarrow(page: Page) {
  return (page.viewportSize()?.width ?? 1440) <= 900;
}

/** Opens the main navigation on narrow layouts (sheet) and follows a link. */
export async function navigate(page: Page, label: string) {
  if (isNarrow(page)) {
    await page.getByRole("button", { name: "Open menu" }).click();
    await page.getByRole("dialog").getByRole("link", { name: label }).click();
    await expect(page.getByRole("dialog")).toBeHidden();
  } else {
    await page.getByRole("navigation", { name: "Main navigation" }).getByRole("link", { name: label }).click();
  }
}

export function kpi(page: Page, label: string) {
  return page.locator(".kpi").filter({ has: page.locator(".kpi-label", { hasText: label }) });
}

/** The page must never scroll horizontally. */
export async function expectNoHorizontalOverflow(page: Page) {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow, "horizontal overflow in px").toBeLessThanOrEqual(0);
}

export async function screenshot(page: Page, testInfo: TestInfo, name: string) {
  await page.screenshot({ path: testInfo.outputPath(`${testInfo.project.name}-${name}.png`), fullPage: true });
}
