---
name: optimize
description: Use when asked to optimize, audit, or find improvements in this repo.
---

# Optimize — LanCartographer

Run a fast, read-only improvement scan of this repository, rank what you
find, and only then propose scoped pull requests.

## Hard timebox: 4 minutes

- **~1 min — orientation.** Read the root `AGENTS.md` and skim the `README.md`
  headings. No deeper reading; the goal is a map, not mastery.
- **~2 min — scan.** Run `git log --oneline -20`, then look at the diff
  surface of the last 10–20 commits for churn hotspots and recent regressions.
  Run `gh pr list --limit 20` to avoid duplicating open work; if `gh` is not
  available or not authenticated, note that PR state is unavailable and
  continue.
- **~1 min — rank and report.** Produce the findings table below.

When the budget is exhausted, stop scanning and report with what you have.

## Read-only scan phase

The scan phase is **strictly read-only**: absolutely no file mutations, no
commits, no branch creation, no generated-output writes. Findings are a
report first; code changes happen later, through the PR process below, and
only after the user picks which findings to act on.

## Improvement lenses

Check each of these during the scan:

- Performance (hot paths, allocations, scan-loop efficiency)
- Bugfix candidates (obvious defects, edge cases, races)
- Exception/error handling (swallowed errors, missing context, panic risk)
- Security best practices and secure-by-design posture (remember: manifest
  stays `asInvoker`, no `requireAdministrator`)
- Tests and CI workflow health (coverage gaps, `bash tests/ci-gate.sh` gaps)
- UI/UX (Fyne GUI usability, CLI parity and ergonomics)
- Logging quality (signal vs. noise, structured output, no secrets logged)
- Network-scan accuracy (false positives/negatives, OUI data freshness)
- Scan sophistication / features (missing discovery methods worth adding)
- Anything that looks wrong, even if it fits no lens above

Never propose removing or ignoring `cmd/lancartographer/resource.syso` or
`internal/oui/oui_data.go` — both are intentionally checked in. Any finding
touching the CFAA authorization sentence in LICENSE/README must keep that
sentence verbatim.

## Output format

Report findings as a ranked table:

| # | Severity | Area | File | Evidence (one line) | Suggested fix |
|---|----------|------|------|---------------------|---------------|

Rank by impact: severity (high/medium/low), the improvement lens it belongs
to, the file, one line of concrete evidence, and a one-line suggested fix.

## Scoped multi-PR process

When the user approves findings to implement:

- One concern per branch, named `kimi/optimize-<slug>` (or the active tool's
  branch prefix, e.g. `codex/`, `claude/`, `grok/`).
- Each branch is cut from `origin/main`.
- Draft PRs only. Never push or merge to `main` without explicit approval.
- PR bodies must follow `.github/PULL_REQUEST_TEMPLATE.md`.
- When findings span files that overlap between branches, use a
  stacked-branch order and document that order in each PR body.
- Run the local gate (`bash tests/ci-gate.sh`) before opening each PR.
- Reference prior findings by their table number across PRs so the trail is
  auditable.

## Stop conditions

- The 4-minute scan budget is exhausted — rank and report what you have.
- No findings are worth a PR — say so plainly, report the scan summary, and
  stop. Do not manufacture findings to fill the table.
