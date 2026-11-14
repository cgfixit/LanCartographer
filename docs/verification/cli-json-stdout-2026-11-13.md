# CLI JSON stdout — 2026-11-13

Application source `84bfa71f537135121a51ef32e2f757be93e3f3cf`, independently
based on `main` at `8caf248`. Go 1.27.3, Fyne 2.8.2, macOS 27.2.1 arm64.
No dependency or workflow changes. The build temporarily moves the tracked
Windows resource aside, so Go's build metadata marks the tree modified; the
resource was restored and the application source matches this commit.

Final packaged executable SHA-256:

- CLI: `8c96d0db5726ea00ebd0e380f2c421f864db07ac49abc763ee649ea0e2d1b509`.
- Signed app executable: `bfa3ff282a8c430b590b23999a829cf220fa1b889b025023da915de6a2f5231c`.

## Checks

- `gofmt`, `git diff --check`: clean.
- `go test ./internal/cli -run 'Test(JSON|TableSanitizes)' -count=1`: pass.
  Covers JSON-only output, empty array, raw-value round trip, no file export,
  output-write failure, scan failure, existing file export and table control
  sanitization.
- `go vet ./internal/cli`: pass.
- `bash deploy/build.sh`: pass, including macOS bundle signature verification.
- `python3 tests/macos-smoke.py dist`: pass through both packaged entry points.
  JSON stdout parsed and matched file JSON; no literal `-` file was created;
  refusal exited 2 with no stdout or HTTP request. The same smoke checks
  deployment targets and license packaging. Both final committed binaries
  were then directly run with `--json -` against an owned loopback service;
  their JSON included the expected HTTP title.
- Computer Use drove the final native app with disposable `APPDATA`: first-run
  notice, loopback authorization, live scan, host selection/details and native
  JSON export. A local assertion confirmed 8080 open with the title
  `Developer native HTTP fixture` in the exported inventory. No real LAN scan.

## Scope and limitations

The user prepared Desktop 4; native interaction used Computer Use. This Fyne
product has no browser web application. No full local suite, Windows, Intel
macOS, macOS 13, or downloaded-release/Gatekeeper acceptance was performed.
Hosted CI is separate. The GUI source is unchanged by this PR.

The GUI custom-port branch and this branch trial-merged without conflicts;
the combined package also passed the packaged loopback smoke. That combined
result is additional evidence, not a substitute for the independent checks
above. Raw inventory/session files remain outside the repository.
