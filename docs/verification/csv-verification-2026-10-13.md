# CSV formula-prefix verification

Verified locally on 2026-10-13, independently from base
`f250435d593a71fba445aa87d477ecdac5012439`, with this PR's Go source changes.
Toolchain: Go 1.27.1, Fyne 2.8.1, Apple clang 21.0.0, macOS 27.0.1 arm64.
This addresses finding #3 from the bounded optimize review.

## Change and baseline

Network-derived text entered CSV cells verbatim. Formula-like prefixes are now escaped with an apostrophe, including after whitespace and full-width forms. Go CSV quoting still handles field boundaries, and JSON remains raw.

TestWriteCSV_FormulaLikeText failed on the unmodified implementation: for example, the exported name cell was `=1+1` rather than an apostrophe-prefixed value.

## Native Computer Use

The independently built native candidate loaded a synthetic Favorites record for 127.0.0.1 with hostname `=1+1`. Native keyboard input invoked Export and saved CSV and JSON to the task-owned temporary directory. Parsing those actual files confirmed the CSV name cell was `'=1+1`, while the JSON hostname remained `=1+1`.

The QA copy used a distinct bundle identifier/executable filename and was
re-signed ad hoc so Computer Use could distinguish it. Application source
was not altered for that copy. Signing changes the embedded signature, so
its hash differs from the packaged executable. APPDATA selected disposable
application data. Native keyboard navigation was used because mouse
operations intermittently returned `noWindowsAvailable`. A macOS Desktop
access prompt during CSV verification was handled by the user after Computer
Use refused access to UserNotificationCenter. That prompt was an OS access
boundary, not a failing CSV assertion.

Packaged app executable SHA-256: `a87c5e629a3450d68def3deacdd2f582a3adffe79f8a0bf619d181acef66977c`.

QA executable SHA-256: `754944cf423c9bd88752b81ab2c3a66fe2c1e140c06b5afaed32f7b645826338`.

## Executed local checks

```bash
gofmt -l .                         # empty
go test -race ./internal/model -run 'TestWriteCSV|TestWriteJSONRoundTrip' -count=1
go vet ./internal/model ./internal/export
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

This verifies emitted text, not Excel or another spreadsheet engine. Saving/reopening CSV can remove text escapes; README and SECURITY.md document the consumer-dependent limit. JSON is the unchanged format for raw values. See [OWASP CSV injection guidance](https://owasp.org/www-community/attacks/CSV_Injection).

Native interaction was on this arm64 Mac only. Windows/Intel native UI,
macOS 13 runtime compatibility, and downloaded Gatekeeper acceptance were
not exercised. No release was published.
