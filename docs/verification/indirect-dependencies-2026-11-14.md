# Indirect Go dependency verification

The candidate selects x/image v0.41.0, x/net v0.55.0 and their shared x/text
requirement v0.37.0. These versions cover the advisory ranges reviewed in
[#25](https://github.com/cgfixit/LanCartographer/issues/25). The desktop app
uses these modules through Fyne's text rendering, SVG rendering and
localization. The dependency paths alone do not establish reachability of
the affected decoder, parser or proxy functions from an untrusted input.

The native build was made from main `0596c3d` with the exact module-file diff
in `5a75411`, using Go 1.27.3, Fyne 2.8.2 and macOS 27.2.1 on Apple Silicon.
The running bundle executable matched SHA-256
`d7d62a802d5339aab578d611eb23dd06479e81e90251e0155d588068ebf4de94`.

- `go mod verify`, `go mod tidy -diff` and
  `go vet ./internal/gui ./internal/probe` passed.
- `bash deploy/build.sh` produced the signed app and ZIP.
  `python3 tests/macos-smoke.py dist` verified the declared deployment targets
  and ran both packaged CLI paths against owned loopback.
- Computer Use verified light and dark rendering, the authorization prompt,
  an owned HTTP fixture's inventory row and title, favorites save/reload, and
  the Sessions file picker and cancel path. A disposable profile contained
  the expected theme and favorites files.

The native run covered Apple Silicon. Hosted CI covers Windows and Intel
build and test paths. Interactive Windows, Intel and macOS 13 acceptance
remain outside this run.
