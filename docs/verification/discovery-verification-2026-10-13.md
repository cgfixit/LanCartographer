# Selected-port discovery verification

Verified locally on 2026-10-13, independently from base
`f250435d593a71fba445aa87d477ecdac5012439`, with this PR's Go source changes.
Toolchain: Go 1.27.1, Fyne 2.8.1, Apple clang 21.0.0, macOS 27.0.1 arm64.
This addresses finding #2 from the bounded optimize review.

## Change and baseline

The engine checked a fixed discovery port list before reaching the selected-port scan. Hosts answering only another selected port and lacking an ARP entry disappeared. Discovery now probes the deduplicated union of the existing list and selected ports through the same limiter.

TestRun_SelectedPortOnly failed on the unmodified implementation. A packaged CLI scan of the owned loopback port-9100 fixture exported an empty array. The native baseline also completed at 100% with zero hosts.

## Native Computer Use

The independently built native candidate scanned only 127.0.0.1/32 against the same port-9100-only fixture. After the authorization confirmation it completed at 100%, displaying one host, one open port, and `9100 open Printer`. The packaged CLI independently exported that host. A focused check also exercised a real ephemeral TCP listener outside the default discovery port list.

The QA copy used a distinct bundle identifier/executable filename and was
re-signed ad hoc so Computer Use could distinguish it. Application source
was not altered for that copy. Signing changes the embedded signature, so
its hash differs from the packaged executable. APPDATA selected disposable
application data. Native keyboard navigation was used because mouse
operations intermittently returned `noWindowsAvailable`. 

Packaged app executable SHA-256: `f23b145fc0304fe8d23af5a3035bd232af32533d6df9ceae5e61760825792571`.

QA executable SHA-256: `1ea459ab6e2de196dfbca3403434f726f85f3dac33c06de758b747d1d1345021`.

## Executed local checks

```bash
gofmt -l .                         # empty
go test -race ./internal/discover ./internal/scan -run 'TestAlive_|TestRun_SelectedPortOnly|TestRun_Stop|TestRun_Progress' -count=1
go vet ./internal/discover ./internal/scan
bash build.sh                     # native build, generation, bundle signing/verification
python3 tests/macos-smoke.py dist  # both packaged entry points, authorization refusal,
                                  # authorized loopback scan/title/JSON and deployment target
git diff --check                  # pass
```

All listed checks passed. Build generation left the tracked icon, Windows
resource, and OUI data unchanged. Existing CI/release/gitleaks workflows were
also linted with actionlint without changing them. Full local race suites and
the Windows-oriented `tests/ci-gate.sh` were not run, following the current
instruction to execute only affected code locally. Hosted CI provides broad
Windows and both macOS architecture coverage separately.

## Limits

The common discovery ports and ARP fallback remain. The authorization policy, global dial limiter, worker cap, and cancellation path are unchanged. Dead hosts may receive additional connection attempts on the ports already selected for inventory; no performance gain is claimed.

Native interaction was on this arm64 Mac only. Windows/Intel native UI,
macOS 13 runtime compatibility, and downloaded Gatekeeper acceptance were
not exercised. No release was published.
