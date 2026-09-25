import { chromium, expect } from "@playwright/test";
import { join } from "node:path";

/**
 * Graceful degradation on the real stack: the API is restarted with Ollama
 * configured at an address where nothing listens. A new analysis must still
 * complete with the same findings, and the investigation must show the
 * deterministic fallback without any error state. No model is needed.
 */
export async function verifyExplanationFallback({ baseUrl, username, password, frontendDir, restartAPIWithOllama }) {
  await restartAPIWithOllama();
  const browser = await chromium.launch();
  try {
    const page = await browser.newPage({ baseURL: baseUrl, viewport: { width: 1440, height: 900 } });
    const problems = [];
    page.on("pageerror", (e) => problems.push(`pageerror: ${e.message}`));
    page.on("console", (msg) => {
      if (msg.type() === "error" || msg.type() === "warning") problems.push(`console.${msg.type()}: ${msg.text()}`);
    });
    page.on("response", (response) => {
      if (response.status() >= 400) problems.push(`http ${response.status()}: ${new URL(response.url()).pathname}`);
    });

    await page.goto("/login");
    await page.getByLabel("Username").fill(username);
    await page.getByLabel("Password").fill(password);
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/dashboard$/);

    await page.getByRole("button", { name: "Run analysis again" }).click();
    await expect(page.getByRole("status").filter({ hasText: "Analysis completed." }))
      .toContainText("4 anomalies detected · 2 require high-priority attention", { timeout: 90_000 });

    const queue = page.getByRole("list", { name: "Findings in priority order" }).getByRole("listitem");
    await expect(queue).toHaveCount(4);
    for (const [i, title] of ["M-109 · Real anomaly", "M-112 · Data quality", "M-104 · Explainable anomaly", "M-106 · False positive"].entries()) {
      await expect(queue.nth(i)).toContainText(title);
    }

    await queue.nth(0).getByRole("link").click();
    await expect(page.getByRole("heading", { level: 1, name: "M-109 · Real anomaly" })).toBeVisible();
    const explanation = page.getByRole("region", { name: "Explanation" });
    await expect(explanation.getByText("Evidence-based fallback")).toBeVisible();
    await expect(explanation).toContainText("no recorded event explains the change");
    await expect(explanation.getByRole("alert")).toHaveCount(0);
    await explanation.getByText("About this explanation").click();
    await expect(explanation).toContainText("Fallback usedYes");
    await expect(page.getByRole("heading", { name: "Investigate meter and installation" })).toBeVisible();
    await page.screenshot({ path: join(frontendDir, "test-results", "explanation-fallback-desktop.png"), fullPage: true });

    expect(problems).toEqual([]);
  } finally {
    await browser.close();
  }
}
