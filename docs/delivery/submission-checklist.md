# Submission Checklist

Run through this immediately before submitting. Every item has a command or a place to look. Evidence of the last full run is in [phase-06-quality-delivery.md](../phases/phase-06-quality-delivery.md).

## Repository

- [ ] `git status --short` shows no unintended changes. Only intended work is committed, and nothing is pushed or tagged without the owner's decision.
- [ ] Required files are present: `README.md`, `compose.yaml`, `.env.example`, `src/backend/Dockerfile`, `src/frontend/Dockerfile`, `.github/workflows/ci.yml`, `docs/api/openapi.yaml`, `docs/demo/demo-guide.md`.
- [ ] No `.env`, credentials, private keys, cookies, database dumps, model files, coverage or Playwright output is tracked: `git ls-files | grep -iE '(^|/)\.env$|\.pem$|\.key$|cookies?\.txt$|\.dump$|\.sql\.gz$|\.gguf$|test-results/|coverage/'` prints nothing.
- [ ] Source data is unchanged: `git hash-object data/input/readings.csv data/input/events.csv` prints `1c1a11a7aef21a75e4649d8e71f52747b1cf8816` and `d6a23acd4b6fbb0600f4fa835fe46bbd9944e426`.
- [ ] No evaluator file: `git ls-files | grep -i expected_results` prints nothing.
- [ ] No dataset special-casing in production code: `git grep -nE 'M-10[46]|M-109|M-112' -- src ':!*_test.go' ':!*.test.ts*' ':!src/frontend/e2e' ':!src/frontend/lib/api/schema.d.ts'` prints nothing.

## Demo

- [ ] Clean start works: `docker compose down -v --remove-orphans`, then `docker compose up --build -d --wait` exits 0, and `docker compose ps -a` shows `migrate` and `seed` exited (0) and `postgres`, `api`, `web` healthy.
- [ ] At http://localhost:3000 the `demo` / `bia-demo-2026` login works, the dashboard shows 12 meters and "Not analyzed yet", and Run AI Analysis gives 4 findings and 2 high priority, in the order M-109, M-112, M-104, M-106.
- [ ] The M-109 investigation shows the evidence, the "Evidence-based explanation" and the recommended action.
- [ ] `curl -s http://localhost:8080/metrics | grep bia_analysis_runs_total` shows the completed run.
- [ ] Reset works: after `docker compose down -v --remove-orphans` and a new start, the dashboard is back to "Not analyzed yet".
- [ ] The optional Ollama mode is documented (README) and is **not** required.

## Quality Gates

- [ ] Backend: `go -C src/backend mod verify`, `go -C src/backend vet ./...`, `go -C src/backend test ./...`, `go -C src/backend test -tags=integration ./...`.
- [ ] Frontend (in `src/frontend`): `pnpm install --frozen-lockfile`, `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`, `pnpm test:e2e`.
- [ ] Generated code is synchronized: the pinned sqlc `diff` is clean; after `pnpm generate:api`, `git diff --exit-code src/frontend/lib/api/schema.d.ts` passes.
- [ ] Images build from scratch: `docker compose build --no-cache`.
- [ ] CI configuration is present. It is only reported as passing once a real GitHub Actions run exists.

## Documentation

- [ ] README quick start, credentials, expected result, reset, optional Ollama and known limitations match the current behavior.
- [ ] Phase records, roadmap and traceability reflect the final evidence; known limitations are listed.
