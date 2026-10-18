# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`AGENTS.md` is the cross-agent operating contract and wins on any conflict.
Read it first; this file only adds what Claude Code needs on top of it.

@AGENTS.md

## Commands

The packaged builds need CGO and a native host (see `AGENTS.md` §4). A Linux
cloud session has neither Fyne's C deps nor MSYS2, but everything outside the
GUI builds and runs there, because `internal/gui` swaps in a `!cgo` stub
(`run_nocgo.go`):

```bash
gofmt -l .                                   # must print nothing
CGO_ENABLED=0 go vet ./...                   # GUI files drop out via the cgo tag
go test ./internal/probe/ -run TestHTTPTitle -count=1    # one package / one test
CGO_ENABLED=0 go build -o "${TMPDIR:-/tmp}/lc-cli" ./cmd/lancartographer
```

Run the changed behavior through the CLI against an owned loopback fixture
and a throwaway data dir (the binary name containing `cli` selects CLI mode):

```bash
python3 -m http.server 18080 --bind 127.0.0.1 &           # owned fixture
APPDATA="$(mktemp -d)" "${TMPDIR:-/tmp}/lc-cli" --range 127.0.0.1/32 --i-am-authorized --ports 18080
```

A Linux pass is not Windows or macOS evidence: the ARP, adapter, SMB and
elevation code there is the `_other.go` fallback. Hosted CI (`ci.yml`) is the
platform check; native GUI work still needs a real desktop (`AGENTS.md` §4).

Workflows: `actionlint .github/workflows/*.yml` (install with
`go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` if missing).

## Architecture

- **One engine, two front ends.** `cmd/lancartographer/main.go` picks CLI
  (`--cli` or a `cli` exe name) or GUI. Both call `authz.NeedsConfirmation`
  themselves (outside local subnets, non-RFC1918, or broader than /22 →
  confirm), then `scan.Run(ctx, prefix, probe.Options, chan<- scan.Event)`.
  `scan.Run` does not enforce authorization; it only rejects non-IPv4 and
  anything broader than /12.
- **Event stream contract** (`internal/scan/engine.go`): live
  `EventPortFound`/`EventShareFound` during probing, exactly one enriched
  `EventHostFound` per alive host, monotonic `EventProgress`, then
  `EventDone`. Per-host failures degrade to empty fields, never
  `EventError`. The GUI table and the CLI table both depend on this order.
- **Two concurrency knobs, not one.** `maxHostsInFlight = 64` host workers,
  and an independent global dial budget (`probe.Limiter`, default 128,
  clamped to 256) shared by liveness and port dials.
- **Per-host pipeline:** liveness (`discover`: fixed port set plus the
  scan's ports, first connect wins, ARP fallback) → port probes (`probe`:
  TCP, then HTTP/HTTPS title under the 1 s / 32 KiB / no-redirect / verified
  TLS limits) → enrichment (MAC via ARP, OUI vendor, SMB shares, name
  NetBIOS > mDNS > PTR where a better answer cancels the rest) → MAC-keyed
  name cache (`hostcache`).
- **Platform split by build tags.** `winutil` carries `_windows.go` /
  `_darwin.go` / `_other.go` implementations and `gui/actions_*` the
  per-OS launch actions; Fyne files are `//go:build cgo`. `*_parse.go`
  files hold the OS-independent parsing and are where cross-platform unit
  tests live. The engine reaches platform code through the `deps` struct in
  `scan` (and `cli`), which tests replace with fakes; prefer that seam over
  real sockets in tests.
- **Generated, checked-in files.** `go generate ./...` runs
  `tools/icongen` (→ `app.ico`) on every OS and `goversioninfo`
  (→ `resource.syso`) on Windows only; `internal/oui/gen.go` turns
  `data.csv` into `oui_data.go`. CI fails if `oui_data.go` or `app.ico`
  drift; `resource.syso` is excluded because its bytes vary by generator
  version.

## Claude Code specifics

- No `gh` in cloud sessions: list PRs, read CI logs and open drafts through
  the GitHub MCP tools. PR bodies mirror `.github/PULL_REQUEST_TEMPLATE.md`;
  in "Test evidence", list only commands actually run and name what wasn't
  run (for example "Windows gate: hosted CI").
- Project skills live in `.claude/skills/`: `lan-optimize` (scan, then
  scoped draft PRs; modeled on CyClaw-Optimize) and `lan-guard` (the
  deterministic invariant check). Run
  `bash .claude/skills/lan-guard/check.sh` before any PR that touches
  workflows, `authz`, the manifest, LICENSE/README, or generated files.
- The agent-agnostic skills in `.agents/skills/` stay authoritative for
  other tools; the `.claude/` copies must not contradict them.
