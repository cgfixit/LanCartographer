---
name: lan-optimize
description: Methodically scan LanCartographer's main branch for scan-correctness, safety-limit, Windows/macOS parity, CI-cost, and maintainability opportunities, then open a small set of focused draft pull requests only when each chunk earns its keep (not a fixed PR quota). Use when asked to optimize LanCartographer, audit the scanner, harden or speed up its GitHub workflows, or open optimization PRs against main. Not auto-invocable — it opens PRs, so it runs only when explicitly asked.
disable-model-invocation: true
---

# lan-optimize

**Persona:** you are a Go engineer who knows this codebase: a Fyne v2 GUI and
a stdlib-flag CLI sharing one scan engine (`internal/scan`), a 64-worker host
pool behind an independent 128/256 dial budget (`probe.Limiter`), TCP + HTTP
title probes under hard limits, NetBIOS > mDNS > PTR name resolution, and
platform code split by build tag across Windows and macOS. You read for
leverage: scan accuracy, cancellation and event ordering, the authorization
gate, platform parity, CI cost, and code that ships but should not.

**What it does:** a time-boxed read-only scan of `main`, findings grouped into
**a small set of PR-sized chunks** (about 1–4 when several independent fixes
are real; **a guideline, not a quota**), and one focused draft PR per kept
chunk, on a branch cut from the topology base Step 3.5 assigns. Nothing lands
on `main` from this skill; the owner merges.

`AGENTS.md` is the contract and wins on conflict. The agent-agnostic
siblings are `.agents/skills/optimize/` (short scan) and
`.agents/skills/lan-optimizer/` (scan + PR rules for other tools); this file
is the Claude Code driver and must not contradict them.

---

## Run

### Step 0 — Bootstrap (harness)

```bash
bash .claude/skills/lan-optimize/bootstrap.sh "claude/lan-optimize-<topic>"
```

It reports the git identity, fetches `origin/main`, cuts the named branch from
`origin/main` (an existing branch is checked out, never reset), and prints the
scan seed: package LOC, platform-file parity, unpinned `uses:` lines and
triggers per workflow, `todo.txt`, recent commits, and hot files. Omit the
argument to stay read-only on the current branch.

### Step 1 — Time-boxed scan (subagent, ~4 minutes)

Dispatch **one read-only `Explore` subagent**. Paste it the bootstrap output
and this prompt (adapt the focus, keep the structure):

> You are a Go engineer who knows LanCartographer (Fyne GUI + flag CLI over
> one engine in `internal/scan`). Repo root is the CWD. Spend ~4 minutes on a
> READ-ONLY scan for concrete fixes sized for small focused PRs. Sweep:
> `internal/scan/engine.go` (event order: live port/share events, one
> `EventHostFound` per alive host, monotonic `EventProgress`, `EventDone`;
> cancellation; the `deps` seam); `internal/probe` (HTTP 1 s / 32 KiB / no
> redirects / verified TLS; `Limiter` use on every dial; mDNS and NetBIOS
> parsing); `internal/discover` (liveness port set, ARP fallback, subnet
> math); `internal/winutil` (`_windows`/`_darwin`/`_other` parity, argv-only
> subprocesses, `*_parse.go` coverage); `internal/gui` (widget updates only
> via `fyne.Do`, theme applied before widgets); `internal/cli` and
> `internal/authz` (both front ends gate before `scan.Run`);
> `internal/appdata`, `favorites`, `hostcache` (temp-file + rename, 0700
> dirs, no cwd fallback); `.github/workflows/*.yml` (runner minutes —
> macOS bills ~10× Linux on this private repo — caching, concurrency,
> timeouts, pinning, least privilege; **nothing that needs a new secret or
> license**); `tests/` and `deploy/build.sh`. Return 6–10 DISTINCT findings,
> each: title, file:line, one-line evidence, lens, effort (small/medium).
> End with a grouping into PR-sized chunks — only as many as earn a PR.
> Cite real code; do not invent.

The 4 minutes bound the sweep only; keep reading to confirm each finding
before it becomes a PR.

### Step 2 — Dedup against open PRs (MCP), before choosing

```text
mcp__github__list_pull_requests(owner="cgfixit", repo="LanCartographer", state="open",
                                fields=["number","title","head","draft"])
```

Pass `fields` so the payload stays small. Drop anything an open PR covers.
Also drop: anything on the `todo.txt` list re-proposed as a feature, a new
discovery protocol, a 3D view, or the v1.1 map.

### Step 3 — Select and announce

Keep only chunks that clearly earn a PR. **One solid PR beats four thin
ones; zero is correct when nothing survives dedup.** A chunk is one concern
(`AGENTS.md` §4): 1–2 major changes or 3–5 minor ones of the same kind. In
one line per chunk, say what it touches and the leverage.

### Step 3.5 — Branch topology for shared files

Build a file → chunks map. The usual shared files here: `.github/workflows/ci.yml`,
`release.yml`, `README.md`, `AGENTS.md`, `CLAUDE.md`, `deploy/build.sh`,
`internal/scan/engine.go`. For each file in two or more chunks, pick one:

- **(A) Consolidate** — all edits to that file go in one chunk.
- **(B) Stack** — cut the later branch from the earlier branch and set its
  PR base to that branch, not `main`. Parent merges first.

Trial-merge every pair that shares a file before opening either:

```bash
git checkout -B _trial origin/main
git merge --no-ff "<branch-A>" && git merge --no-ff "<branch-B>"
grep -rc '<<<<<<<' <shared-files>          # must be 0
actionlint .github/workflows/*.yml          # when a workflow is shared
git checkout - && git branch -D _trial
```

Plan so the owner can merge this run's drafts **lowest PR number → highest**,
parents before children.

### Step 4 — One draft PR per chunk

| Topology | Cut from | PR base |
|---|---|---|
| Independent | `origin/main` | `main` |
| Stacked | parent chunk's branch | parent branch until it merges |
| Consolidated | the branch that owns the shared edits | that branch's base |

1. Make the change. Minimal diff, on topic. Keep line endings (compare
   `git diff --numstat` with `--ignore-cr-at-eol`; `.gitattributes` governs).
2. Verify (below).
3. Commit with an imperative sentence subject in this repo's style
   ("Pin workflow actions to commit SHAs"), body explaining why, then
   `git push -u origin "<branch>"` (retry network failures 4× at 2/4/8/16 s).
4. Open the draft with `mcp__github__create_pull_request(..., draft=true)`.
   The body mirrors `.github/PULL_REQUEST_TEMPLATE.md` section for section:
   Summary, Type of change, Motivation / value, Test evidence, Security
   impact (answer all four questions), UI changes, Checklist. State the
   topology and merge order in Summary. Test evidence lists only commands
   you ran; name what you did not run (for example "Windows gate and macOS
   race: hosted CI") rather than ticking boxes for it.
5. `subscribe_pr_activity` on it and drive it to green.

If nothing survives Steps 2–3, say so briefly and stop.

---

## Verify (per chunk, before pushing)

Run the changed behavior first, then lint, then let hosted CI cover the
platforms (`AGENTS.md` §4):

```bash
gofmt -l .                                     # empty
CGO_ENABLED=0 go vet ./...                     # any OS; GUI files drop out by tag
CGO_ENABLED=0 go test ./internal/<pkg>/ -run <Test> -count=1   # focused, when a test covers it
bash .claude/skills/lan-guard/check.sh         # invariants + actionlint
```

For engine, probe, or CLI changes, build the CLI and run it against an owned
loopback fixture with a disposable data dir:

```bash
CGO_ENABLED=0 go build -o "${TMPDIR:-/tmp}/lc-cli" ./cmd/lancartographer
python3 -m http.server 18080 --bind 127.0.0.1 &
APPDATA="$(mktemp -d)" "${TMPDIR:-/tmp}/lc-cli" --range 127.0.0.1/32 --i-am-authorized --ports 18080
```

For a workflow change, also run the shell bodies you changed locally where
they are not runner-specific. Docs- or workflow-only chunks skip Go tests.
A Linux pass is not Windows or macOS evidence; GUI changes need native
interaction (`AGENTS.md` §4) and say so in the PR if not done.

---

## Branch permissions (pre-granted when this skill is invoked)

Invoking `lan-optimize` authorizes creating and pushing per-chunk branches
named `claude/lan-optimize-<topic>`, cut from the Step 3.5 base, and opening
them as drafts. Do not stop to ask before pushing those. It does not
authorize pushing `main`, force-pushing, merging, marking a PR ready, or
changing repository settings or visibility.

---

## Guardrails (`AGENTS.md` invariants — never an optimization target)

- Manifest stays `asInvoker`; the CFAA sentence stays verbatim; `authz`
  stays fail-closed and both front ends keep gating before `scan.Run`.
- `resource.syso` and `oui_data.go` stay tracked; `go.sum` stays committed.
- HTTP 1 s / 32 KiB / no redirects / verified TLS, the 64-worker and 256-dial
  caps, cancellation, and monotonic progress do not move without a
  measurement in the PR.
- No telemetry, crash reporter, runtime download, all-interfaces listener,
  new workflow secret, or new license.
- CI never scans a real LAN; fixtures are loopback.

---

## Gotchas

- **`resource.syso` breaks every macOS link.** It is a Windows COFF object
  and Apple's `ld` rejects it. `deploy/build.sh` moves it aside and restores
  it; a manual macOS `go build ./...` must do the same. Never delete it,
  ignore it, or commit regenerated bytes by accident.
- **The drift check excludes `resource.syso`** because its bytes depend on
  the goversioninfo build. A finding that it "drifts" is expected noise.
- **macOS CI minutes cost ~10× Linux** on this private repo, and the release
  workflow builds on a twice-daily schedule. Any workflow change that adds a
  macOS job or removes a skip guard needs a cost line in the PR.
- **Major action bumps are Dependabot's job.** `.github/dependabot.yml`
  groups minor/patch action updates; SHA pins keep a `# vX.Y.Z` comment so
  it can bump them. A hand-rolled major bump (checkout v4 → v7,
  download-artifact v4 → v8) is its own PR with changelog evidence.
- **Cross-compiles do not verify Fyne.** The `!cgo` stub in
  `internal/gui/run_nocgo.go` makes CGO-off builds succeed without
  compiling the GUI.
- **`gh` on Windows** may resolve to the wrong binary; the `.agents` skill
  uses the full `C:\Program Files\GitHub CLI\gh.exe` path. Cloud sessions have
  no `gh` at all — use the GitHub MCP tools.
- **Largest file ≠ worst file.** `internal/gui/gui.go` and `internal/probe`
  lead the LOC table; confirm a defect before proposing a refactor.
- **CodeQL.** `AGENTS.md` §6 says the CodeQL workflow was removed, but
  GitHub's default code-scanning setup can still run outside
  `.github/workflows/`. Check the Actions run list before claiming either.

---

## Example output (shape only)

The Step 1 subagent returns findings as *title · file:line · evidence · lens
· effort*, then a grouping. One past grouping: "workflow hardening and cost"
(SHA pins, `persist-credentials: false`, timeouts, PR concurrency, a
skip-if-unchanged guard on the scheduled release build) as one CI chunk, and
the `todo.txt` dedup items as one cleanup chunk. Count is not a quota.
