# Development Environment Readiness

- Date: 2026-09-24
- Result: **READY** (the optional items from the first pass were completed the same day: GPU driver, standalone agent CLIs, Docker memory limit)
- Scope: machine-level prerequisites only. No application bootstrap, manifests, migrations, compose files, or product code were created. Phase 01 was not started.

## Platform

| Item | Value |
| --- | --- |
| OS | Windows 11 Pro for Workstations, build 26200, x64 |
| Shells | Windows PowerShell 5.1; Git Bash (bundled with Git) |
| Privileges | Non-elevated session; user can elevate through UAC |
| Virtualization | Hypervisor present; Virtual Machine Platform and WSL enabled |
| WSL | WSL 2.6.1, kernel 6.6.87.2, default version 2; only the `docker-desktop` distribution (no Linux distribution installed or needed) |
| Docker backend | Docker Desktop on WSL2, Linux containers |
| PowerShell execution policy | CurrentUser = RemoteSigned (see Changes) |

## Required Toolchain

| Tool | Target | Detected | Status | Notes |
| --- | --- | --- | --- | --- |
| Git | latest stable | 2.54.0 | OK | Global identity configured |
| Go | 1.27.1 (≥ 1.27.1, < 1.28) | 1.27.1 windows/amd64 | OK | Official archive, SHA-256 verified; user-space install under `%LOCALAPPDATA%\Programs\Go`; `GOPATH=%USERPROFILE%\go`; `GOBIN` on user PATH |
| Node.js | 24.21.0 LTS | 24.21.0 | OK | Official MSI, SHA-256 verified; replaced system Node 20.18 (EOL) |
| npm | bundled with Node | 11.19.0 | OK | Ships with Node 24.21.0 |
| pnpm | 12.6.0 | 12.6.0 | OK | `npm install -g pnpm@12.6.0` in the user prefix; npm's default install-script blocking kept (pnpm runs via its Node placeholder) |
| Docker Desktop | current stable | 4.91.0 | OK | Upgraded from 4.52.0; installer hash verified by winget |
| Docker Engine | current stable | 29.8.0 | OK | Linux containers on WSL2; 12 CPUs; VM memory limited to 6 GB via `.wslconfig` (5.79 GiB usable) |
| Docker Compose | v2+ (`docker compose`) | 5.5.1 | OK | Bundled with Docker Desktop |
| PostgreSQL image | `postgres:18.6-alpine` | 18.6 | OK | Pulled; disposable smoke test passed |
| GitHub CLI | recommended | 2.95.0 | OK | Authenticated (repo and workflow scopes) |
| Ollama | 0.34.x stable (optional) | 0.34.4 | OK, GPU (CUDA) | Signed official 0.34.3 installer, SHA-256 verified. Ollama's built-in auto-updater moved it to 0.34.4 (same-minor stable patch) during the reboot. Uses the GTX 1050 through CUDA |
| Next.js | 16.3.x Active LTS (project dependency) | not installed | Recorded only | 16.3.6 is the current `latest`; installed and pinned locally when frontend bootstrap is authorized |

Version policy: runtime majors are fixed (Go 1.27.x, Node 24.x LTS, pnpm 12.x, PostgreSQL 18.x, Next.js 16.3.x). Application libraries are installed only through project manifests and lockfiles. No prereleases were installed.

Not installed globally (by design): next, react, typescript, tailwind, shadcn, TanStack Query, echarts, playwright, vitest, testify, pgx, chi, sqlc, goose. sqlc and goose versions are pinned in Phase 01.

## Changes Performed On This Machine

1. Docker Desktop 4.52.0 → 4.91.0 (winget, UAC approved by the user).
2. Go 1.27.1 installed in user space; `Go\bin` and `%USERPROFILE%\go\bin` appended to the user PATH.
3. System Node.js 20.18.0 → 24.21.0 through the official MSI (UAC approved by the user). fnm also holds an unused Node 24.21.0 copy that is not on PATH.
4. pnpm 12.6.0 installed in the npm user prefix.
5. PowerShell execution policy set to RemoteSigned for the current user (user decision). Node 24 ships `npm.ps1`/`npx.ps1`, and the pnpm shim adds `pnpm.ps1`; under the previous Restricted default these failed in PowerShell.
6. Ollama 0.34.3 installed per user. The installer registers a Windows startup entry (vendor default).
7. `postgres:18.6-alpine` and `hello-world` images pulled.
8. NVIDIA driver 431.84 (2019, OEM) → 582.66 (official WHQL DCH notebook "Security Update Driver", June 2026), display driver component only. Pascal GPUs receive security-branch drivers only. Package signature (NVIDIA Corporation) verified and exact hardware ID `DEV_1C92&SUBSYS_126D1462` confirmed in its INF. Silent install with UAC approved by the user; the machine was rebooted afterwards.
9. Claude Code CLI 2.1.281 installed with Anthropic's official native installer (per-user, auto-updating, SHA-256 verified by the installer, binary signed by Anthropic, PBC).
10. Codex CLI 0.156.1 installed with OpenAI's official standalone installer (per-user, SHA-256 verified by the installer, binary signed by OpenAI), which added its bin folder to the user PATH.
11. `%USERPROFILE%.wslconfig` created with `[wsl2] memory=6GB`, following Microsoft's documented keys. WSL and Docker restarted to apply it (the default was 50% of RAM, 7.68 GiB).

Open terminals and VS Code must be restarted to pick up PATH changes.

## Validation Performed

Final run from a fresh shell context (PATH from the registry, default execution policy):

| Command | Result |
| --- | --- |
| `git --version` | git version 2.54.0.windows.1 |
| `go version` | go1.27.1 windows/amd64 |
| `go env GOOS GOARCH GOPROXY GOSUMDB` | windows, amd64, `https://proxy.golang.org,direct`, `sum.golang.org` |
| `node --version` / `npm --version` / `pnpm --version` | v24.21.0 / 11.19.0 / 12.6.0 |
| `npm`, `npx`, `pnpm` under the default PowerShell policy | Run correctly (failed before the policy change) |
| `docker version` | Client and Engine 29.8.0, Docker Desktop 4.91.0 |
| `docker compose version` | 5.5.1 |
| `docker info` | Docker Desktop, linux, 12 CPUs, 5.79 GiB after the `.wslconfig` limit (7.68 GiB before) |
| `docker run --rm hello-world` | "Hello from Docker!" (before and after the upgrade) |
| Disposable Compose project (`config`, `up`, `down`) outside the repository | Valid config; service exited 0; no leftover containers or networks |
| PostgreSQL smoke: `docker run --rm postgres:18.6-alpine` with a random one-time password, no published port | `pg_isready` accepting connections in about 4 s; `SHOW server_version` → 18.6; `SELECT version()` → PostgreSQL 18.6 on x86_64-pc-linux-musl; `SELECT 1` → 1; container stopped and removed; 0 dangling volumes. Run twice, the second time on the upgraded engine |
| `gh --version` / `gh auth status` | 2.95.0; logged in |
| `ollama --version` / `GET http://127.0.0.1:11434/api/version` | 0.34.4 / 0.34.4 (0.34.3 at install; auto-updated) |
| Ollama server log after the driver update | `inference compute ... library=CUDA compute=6.1 name=CUDA0 description="NVIDIA GeForce GTX 1050" ... total="4.0 GiB"` |
| `nvidia-smi` | GeForce GTX 1050, driver 582.66, 4096 MiB |
| `claude --version` / `codex --version` | 2.1.281 (Claude Code) / codex-cli 0.156.1; Authenticode signatures valid |
| Post-reboot re-validation | All commands above re-run after the reboot and the `.wslconfig` change: hello-world OK; PostgreSQL 18.6 ready, `SELECT 1` → 1, no leftovers |
| PATH resolution (`Get-Command`) | A single executable per tool; no shadowing |
| Registry and TLS reachability (certificate validation on) | proxy.golang.org, sum.golang.org, registry.npmjs.org, github.com, api.github.com, ollama.com → 200; Docker registry → 401 auth challenge (reachable). No proxy in use |
| Repository `git status` | 57 untracked governance files; no commits; no manifests, code, migrations, compose files, or CSVs |

## Hardware For Local AI

| Item | Value |
| --- | --- |
| CPU | Intel Core i7-8750H, 6 cores / 12 threads |
| RAM | 15.8 GiB total; the Docker/WSL VM is capped at 6 GB, leaving about 10 GB for Windows, the IDE and Ollama |
| GPU | NVIDIA GeForce GTX 1050 (Pascal, compute 6.1), 4 GiB VRAM, driver 582.66; Intel UHD 630 integrated |
| Ollama compute | CUDA on the GTX 1050 (4.0 GiB total, about 3.3 GiB available); CPU fallback for layers that do not fit |
| Free disk | About 620 GiB on C:, about 1.8 TiB on D: |

No model was downloaded. Model choice is deferred to Phase 05 (OD-16). Constraints: 4 GiB VRAM favors small quantized models that fit on the GPU, and generation latency must stay within the explanation timeout. Ollama auto-updates by default; disable it in Ollama's settings if the version must stay fixed during the demo.

## Port Check

| Port | Intended use | Status |
| --- | --- | --- |
| 3000 | Frontend candidate | Free |
| 8080 | Backend candidate | Free |
| 5432 | PostgreSQL | Free on the host (the smoke test published no port) |
| 11434 | Ollama | Used by Ollama on 127.0.0.1 after installation (expected) |

## Development Agent Tooling

| Tool | Status |
| --- | --- |
| Claude Code | VS Code extension and standalone CLI 2.1.281 (`%USERPROFILE%.localin`). Sign-in is interactive: run `claude` once if the CLI asks |
| Codex | VS Code extension and standalone CLI 0.156.1. Sign-in is interactive: run `codex` and choose "Sign in with ChatGPT" |
| GitHub Copilot | No separate Copilot extension detected in the extension list; check in VS Code if needed |
| GitHub CLI | Installed and authenticated |

## Blockers

None.

Notes:

- Windows Update still reports a pending reboot for updates installed on 2026-09-22. It is unrelated to this toolchain and not a blocker.
- Sign-in for the standalone Claude Code and Codex CLIs, if needed, is done interactively by the user (USER_AUTH_REQUIRED only for those CLIs).
