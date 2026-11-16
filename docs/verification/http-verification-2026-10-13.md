# HTTP title byte-offset verification

Verified locally on 2026-10-13, independently from base
`f250435d593a71fba445aa87d477ecdac5012439`, with this PR's Go source changes.
Toolchain: Go 1.27.1, Fyne 2.8.1, Apple clang 21.0.0, macOS 27.0.1 arm64.
This addresses finding #1 from the bounded optimize review.

## Change and baseline

Unicode lowercasing changed byte lengths, so indexes into the folded response could panic when slicing the original body. ASCII-only tag folding keeps byte offsets intact, including invalid UTF-8 before a tag.

A real loopback HTTP fixture serving 100 copies of U+023A before an uppercase title caused a slice-bounds panic in TestHTTPTitle/Unicode_before_tags_preserves_byte_offsets on the unmodified implementation.

## Native Computer Use

The independently built native app scanned only 127.0.0.1/32 with the fixture on port 8080, completed at 100%, and remained responsive. The host was selected and Save Favorites was invoked with native keyboard input. Reading that disposable Favorites file confirmed the exact title `Native Unicode K Ⱥ fixture`. The packaged CLI independently returned the same title.

The QA copy used a distinct bundle identifier/executable filename and was
re-signed ad hoc so Computer Use could distinguish it. Application source
was not altered for that copy. Signing changes the embedded signature, so
its hash differs from the packaged executable. APPDATA selected disposable
application data. Native keyboard navigation was used because mouse
operations intermittently returned `noWindowsAvailable`. 

Packaged app executable SHA-256: `1009ac1197bd03585baac2e4009d1245d5183677dc9092c2fcd5214e6ce812c2`.

QA executable SHA-256: `3929e9e35ce19cc9b6a1bf05b4cd09920a2b3640dae7c1862e968b8f853c29e5`.

## Executed local checks

```bash
gofmt -l .                         # empty
go test -race ./internal/probe -run 'TestHTTPTitle|TestDetectWeb_ExtractsTitle' -count=1
go vet ./internal/probe
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

The original HTTP timeout, body cap, redirect refusal, certificate verification, and title length cap are unchanged. No scan-speed improvement is claimed.

Native interaction was on this arm64 Mac only. Windows/Intel native UI,
macOS 13 runtime compatibility, and downloaded Gatekeeper acceptance were
not exercised. No release was published.
