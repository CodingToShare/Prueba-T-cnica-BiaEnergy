// Real-stack end-to-end run: disposable PostgreSQL 18.6 container → Go
// migrate + seed (challenge CSVs) → Go API → production Next.js build →
// Playwright. Nothing is mocked. Everything started here is torn down at the
// end, including on failure or Ctrl+C.
//
// Requirements: Docker, Go (on PATH or via GO=path/to/go), dependencies
// installed with `pnpm install` and Chromium via `pnpm exec playwright install chromium`.
// Usage: pnpm test:e2e [playwright args…]

import { spawn, spawnSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import { closeSync, mkdtempSync, openSync, readFileSync, rmSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { basename, dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const frontendDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repoRoot = resolve(frontendDir, "..", "..");
const backendDir = join(repoRoot, "src", "backend");
const isWindows = process.platform === "win32";
const exe = isWindows ? ".exe" : "";
const go = process.env.GO ?? "go";
const POSTGRES_IMAGE = "postgres:18.6-alpine";

const workDir = mkdtempSync(join(tmpdir(), "bia-e2e-"));
const container = `bia-e2e-${randomBytes(4).toString("hex")}`;
const children = [];
let cleanupPromise;

function log(message) {
  console.log(`[e2e] ${message}`);
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { stdio: "inherit", ...options });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error(`${command} exited with ${result.status}`);
  }
  return result;
}

function freePort() {
  return new Promise((resolvePort, reject) => {
    const server = createServer();
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address();
      server.close(() => resolvePort(port));
    });
  });
}

function start(name, command, args, options) {
  const logFile = join(workDir, `${name}.log`);
  const out = openSync(logFile, "w");
  const child = spawn(command, args, { windowsHide: true, detached: !isWindows, stdio: ["ignore", out, out], ...options });
  closeSync(out);
  child.logFile = logFile;
  child.label = name;
  children.push(child);
  return child;
}

async function stop(child) {
  if (child.exitCode !== null || child.pid === undefined) {
    return;
  }
  await new Promise((resolveExit) => {
    const timer = setTimeout(() => {
      if (!isWindows) process.kill(-child.pid, "SIGKILL");
    }, 5000);
    child.once("close", () => { clearTimeout(timer); resolveExit(); });
    if (isWindows) {
      spawnSync("taskkill", ["/pid", String(child.pid), "/T", "/F"], { stdio: "ignore", windowsHide: true });
    } else {
      process.kill(-child.pid, "SIGTERM");
    }
  });
}

function cleanup() {
  cleanupPromise ??= (async () => {
    for (const child of [...children].reverse()) await stop(child);
    spawnSync("docker", ["rm", "-f", container], { stdio: "ignore", windowsHide: true });
    if (dirname(resolve(workDir)) !== resolve(tmpdir()) || !basename(workDir).startsWith("bia-e2e-")) {
      throw new Error("Refusing cleanup outside the disposable test directory");
    }
    rmSync(workDir, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
  })();
  return cleanupPromise;
}

process.on("SIGINT", async () => {
  await cleanup();
  process.exit(130);
});
process.on("SIGTERM", async () => { await cleanup(); process.exit(143); });

/** Polls `check` until it succeeds (condition-based, never a fixed sleep). */
async function waitFor(what, check, timeoutMs, child) {
  const deadline = Date.now() + timeoutMs;
  let lastError;
  while (Date.now() < deadline) {
    if (child && child.exitCode !== null) {
      throw new Error(`${what}: process exited early (${child.exitCode})\n${readFileSync(child.logFile, "utf8")}`);
    }
    try {
      if (await check()) {
        return;
      }
    } catch (error) {
      lastError = error;
    }
    await new Promise((r) => setTimeout(r, 250));
  }
  throw new Error(`Timed out waiting for ${what}${lastError ? `: ${lastError.message}` : ""}`);
}

async function httpOk(url) {
  const response = await fetch(url, { redirect: "manual", signal: AbortSignal.timeout(2000) });
  return response.status === 200;
}

async function main() {
  const [dbPort, apiPort, webPort] = [await freePort(), await freePort(), await freePort()];
  const dbPassword = randomBytes(12).toString("hex");
  const username = "e2e-operator";
  const password = randomBytes(12).toString("hex");
  const databaseUrl = `postgres://bia:${dbPassword}@127.0.0.1:${dbPort}/bia_e2e?sslmode=disable`;
  const backendEnv = { ...process.env, DATABASE_URL: databaseUrl, LOG_LEVEL: "warn" };

  log(`starting disposable ${POSTGRES_IMAGE} (${container})`);
  run("docker", [
    "run", "-d", "--rm", "--name", container,
    "-e", "POSTGRES_USER=bia", "-e", `POSTGRES_PASSWORD=${dbPassword}`, "-e", "POSTGRES_DB=bia_e2e",
    "-p", `127.0.0.1:${dbPort}:5432`, POSTGRES_IMAGE,
  ], { stdio: ["ignore", "ignore", "inherit"] });

  log("building Go commands");
  for (const cmd of ["migrate", "seed", "api"]) {
    run(go, ["build", "-o", join(workDir, `${cmd}${exe}`), `./cmd/${cmd}`], { cwd: backendDir });
  }

  log("migrating (waits for PostgreSQL to accept connections)");
  await waitFor(
    "database migrations",
    () => spawnSync(join(workDir, `migrate${exe}`), ["up"], { cwd: repoRoot, env: backendEnv, stdio: "ignore" }).status === 0,
    90_000,
  );
  log("seeding the challenge dataset");
  run(join(workDir, `seed${exe}`), [], { cwd: repoRoot, env: backendEnv });

  log("starting the API");
  const apiOptions = {
    cwd: repoRoot,
    env: {
      ...backendEnv,
      HTTP_ADDR: `127.0.0.1:${apiPort}`,
      DEMO_AUTH_USERNAME: username,
      DEMO_AUTH_PASSWORD: password,
      SESSION_SIGNING_KEY: randomBytes(32).toString("hex"),
      SESSION_COOKIE_SECURE: "false",
    },
  };
  const apiProcess = start("api", join(workDir, `api${exe}`), [], apiOptions);
  await waitFor("API readiness", () => httpOk(`http://127.0.0.1:${apiPort}/readyz`), 30_000, apiProcess);

  const nextBin = join(frontendDir, "node_modules", "next", "dist", "bin", "next");
  const webEnv = { ...process.env, BACKEND_URL: `http://127.0.0.1:${apiPort}`, NEXT_TELEMETRY_DISABLED: "1" };
  log("building the frontend (rewrites point at the disposable API)");
  run(process.execPath, [nextBin, "build"], { cwd: frontendDir, env: webEnv });

  log("starting the frontend");
  const web = start("web", process.execPath, [nextBin, "start", "-H", "127.0.0.1", "-p", String(webPort)], {
    cwd: frontendDir,
    env: webEnv,
  });
  const baseUrl = `http://127.0.0.1:${webPort}`;
  await waitFor("frontend", () => httpOk(`${baseUrl}/login`), 60_000, web);

  log(`running Playwright against ${baseUrl}`);
  const playwrightEnv = { ...process.env, E2E_BASE_URL: baseUrl, E2E_USERNAME: username, E2E_PASSWORD: password };
  delete playwrightEnv.NO_COLOR; // Playwright sets FORCE_COLOR for its workers.
  const playwright = start("playwright",
    process.execPath,
    [join(frontendDir, "node_modules", "@playwright", "test", "cli.js"), "test", ...process.argv.slice(2)],
    {
      cwd: frontendDir,
      stdio: "inherit",
      env: playwrightEnv,
    },
  );
  const playwrightStatus = await new Promise((resolveExit, reject) => {
    playwright.once("error", reject);
    playwright.once("exit", (status) => resolveExit(status ?? 1));
  });
  if (playwrightStatus !== 0) {
    for (const child of children) {
      console.log(`\n[e2e] --- ${child.label} log ---\n${readFileSync(child.logFile, "utf8").slice(-4000)}`);
    }
  }
  if (playwrightStatus === 0) {
    const { verifyAPIRecovery } = await import("./api-recovery.mjs");
    await verifyAPIRecovery({ baseUrl, username, password, frontendDir,
      stopAPI: () => stop(apiProcess),
      restartAPI: async () => {
        const restarted = start("api-restarted", join(workDir, `api${exe}`), [], apiOptions);
        await waitFor("restarted API", () => httpOk(`http://127.0.0.1:${apiPort}/readyz`), 30_000, restarted);
      },
    });
    log("real API outage → safe dashboard error → restart → retry: passed");
  }
  return playwrightStatus;
}

let code = 1;
try {
  code = await main();
} catch (error) {
  console.error(`[e2e] ${error instanceof Error ? error.message : String(error)}`);
} finally {
  await cleanup();
}
process.exit(code);
