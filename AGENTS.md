# AGENTS.md — LanCartographer operating contract

Cross-agent contract for any AI coding agent (Claude Code, Codex, Copilot,
Kimi, Grok, or anything else) working in this repo. Read it fully before your
first edit. `CLAUDE.md` imports this file; `.agents/README.md` explains the
shared skill library. When this file and a tool-specific file disagree, this
file wins unless the owner says otherwise in the session.

Repo: `github.com/cgfixit/LanCartographer`, canonical remote `origin`, default
branch `main`. MIT licensed. The **Go module path is
`github.com/local/lancartographer`**, so import paths do not match the GitHub
URL. Use the module path. Repository visibility and billing are the owner's
decisions alone. Agents never change either.

## 0. Owner working rules

- **Verify by running it.** Exercise the changed behavior directly (CLI
  against an owned loopback fixture, the native app, browser or computer use
  for UI) before relying on tests. Add focused tests for behavior changes;
  do not build or run whole suites to prove a change.
- **Keep tests and CI current.** When a change makes anything in `tests/` or
  `.github/workflows/` stale, update or remove it in the same PR, then run
  the changed script and `actionlint`.
- **Prefer lint to tests for mechanical mistakes.** Baseline lint is `gofmt`
  and `go vet`; there is no golangci config yet. Propose a new lint check in
  its own PR and add its command to §4. Never bundle a lint rollout with
  feature work.
- **Docs freshness.** Before opening a PR that changes user-visible behavior,
  CLI flags, build steps or workflows, or when `README.md` and `docs/` have
  not changed in 7+ days, review `README.md`, `docs/USER-GUIDE.md` and
  `docs/README.md` for stale or duplicate content and fix it in that PR.
  When an edit adds 50+ words to a file, cut 5–15% of stale or repeated
  content elsewhere in that file. Exempt: dated records in
  `docs/verification/`, the CFAA sentence, and `INVARIANTS.md`. Check that
  relative links still resolve after moving or renaming a doc.

## 1. Product and hard invariants

LanCartographer is an **original** Windows 10/11 and macOS 13+ LAN and port
inventory tool. It is built with Go and Fyne (versions in `go.mod`), and a
stdlib-`flag` CLI shares the same scan engine. It is authorization-gated and
has zero telemetry. It is inspired by Advanced IP Scanner (Famatech), Angry IP
Scanner and Nmap, and must credit them without implying any affiliation.

The CFAA sentence governs anything touching scan or probe behavior. Keep it
verbatim wherever it appears: `LICENSE`, `SECURITY.md`, `CONTRIBUTING.md`,
`docs/USER-GUIDE.md`, `docs/network-workbench-roadmap.md`, and `authz.Notice`
in `internal/authz/authz.go` (a two-part Go string concatenation; the joined
text must match exactly), which `authz_test.go` also checks:

> Scan only networks you own or have explicit authorization to inventory. Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws.

`INVARIANTS.md` holds the full security invariants (gate thresholds, external
actions, egress). Read it before touching scan, probe, `authz`, `servicecheck`
or launch code.

Never violate these, even if a request or a skill seems to ask for it:

- The Windows manifest stays `asInvoker`, never `requireAdministrator`. macOS
  runs unelevated. An empty ARP cache is not evidence that root is needed.
- `authz.NeedsConfirmation` stays fail-closed. All three callers check it
  before contacting anything: `internal/gui/gui.go` (scan),
  `internal/cli/cli.go` (scan; `--i-am-authorized`) and
  `internal/gui/service_check.go` (Check Service). Any new caller that reaches
  the engine or `servicecheck` must apply the same policy.
- No telemetry, crash reporter, runtime download, update check, or listener on
  `0.0.0.0`. Justify any new outbound network path in the PR.
- Probe limits are measured contracts, not tuning knobs:
  - HTTP probe: 1 s timeout, 32 KiB body cap, TLS verification on, no redirects
    (`internal/probe/http.go`).
  - Global dial budget: default 128, hard cap 256 (`internal/probe/limiter.go`).
  - Host workers: 64 hosts in flight (`maxHostsInFlight`, `internal/scan/engine.go`).
  - Check Service: resolve 5 s, connect 3 s, TLS 3 s, HTTP 1 s, at most 16
    addresses. No proxy, no redirects, no re-resolution after the operator picks
    an address (`internal/servicecheck`).
  - Cancellation and monotonic progress events are part of the contract.
  - Change any of these only with a measurement and an explicit PR callout.
- `cmd/lancartographer/resource.syso` and `internal/oui/oui_data.go` are
  generated **and intentionally tracked**. Never delete them, gitignore them,
  or commit an accidental change to them. `go.sum` is committed.
- Subprocesses are argv-only and never shell strings. Their arguments are built
  in platform files.
- Never log or publish real LAN inventory, session JSON, prompts, or
  credentials.

## 2. Layout

- `cmd/lancartographer/`: the single entry point (`main.go`) and the
  `go:generate` directives (`generate.go` → `tools/icongen`, which writes
  `app.ico`; `generate_windows.go` → `goversioninfo`, which writes
  `resource.syso` from `versioninfo.json` + `app.manifest` + `app.ico`).
- `internal/`, one package per concern:
  - `scan`: engine, worker pool, ordered events, cancellation, and the
    enrichment/cache lifecycle.
  - `probe`: bounded TCP/HTTP/ICMP/mDNS/name/MAC/share probes and the dial
    limiter.
  - `discover`: liveness and subnet enumeration.
  - `winutil`: platform boundaries for adapters, ARP, elevation and SMB shares
    (`_windows`, `_darwin`, `_other` files).
  - `servicecheck`: single-endpoint resolve → connect → TLS → HTTP diagnosis.
    No UI and no global state.
  - `authz`, `model`, `cli`, `wol`, `favorites`, `export`, `hostcache`,
    `appdata`, `session`.
  - `oui`: `data.csv` → `go generate ./internal/oui` → `oui_data.go`.
  - `gui`: Fyne UI. Feature files include `service_check.go`,
    `internet_quality.go`, `wifi_quality*.go` and `sessions.go`, and
    `run_nocgo.go` is the non-CGO stub.
- `tools/icongen/`: icon generator.
- `deploy/build.sh`: native build into `dist/` (Windows `.exe` pair; macOS
  ad-hoc-signed `.app`, CLI, checksums and per-arch ZIP). On macOS it moves the
  COFF `resource.syso` aside and restores it on exit.
- `packaging/macos/Info.plist`: bundle identity and local-network usage text.
  The macOS minimum (13.0) lives in three places that must stay aligned:
  `LSMinimumSystemVersion`, `MACOSX_DEPLOYMENT_TARGET`, and the
  `-mmacosx-version-min` CGO flags in `deploy/build.sh`. The smoke test
  inspects the Mach-O headers. Ad-hoc signing is not notarization.
- `tests/ci-gate.sh`: the full Windows-oriented gate (gofmt, vet, generate
  drift, CGO build, race tests). It hard-codes `/c/Program Files/Go/bin` and
  `/c/msys64/ucrt64/bin`.
- `tests/macos-smoke.py`: an owned loopback fixture that exercises both
  packaged entry points.
- `docs/`: `docs/README.md` indexes everything. Dated verification records live
  in `docs/verification/<topic>-YYYY-MM-DD.md`, with images in
  `docs/verification/<topic>/`. `screenshots/` holds older README images.
- `INVARIANTS.md`: security invariants (see §1).
- User docs: `README.md`, `docs/USER-GUIDE.md` (workflows plus a code tour),
  `docs/update-guide.md`, `CONTRIBUTING.md`, `SECURITY.md`, `SUPPORT.md`.
  Links into `docs/` from the root use `docs/…`; links from inside `docs/` are
  relative to it.
- `.agents/skills/`: shared skills (`optimize`, `lan-optimizer`). There is no
  `.claude/skills/` directory, so do not reference one.
- Open work lives in GitHub issues (#22 roadmap, #24 platform acceptance,
  #25 dependency review, #26 legacy cleanup); refresh their state first.
- `packaging/note` and `cmd/-` are owner scratch files, not build inputs. Do
  not act on or delete them unless asked.

## 3. Execution and ownership

- `main.go` runs the CLI when the executable base name contains `cli`
  (case-insensitive) or when the first argument is `--cli`, which is then
  stripped. Otherwise it starts Fyne. On macOS, `lancartographer-cli` is a
  second name for the same binary.
- CLI flags: `--range` (CIDR; defaults to local physical subnets), `--ports`,
  `--timeout` (400 ms), `--concurrency` (128, clamped to 256), `--json PATH`
  (`-` sends JSON only to stdout), `--i-am-authorized`.
- Fyne widgets are touched only on the UI thread, using `fyne.Do` from workers.
  Apply the saved theme before creating widgets. Shared panel labels must react
  to theme changes.
- `session` owns bounded saved snapshots, compatibility checks (matching app
  version and probe profile) and selected plain-text reports. Full session JSON
  is unredacted. Report redaction is on by default. GUI session copies never
  replace the live inventory. Save, open, compare and export never start scans
  or probes. There is no schema migration: unsupported versions are rejected.
- `appdata.Path` uses an absolute `APPDATA` if set; otherwise it uses
  `os.UserConfigDir()` + `LanCartographer/`. A relative or missing directory is
  an error and never falls back to the working directory. Files written there:
  `settings.json`, `favorites.json`, `names.json`, `firstrun_seen`, `sessions/`.
  Writes go to a temporary file that is then renamed into place. New
  directories are created with mode 0700.

## 4. Build and verification

Read `go.mod` and check the real compiler and PATH before building. Fyne needs
CGO.

- **Windows:** MSYS2 UCRT64 gcc (`CC=gcc`) and `goversioninfo` v1.7.0.
- **macOS:** Xcode Command Line Tools and system clang. Do not use the Windows
  `CC=gcc` setting.
- **Linux:** useful only for non-GUI packages. A cross-compile does not verify
  Fyne.

```bash
bash deploy/build.sh                         # native package
python3 tests/macos-smoke.py dist            # macOS: packaged CLI + app --cli on loopback
bash tests/ci-gate.sh                        # Windows full gate; only when needed
gofmt -l . && go vet ./...                   # always for Go changes
actionlint .github/workflows/*.yml           # any workflow change (all five files)
```

- Verify per §0, then gofmt, relevant vet/build and a focused package test.
  Hosted CI covers broad and platform regressions. Record any user waiver
  separately from passing evidence.
- On macOS, a manual `go build ./...` or a test that links
  `cmd/lancartographer` requires moving `resource.syso` aside and restoring it.
  Prefer `deploy/build.sh`.
- UI changes require real native interaction, rendered before/after
  screenshots, and light and dark themes. Use owned loopback fixtures and
  disposable `APPDATA` directories. Never use real LAN data in screenshots.
- Keep evidence tiers distinct: native GUI, CLI fixture, focused tests, hosted
  CI, and downloaded-release acceptance. New verification goes in a **new**
  dated record. Never rewrite an old record as current acceptance.
- Any time a `go:generate` input changes (`data.csv`, `versioninfo.json`,
  `app.manifest`, `tools/icongen`), regenerate and commit. CI checks
  `oui_data.go` and `app.ico` for drift, but not `resource.syso`, whose bytes
  vary with the `goversioninfo` build.

## 5. Branches, PRs, and GitHub etiquette

- One concern per branch and PR. Cut branches from fresh `origin/main` and open
  them as **drafts**. Never push to `main`, never force-push, and never merge
  without an explicit request. Commit and publish only within the scope the
  user authorized.
- Branch prefix: use the one your harness or the user requires. Otherwise use
  `<agent>/<slug>` (`claude/…`, `codex/…`, `grok/…`).
- PR bodies follow `.github/PULL_REQUEST_TEMPLATE.md`, which requires test
  evidence with the exact commands, platform limits, security-impact answers,
  and screenshots for UI changes.
- Chunks touching the same file share a PR or are stacked; trial-merge first.
- **Do not post PR or issue comments that summon bot reviewers** (for example
  "@codex review") without the owner's approval.
- Do not add workflow secrets, a new license, or a CodeQL workflow file. See §7.

## 6. Skills

- To optimize, audit, or "find improvements", read
  `.agents/skills/optimize/SKILL.md` and follow it. It is a 4-minute
  read-only scan that ends in a ranked findings table, with no mutations.
- For `/lan-optimizer`, or when findings should become PRs, read
  `.agents/skills/lan-optimizer/SKILL.md`. It covers branch topology, the
  draft-PR gate, and a `gh` path note for Windows.
- Skills are read-only contracts. Change them only through a reviewed PR,
  never mid-task to make a result fit.

## 7. CI and release reference

| Workflow | Trigger | What it does |
| --- | --- | --- |
| `ci.yml` | push/PR → `main` | `gate` on windows-latest: gofmt, vet, generate drift, CGO build, `-race` tests, non-GUI cross-compile (darwin/arm64, linux/amd64). `macos` matrix on macos-15 (arm64) and macos-15-intel (amd64): `deploy/build.sh`, smoke, vet, `-race`, artifact upload. **`ci.yml` defines done.** |
| `gitleaks.yml` | push/PR → `main`, dispatch | Secret scan. Permissions: `contents: read`, `pull-requests: read`. |
| `static-checks.yml` | push/PR → `main`, weekly `41 7 * * 1` UTC, dispatch | Free, tokenless: `govulncheck` v1.9.0 (windows-latest + MSYS2 so CGO GUI packages are analyzed), `actionlint` v1.7.12, `zizmor` 1.31.1 `--offline`. Permissions: `contents: read`. Separate from `ci.yml`, so new advisories do not block PRs or the macOS prerelease. |
| `release.yml` | `v*` tag, schedule `17 8,20 * * *` UTC, dispatch | Builds and verifies all platforms. Only a `v*` tag publishes (a dispatch also needs `publish: true`). The schedule rebuilds only if `main` moved. |
| `release-main-macos.yml` | `workflow_run` of CI / gitleaks / CodeQL on `main` | For a merged PR commit, it requires a successful CI push run, gitleaks push run, and default-setup CodeQL run (`dynamic/github-code-scanning/codeql`, run name "Push on main"). It waits 5 min, then publishes the CI macOS ZIPs as prerelease `main-<sha>`. |

CodeQL is expected to come from GitHub **default setup**, not a workflow file.
Do not add a CodeQL workflow unless the owner first turns off default setup. A
missing Analyze check means the scan is off; it is not a pass.

### Diagnosing red CI

Infrastructure failures are not code defects. Do not "fix" them in code, and
do not weaken gates to get around them; report them to the owner.

- A failed job with no failed step usually never started (often billing).
  Read `gh api repos/cgfixit/LanCartographer/check-runs/<id>/annotations`
  before debugging.
- If the repository becomes private, default-setup CodeQL can stop (private
  repos need GitHub Code Security), and without a "Push on main" CodeQL run
  `release-main-macos.yml` cannot publish. Check `gh api
  repos/cgfixit/LanCartographer --jq .visibility`.
