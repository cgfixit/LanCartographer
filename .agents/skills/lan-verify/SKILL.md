---
name: lan-verify
description: >
  Verify a LanCartographer change by running it against an owned loopback
  fixture with a disposable data directory, then report the evidence by
  tier and, when warranted, write a new dated verification record. Use
  before opening a PR, when asked to verify, smoke-test or prove a change,
  or to run /lan-verify.
---

# LanCartographer verification

The repo rule is "verify by running it". This skill is the repeatable way
to do that without touching a real network or real user data.

Read `AGENTS.md` §0 and §4 first. They win over this file.

## 1. Pick the tiers the change needs

Evidence tiers stay separate. Never let one stand in for another.

| Tier | Proves | Where it can run |
| --- | --- | --- |
| CLI fixture | Scan engine, gate, JSON output | Windows, macOS, Linux |
| Focused test | One package's behavior | Wherever that package builds |
| Native GUI | Fyne windows, themes, dialogs | Windows or macOS only |
| Packaged smoke | The built artifacts | macOS (`build/macos-smoke.py`) |
| Hosted CI | Broad and cross-platform regressions | After push |
| Downloaded release | What a user installs | After a release build |

A Linux cross-compile proves that non-GUI packages compile. It does not
verify Fyne, the macOS helpers or the Windows APIs. Say so in the report.

## 2. CLI fixture on loopback

This runs on any OS with Go and Python 3. The fixture binds `127.0.0.1`
only, and the data directory is thrown away at the end.

```bash
tmp=$(mktemp -d); mkdir "$tmp/www" "$tmp/appdata"
python3 -m http.server 8080 --bind 127.0.0.1 --directory "$tmp/www" >"$tmp/fixture.log" 2>&1 &
fixture=$!
CGO_ENABLED=0 go build -o "$tmp/lancartographer" ./cmd/lancartographer
scan=(--cli --range 127.0.0.1/32 --ports 8080 --timeout 100ms --json -)

APPDATA="$tmp/appdata" "$tmp/lancartographer" "${scan[@]}"                      # expect exit 2
grep -c GET "$tmp/fixture.log"                                                # expect 0
APPDATA="$tmp/appdata" "$tmp/lancartographer" "${scan[@]}" --i-am-authorized  # expect exit 0

kill "$fixture"; rm -rf "$tmp"
```

Expected results:

- Without `--i-am-authorized`, the CLI prints the CFAA notice, exits 2,
  writes nothing to stdout, and the fixture log has no request. A refused
  scan that reaches the fixture is a gate failure. Stop and report it.
- With the flag, stdout is JSON only: one host, `127.0.0.1`, port 8080
  `open`, title `Directory listing for /`.

Notes:

- Use a port from `discover.DiscoveryPorts` (8080 or 8443 need no
  privilege), or the host is never found alive.
- If the port is occupied, pick another discovery port. Never stop or take
  over someone else's listener.
- `CGO_ENABLED=0` builds the CLI path with the non-CGO GUI stub. It is the
  right build for this tier and the wrong one for anything visual.
- On macOS a plain `go build` that links `cmd/lancartographer` trips over
  the COFF `resource.syso`. Use `bash build/build.sh` and run
  `dist/lancartographer-cli` instead.

Extend the fixture to match the change: a TLS listener for certificate
paths, a closed port for refusal, a slow handler for timeouts and
cancellation, a body over 32 KiB for the HTTP cap.

## 3. Native and packaged tiers

- macOS: `bash build/build.sh`, then `python3 build/macos-smoke.py dist`.
- UI changes need real interaction in the native app, before and after
  screenshots, and both light and dark themes. Launch with a disposable
  `APPDATA`. Scan only the loopback fixture so no real host, MAC, SSID or
  hostname appears in an image.
- If the session has no desktop for the platform, do not substitute a
  rendered mock. Record the gap.

## 4. Lint

```bash
gofmt -l . && go vet ./...
actionlint .github/workflows/*.yml   # only when a workflow changed
```

Add a focused `go test ./internal/<pkg> -run <Name> -count=1` for behavior
you changed. Do not run whole suites to prove one change.

## 5. Report

List each tier as one of:

- **Passed**: the exact command and the observed result.
- **Not run**: why, and which hosted CI job covers it.
- **Waived by the owner**: quoted, and kept apart from passing evidence.

Paste the same list into the PR's "Test evidence" and "Platform and CI
limitations" sections.

## 6. Dated record

Write a record when the change is user-visible, touches a measured limit
or the gate, or changes packaging. A typo fix does not need one.

- New file: `docs/verification/<topic>-YYYY-MM-DD.md`. Images go in
  `docs/verification/<topic>/`.
- State the platform and OS build, Go and compiler versions, the commit,
  the commands, what was observed, and what still needs its own
  environment.
- Add a row to the table in `docs/README.md`.
- Never edit an older record to make it current. Old records keep their
  old paths and results.

## Do not

- Scan anything but loopback or a fixture you own.
- Use the real `APPDATA`, real session JSON or real inventory, in commands,
  logs, screenshots or the record.
- Report a tier as passed because a neighboring tier passed.
- Skip, disable or loosen a check to get a pass.
