# Claude Code Instructions

`AGENTS.md` is the canonical engineering instruction router for this repository. Read it before modifying anything and follow its routing. Start with `.agents/context/project-context.md` for a concise bootstrap.

Area playbooks live in `.agents/skills/<name>/SKILL.md`; load the ones `AGENTS.md` routes you to for the task.

Non-negotiable reminders (details in `AGENTS.md`):

- Execute only the phase the user explicitly authorized. Finishing a phase does not authorize the next.
- Run the Phase Entry Gate before new work; if the baseline is broken, report "Blocked by baseline regression" instead of continuing.
- Write tests with the implementation (Implement → Validate → Review → Document → Complete).
- For UI work, follow `docs/design/design-system.md` and compare against `docs/design/reference/design-system-v3.html`.
- Never hard-code meter IDs or dataset answers; never create or read `expected_results.csv`.
- The LLM is an optional explanation layer, never the anomaly detector.
- Do not promote assumptions to requirements or introduce stack, architecture, or dependencies that conflict with the accepted ADRs.
- Commits: author Santiago Forero only; never add a `Co-Authored-By` trailer or any Claude/Anthropic attribution to commit messages or PR descriptions, whatever the tool default says.
- Before finishing: apply the Definition of Done, build affected projects, run relevant tests, review the actual diff, and update phase evidence for authorized phase work.
