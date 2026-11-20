---
name: lan-diagnostic
description: >
  Checklist for adding or changing a bounded, read-only diagnostic tool in
  LanCartographer (a Tools menu entry backed by an OS helper or API, like
  Connections or Route & DNS). Use when implementing a slice from issue #40
  or the workbench roadmap, when porting a diagnostic to Windows, or to run
  /lan-diagnostic.
---

# LanCartographer diagnostic slice

Diagnostics read local state and show it. They are where this app is most
likely to grow an unbounded subprocess, a silent outbound request or a
leak of real network data, so each slice follows the same shape.

Read `AGENTS.md`, `INVARIANTS.md` and `docs/network-workbench-roadmap.md`
first. They win over this file. Refresh issue #40 and #22 for the agreed
order before picking a slice.

## 1. Answer these before writing code

Write the answers in the PR body.

1. **What does it contact?** Nothing, the local OS only, the LAN, or the
   Internet. Anything past the local OS is a new egress path: it needs a
   confirmation that names the destination and traffic, and an update to
   `INVARIANTS.md` and `SECURITY.md` in the same PR.
2. **Does it reach the scan engine or `servicecheck`?** Then it checks
   `authz.NeedsConfirmation` before contacting anything.
3. **What is the source?** An OS API, or a helper binary at a fixed
   absolute path. Name the man page or vendor reference.
4. **What are the bounds?** A deadline, an output cap, a row cap, and what
   the user sees when each one trips.
5. **Does it need elevation?** It must work unelevated. A missing field is
   shown as unavailable; it is never a reason to ask for root or to change
   the manifest.
6. **What input does the user type?** Parse it to a typed value
   (`netip.Addr`, a validated interface name) before it gets near argv.

## 2. Code shape

Follow the existing slices: `connections_*` and `route_*` in
`internal/macdiag` and `internal/gui`.

- **Parser**: `internal/macdiag/<name>_parse.go` with no build tag, so it
  compiles and its tests run on every OS. It takes bytes and returns typed
  rows. Cap the row count. Reject control characters before anything is
  rendered or copied.
- **Reader**: `internal/macdiag/<name>_darwin.go`. Build argv from typed
  values in this platform file. Call `macdiag.Run`, which fixes `PATH` and
  `LC_ALL=C` and caps each output stream.
- **Deadline**: `macdiag.Run` has none of its own. Wrap the context with
  `context.WithTimeout` at the call site. Local reads use about 2 s.
- **Errors**: map helper failures to a short sentence the user can act on.
  Do not show raw stderr.
- **GUI**: `internal/gui/<name>_darwin.go` (`//go:build cgo`) holds the
  window; `internal/gui/<name>_other.go` (`//go:build cgo && !darwin`)
  holds the stub with the same method set. Add the entry through a
  `<name>Menu()` method appended in `mainMenu` in `gui.go`.
- **Lifecycle**: one job at a time using `diagnosticJob` (cancel plus a
  `done` channel), a visible Stop button, and a `stopAndWait` that app
  shutdown calls. Touch widgets only through `fyne.Do` from the worker.
- **No side effects**: opening, filtering or closing the window starts no
  scan and no probe, and never replaces the live inventory.
- **Subprocesses** are argv-only. No shell, no string concatenation into a
  command, no helper found through `PATH`.

A Windows backend keeps the same view and snapshot semantics and goes in
`_windows` files. Moving shared types out of the `macdiag` package is its
own PR, opened before the backend.

## 3. Tests

Focused tests only, next to the code:

- Parser: a normal sample, an empty one, a truncated one, one over the
  row cap, and one with control characters. Use synthetic text. Never
  paste output from a real machine.
- Input: every rejected form (hostname where a literal is required, a
  port, an unknown zone, a leading dash).
- Runner: a child that stalls and a child that floods output both end
  within the bounds.
- Stub: the non-darwin build still compiles
  (`GOOS=windows go vet ./internal/macdiag/...` covers the untagged parts).

## 4. Verification

Use `.agents/skills/lan-verify/SKILL.md`. For a diagnostic, the native tier
is required on the platform it targets:

- Reproduce the result with an owned fixture: a loopback listener, a
  loopback route, a local resolver read.
- Show Stop and window close during a running job.
- Show light and dark themes with details that could identify a real
  network collapsed or replaced by fixture data.
- Confirm with the fixture log that opening the window sent nothing.
- Write a dated record and add it to `docs/README.md`.

## 5. Docs in the same PR

- `docs/USER-GUIDE.md`: the workflow and its limits.
- `docs/network-workbench-roadmap.md`: mark the item shipped and link the
  record.
- `README.md` feature list, if the tool is user-visible there.
- `AGENTS.md` §2 if a package or a feature file was added.
- `INVARIANTS.md` and `SECURITY.md` only when question 1 found new egress.

## Do not

- Run a helper through a shell, or pass unparsed user text as an argument.
- Add a background poller. Refresh is manual unless the roadmap says
  otherwise, and any watch is foreground, bounded and stoppable.
- Send anything off the machine without a confirmation naming where.
- Request elevation, entitlements or a manifest change.
- Store diagnostic output in session JSON or `APPDATA` unless the slice
  was specified to.
- Put real SSIDs, MACs, hostnames, process lists or addresses in tests,
  screenshots or records.
