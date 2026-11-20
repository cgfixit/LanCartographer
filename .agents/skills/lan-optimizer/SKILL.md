---
name: lan-optimizer
description: >
  Time-boxed read-only scan of cgfixit/LanCartographer that ends in a ranked
  findings table, then a few draft PRs only for the findings the owner picks.
  Use when asked to optimize, audit or "find improvements" in this repo, or
  to run /lan-optimizer.
---

# LanCartographer optimizer

Scan this Go LAN inventory app for small, reviewable fixes, rank them, and
stop. Code changes come later, one draft PR per chunk the owner picks.

Read `AGENTS.md` and `INVARIANTS.md` first. They win over this file.

## 1. Scan (read-only, about four minutes)

No file edits, commits, branches or generated output in this phase.

1. Skim the `README.md` headings for a map.
2. `git fetch origin main`, then `git log --oneline -20 origin/main`. Look
   at the files the last 10–20 commits touched for churn and regressions.
3. List open PRs and the open issues (#22 roadmap, #40 macOS diagnostics,
   #80 Windows acceptance) and drop anything they already cover. Use
   whatever GitHub access the session has: the GitHub MCP tools, or
   `gh pr list --repo cgfixit/LanCartographer --state open --limit 20`.
   On Windows, `gh` on PATH may be the wrong binary; use
   `"C:\Program Files\GitHub CLI\gh.exe"`. If PR state is unavailable, say
   so and continue.
4. Sweep `internal/scan`, `internal/probe`, `internal/discover`,
   `internal/servicecheck`, `internal/macdiag`, `internal/session`,
   `internal/gui`, `internal/cli`, `internal/authz`, `.github/workflows`
   and `build/`.

When the time is spent, stop scanning and report what you have.

## 2. Lenses

- Scan correctness: false positives and negatives, ordering, stale OUI data.
- Bounds: dial budget, host workers, timeouts, output caps, cancellation,
  monotonic progress.
- Authorization and egress: every path to the engine or `servicecheck`
  checks `authz.NeedsConfirmation`; no new outbound path, no telemetry.
- Error handling: swallowed errors, missing context, panic risk, races.
- Performance: hot paths and allocations, backed by a measurement.
- Windows and macOS parity, and GUI and CLI parity.
- UI: Fyne thread rules, theme reaction, wording.
- Logging: no inventory, session JSON, prompts or secrets.
- Tests and CI: stale scripts in `build/`, gaps a lint would catch, workflow
  drift.
- Anything that is plainly wrong, even if it fits no lens.

New features are not optimizer findings. A missing discovery method or a
new view belongs in the roadmap issue (#22), so mention it there and leave
it out of the table.

## 3. Report

Return only findings you can point at in the tree, ranked by impact:

| # | Severity | Lens | File | Evidence (one line) | Suggested fix | Effort |
|---|----------|------|------|---------------------|---------------|--------|

Severity is high, medium or low. Effort is small or medium; anything larger
is an issue, not a chunk. Zero findings is a valid result: say so and stop.
Do not pad the table.

## 4. Chunks

After the owner picks findings, group them into one to four chunks. One
solid chunk beats four thin ones. A chunk is one concern.

- Cut each branch from fresh `origin/main` with the session's prefix
  (`claude/…`, `codex/…`, `grok/…`).
- If two chunks edit the same file, put them in one PR, or cut the later
  branch from the earlier one, set the PR base to that branch, and write
  the merge order in both PR bodies. Trial-merge before opening.
- Quote the finding number from the table in each PR so the trail is
  auditable.

## 5. Pull requests

Open a draft only for a chunk the owner asked for. Never push `main`, never
force-push, never merge.

- The body follows `.github/PULL_REQUEST_TEMPLATE.md`. Test evidence lists
  commands you actually ran, and what you did not run.
- Verify by running the changed behavior (`.agents/skills/lan-verify/SKILL.md`),
  then `gofmt -l .`, `go vet` and a focused test for the packages touched.
- Run `bash build/ci-gate.sh` only when the change is on the gate path and
  hosted CI would not catch it sooner. A docs-only chunk needs no race
  suite.
- On PowerShell, quote any argument containing `#`.

## Do not

- Bind a listener on `0.0.0.0`, or add telemetry, a crash reporter, an
  update check or a runtime download.
- Change the Windows manifest away from `asInvoker`.
- Weaken `authz.NeedsConfirmation` or reword the CFAA sentence.
- Delete, gitignore or accidentally change
  `cmd/lancartographer/resource.syso` or `internal/oui/oui_data.go`.
- Raise the dial cap, the 64 host workers, or the HTTP one-second and
  32 KiB limits to "go faster" without a measurement and a PR callout.
- Scan a real LAN. Use loopback fixtures and a disposable `APPDATA`.
- Add a workflow secret, a new license or a CodeQL workflow file.
- Post a comment that summons a bot reviewer.
