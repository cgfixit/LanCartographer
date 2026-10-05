# AGENTS.md — LanCartographer operating contract

Cross-agent contract for any AI coding agent working in this repo. Read it
fully before your first edit. Repo: `github.com/cgfixit/LanCartographer`;
canonical remote is `origin`. MIT licensed. Repository visibility is the
owner's decision alone — agents do not change it either way.

## 1. Product

LanCartographer is an **original** Windows 10/11 LAN & port inventory tool:
Go 1.27, Fyne v2.8.1 GUI, and a stdlib-flag CLI sharing one scan engine.
It is authorization-gated and zero-telemetry. It is inspired by — and must
credit, without implying affiliation — Advanced IP Scanner (Famatech),
Angry IP Scanner, and Nmap. The CFAA authorization sentence governs
anything touching scan behavior and must be preserved verbatim wherever
referenced (LICENSE, README, authz flow):

> Scan only networks you own or have explicit authorization to inventory. Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws.

The app manifest is `asInvoker` — never `requireAdministrator`.

## 2. Layout quick map

- `cmd/lancartographer/` — GUI entry point + `go:generate` directives;
  contains the intentionally-checked-in `resource.syso` (never ignore/delete).
- `internal/` — one package per concern: `discover`, `probe`, `scan`,
  `model`, `authz`, `oui` (`oui_data.go` is generated AND intentionally
  checked in), `winutil`, `gui`, `cli`, `maplayout`, `wol`, `favorites`,
  `export`, `hostcache`.
- `tools/icongen/` — icon/resource generator.
- `tests/ci-gate.sh` — the local gate; run it via bash.
- `.github/workflows/` — CI source of truth; `ci.yml` defines done.
- `build.sh` — runs `go generate ./...` then builds `dist/LanCartographer.exe`
  (`-trimpath -ldflags "-s -w -H windowsgui"`) and
  `dist/lancartographer-cli.exe` (no `-H windowsgui`).
- `.agents/skills/optimize/SKILL.md` — the optimize skill (see §5).

`go.sum` is committed. The two generated/binary files above stay tracked.

## 3. Commands

Toolchain PATH: Go at `/c/Program Files/Go/bin`, gcc (MSYS2 ucrt64) at
`/c/msys64/ucrt64/bin`. Everything below needs `CGO_ENABLED=1 CC=gcc`.

**macOS gate notes:** there is no MSYS2 on the macOS CI job — CGO uses the
runner's system clang (`CGO_ENABLED=1`, default `CC`). `go generate ./...`
on macOS runs only `tools/icongen`; the goversioninfo directive lives in
`cmd/lancartographer/generate_windows.go` (`//go:build windows`), and the
checked-in `resource.syso` regenerates on Windows only. The Windows CI job
also runs cross-compile smoke builds (`CGO_ENABLED=0 GOOS=darwin
GOARCH=arm64` and `GOOS=linux`, non-GUI packages only — Fyne's desktop
driver requires CGO, so the GUI build for darwin is verified natively by
the macos-latest job, never by cross-compile). Keep those green; a
Windows-only API used in the engine, CLI, or `winutil` stubs fails the
gate even though the Windows build is fine.

The local gate — run from the repo root:

```bash
bash tests/ci-gate.sh
```

It runs, in order: `gofmt -l .` (must be empty), `go vet ./...`,
`go generate` drift check, CGO build of both exes, and
`go test ./... -race` (race detector requires CGO + gcc).

Release-style builds:

```bash
bash build.sh   # dist/LanCartographer.exe + dist/lancartographer-cli.exe
```

## 4. Standing preferences

- **VERIFY BY RUNNING.** After code changes, actually run the program or
  the tests and observe the result. A green compile is not verification.
- **PREFER CI/LINT WORKFLOWS OVER ad-hoc test picking.** Treat
  `.github/workflows/ci.yml` as the source of truth and make changes pass
  the full gate (`bash tests/ci-gate.sh`) rather than running a single
  favorable test. When time-constrained, gofmt + vet + build still
  precede any claim of done.
- One concern per branch/PR. Open PRs as drafts. Never push to `main`.
- No telemetry, ever. No new network paths without explicit justification.
- Never weaken the authorization gate (`internal/authz`) or drop the CFAA
  sentence from any user-facing surface.
- Never `git add`/`git commit` unless the user explicitly asks; the
  controller commits after review.
- Where you create YAML, validate it parses:
  `python -c "import yaml,sys; yaml.safe_load(open('<file>', encoding='utf-8'))"`

## 5. The optimize skill

When asked to optimize, audit, or find improvements in this repo, use the
skill at `.agents/skills/optimize/SKILL.md` (repo-relative path — read it
first). It defines a 4-minute read-only scan of the repo and a scoped
multi-PR process (one concern per PR, drafts only). Follow its process
rather than free-styling an audit.
