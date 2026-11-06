# AGENTS.md — LanCartographer operating contract

Cross-agent contract for any AI coding agent working in this repo. Read it
fully before your first edit. Repo: `github.com/cgfixit/LanCartographer`;
canonical remote is `origin`. MIT licensed. Repository visibility is the
owner's decision alone — agents do not change it either way.

## 1. Product

LanCartographer is an **original** Windows 10/11 and macOS LAN & port inventory tool:
Go 1.27.1, Fyne v2.8.1 GUI, and a stdlib-flag CLI sharing one scan engine.
It is authorization-gated and zero-telemetry. It is inspired by — and must
credit, without implying affiliation — Advanced IP Scanner (Famatech),
Angry IP Scanner, and Nmap. The CFAA authorization sentence governs
anything touching scan behavior and must be preserved verbatim wherever
referenced (LICENSE, README, authz flow):

> Scan only networks you own or have explicit authorization to inventory. Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws.

The Windows manifest is `asInvoker` — never `requireAdministrator`. macOS
runs unelevated; an empty ARP cache is not evidence that root is needed.

## 2. Layout quick map

- `cmd/lancartographer/` — GUI entry point + `go:generate` directives;
  contains the intentionally-checked-in `resource.syso` (never ignore/delete).
- `internal/` — one package per concern: `discover`, `probe`, `scan`,
  `model`, `authz`, `oui` (`oui_data.go` is generated AND intentionally
  checked in), `winutil`, `gui`, `cli`, `wol`, `favorites`,
  `export`, `hostcache`, `appdata`, `session`.
- `tools/icongen/` — icon/resource generator.
- `tests/ci-gate.sh` — Windows-oriented full gate, also represented in CI.
- `tests/macos-smoke.py` — owned loopback fixture exercising both packaged entry points.
- `.github/workflows/` — CI source of truth; `ci.yml` defines done.
- `deploy/build.sh` — native platform build: Windows GUI/CLI executables, or macOS
  `.app` plus CLI and architecture ZIP. The macOS build temporarily moves the
  Windows COFF resource aside and restores it on exit.
- `packaging/macos/Info.plist` — bundle identity and local-network usage text.
  Ad-hoc signing verifies integrity; it is not Developer ID signing/notarization.
  Keep the plist minimum OS and the CGO compilation/link deployment targets
  aligned; the runtime smoke inspects the Mach-O headers.
- `.agents/skills/optimize/SKILL.md` — the optimize skill (see §5).
- `.claude/skills/` — Claude Code drivers: `lan-optimize` (scan + draft PRs)
  and `lan-guard` (`check.sh`, the scriptable invariant check). `CLAUDE.md`
  imports this file.

`go.sum` is committed. The two generated/binary files above stay tracked.

## 3. Execution and ownership map

- `cmd/lancartographer/main.go` dispatches to CLI for `--cli` or a filename
  containing `cli`; otherwise it starts Fyne. macOS can reuse one executable
  for both entry points because its linker flags are identical.
- GUI/CLI perform authorization before invoking `internal/scan`; direct
  engine callers must apply the same policy. `scan` owns the worker pool,
  ordered event emission, cancellation, and enrichment/cache lifecycle.
- `probe` owns bounded TCP/HTTP probes; `discover` and `winutil` contain
  platform-specific interface, ARP, elevation, and SMB boundaries.
- Fyne widgets are updated on the UI thread (`fyne.Do` from workers).
  Apply the saved theme before creating widgets; shared panel labels must
  respond to theme changes. Subprocess arguments are built in platform files.
- `session` owns bounded saved snapshots, compatibility checks and selected
  plain-text reports. GUI session copies never replace the live inventory.
  Full JSON is unredacted; report redaction is on by default. Save/open/compare/
  export do not invoke scans or probes. See `USER-GUIDE.md` for the workflow.
- `appdata.Path` chooses the native per-user directory, with an absolute
  `APPDATA` override for Windows/fixtures. Never fall back to cwd. Persistence
  uses temporary files renamed into place; new directories are mode 0700.

## 4. Commands and verification

Honor `go.mod` and inspect the actual compiler/PATH before building. Fyne
requires CGO. Windows needs MSYS2 ucrt64 gcc and `goversioninfo`; macOS uses
Xcode Command Line Tools and system clang, without Windows `CC=gcc` settings.

```bash
bash deploy/build.sh
# macOS: exercise the packaged CLI and app --cli against an owned fixture
python3 tests/macos-smoke.py dist
# changed workflows
actionlint .github/workflows/ci.yml .github/workflows/release.yml .github/workflows/gitleaks.yml
```

- Run changed behavior as the primary local check. Use gofmt, relevant vet,
  builds, workflow lint, and focused regression checks for concrete risks.
  Do not run the full local suite by default; hosted CI covers broad/platform
  regressions. Record any user waiver separately from passing evidence.
- CI runs the Windows gate and macOS 15 arm64/Intel native builds, packaged
  smoke checks, vet, and race tests. A cross-compile does not verify Fyne.
- macOS manual `go build ./...` or tests linking `cmd/lancartographer` need the
  Windows `resource.syso` moved aside and restored; prefer `deploy/build.sh` for
  packaging. Never delete or commit a changed Windows resource accidentally.
- UI changes require actual native interaction and rendered inspection.
  Keep native GUI, CLI fixtures, focused tests, hosted CI, and downloaded
  release acceptance distinct. Use owned loopback fixtures and disposable
  data directories; never publish real LAN inventory or credentials.
- Preserve the HTTP one-second/32-KiB limits, TLS verification, no redirects,
  dial budget, worker limit, cancellation, and monotonic progress events.
- No telemetry or runtime downloads. Justify new network paths and preserve
  the authorization notice verbatim.
- One concern per branch/PR; open drafts and never push to `main`. Commit and
  publish only within the user's authorized scope. Do not merge without an
  explicit request.
- Validate changed YAML syntax and actionlint, including `.github/workflows/gitleaks.yml`. Inspect scripts invoked by CI.
- `gitleaks.yml` runs on push to `main` and on pull request to `main`. Its permissions are `contents: read` and `pull-requests: read`.

## 5. The optimize skill

When asked to optimize, audit, or find improvements in this repo, use the
skill at `.agents/skills/optimize/SKILL.md` (repo-relative path — read it
first). It defines a 4-minute read-only scan of the repo and a scoped
multi-PR process (one concern per PR, drafts only). Follow its process
rather than free-styling an audit. `/lan-optimizer` loads
`.agents/skills/lan-optimizer/SKILL.md`, which adds branch topology and
the draft-PR gate. `todo.txt` is the current deletion list.

## 6. CI status notes

CodeQL was removed while the repository was private. Check current visibility,
plan, workflow configuration, and authorization before proposing restoration;
an absent security workflow is not a passing security scan.
