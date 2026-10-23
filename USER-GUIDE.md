# LanCartographer User Guide — a technical tour for Python and PowerShell folks

You know Python and PowerShell. You've never read Go. This guide walks the
codebase the way you'd want it walked: every Go concept is introduced by
analogy to something you know, then used precisely. By the end you should be
able to read `internal/scan/engine.go` the way you read a well-structured
Python module.

## Analogy map

| Go construct | You know it as |
|---|---|
| `go.mod` / `go.sum` | `pyproject.toml` + `requirements.txt` + a pinned `pip freeze`, in one file (mod) plus a lockfile (sum). The module path is `github.com/local/lancartographer`, and `internal/` packages can only be imported by this module — like a `src/` layout with an enforced boundary. |
| goroutine (`go func()`) | A thread or PowerShell runspace that costs almost nothing to start. Thousands, not tens. `go f()` is roughly `Start-ThreadJob { f }` with microsecond overhead and no job object to clean up. |
| channel (`chan Event`) | `System.Collections.Queue` with blocking handoff built in. Sending blocks when it's full; receiving blocks when it's empty. The compiler and runtime own the locks — you never write them. |
| `context.Context` | A `CancellationToken`. Threaded through every blocking call so one `Cancel()` (the Stop button, Ctrl-C, a timeout) unwinds all in-flight work. Every dial in the codebase takes a ctx. |
| `struct` | A Python `@dataclass` or a `PSObject` with typed fields and no ceremony: `type Host struct { IP netip.Addr; Name string; ... }`. |
| `interface` | Python duck typing, but declared and checked at compile time. A function that takes `io.Writer` accepts anything with a `Write` method — like a protocol/ABC, enforced before runtime. |
| struct embedding | Mixin / inheritance-lite. Embedding `sync.Mutex` in a struct promotes its methods — the Go idiom for "this struct has a lock," comparable to `$this.Lock()` conventions you'd hand-roll. |
| pointer (`*Host`) | Passing a list by reference instead of a copy. Go is pass-by-value everywhere; a pointer is how a function mutates the caller's struct. Most of this codebase passes structs by value unless mutation is the point. |
| `cgo` | `ctypes` / `CDLL` — calling C from Go. This project deliberately avoids cgo for OS APIs; see "Talking to the OS without .NET" below. |
| single compiled exe | PyInstaller output, but first-class: one static-ish binary, no interpreter, no venv, no DLL hell, ~instant startup. |

## What happens when you click Scan

The walk below is the actual call chain in the code, starting at
`internal/scan/engine.go` (`scan.Run`).

1. **Target selection.** The GUI asks `internal/discover` for your local
   physical subnets, which wraps the Windows `GetAdaptersAddresses` API
   (virtual adapters excluded) and masks each adapter address to its on-link
   prefix. Your `/24` becomes the target prefix. (Or you typed a `--range` in
   the CLI.)

2. **Address expansion.** `discover.HostsIn` enumerates every usable host
   address in the prefix — for a `/24`, the 254 addresses between the network
   and broadcast addresses. The engine rejects IPv6 and anything broader than
   a `/12` before this step, so a typo can't materialize a billion-address
   list and OOM your own machine.

3. **Worker pool.** The addresses are pushed onto a `jobs` channel (a
   queue with blocking handoff), and **64 goroutines** consume them — a
   worker pool, like 64 runspaces pulling from a shared `BlockingCollection`.
   The pool sizes down automatically for small ranges. This is why a `/24`
   finishes in seconds instead of the minutes sequential liveness checks
   would take.

4. **Per-host liveness.** Each worker first asks "is anything there?"
   (`discover.Alive`): it dials `DiscoveryPorts`
   (`445, 80, 443, 135, 139, 22, 3389, 8080, 8443`, ordered by expected hit
   rate) plus the selected scan ports. Duplicates are skipped. The dials share
   the scan limiter. The first success wins and cancels the rest. If no port
   answers, an ARP lookup (`SendARP` on Windows, the ARP cache on macOS) is
   the next fallback. On macOS, a host still undetected gets one ICMP echo
   request through `/sbin/ping`, using the same concurrency limiter and a
   one-second timeout that Stop can cancel. A host can be alive with every
   probed TCP port closed or filtered. The result is recorded as `FoundVia`:
   `"tcp"`, `"arp"`, or `"icmp"`. Port states still describe the TCP probes.
   Silent host → the worker moves on; nothing is reported for dead addresses.

5. **Port probe.** For each live host, `probe.ScanHost` dials every port in
   the port list (the 15 defaults live in `internal/gui/actions.go` as
   `specPorts` and in `internal/cli/cli.go` as `defaultPorts`) with a plain
   TCP connect — `net.Dialer.DialContext`, the Go equivalent of
   `socket.create_connection()` with a timeout. Each result is classified
   `open`, `closed`, or `timeout` (`internal/model/model.go`). The per-dial
   budget is 800 ms by default (the CLI's `--timeout` sets a 400 ms
   liveness budget; the probe budget is 2×).

6. **Service identification.** Open ports get a label from a classifier
   (`internal/probe/classify.go`): 445 → `smb`, 3389 → `rdp`, and so on. If
   the label is `unknown`, the prober gives the port **one second** to answer
   an HTTP `GET /`; a device with a web interface is promoted to service
   `Web` with the page `<title>` attached (capped at 80 runes). That is how a
   random NAS or printer shows up as `Web "NAS Login"` instead of
   `unknown`.

7. **Share enumeration — only if 445 answered.** `probe.ListShares` calls
   `winutil.NetShareEnum` against the host and walks the returned buffer as
   `ShareInfo1` records: name, type (`disk`/`print`/`ipc`), remark. Windows
   may answer "access denied" under your token — that's not an error here;
   the engine synthesizes a single marker share with remark
   `"shares: access denied"` so the UI can say so honestly. Other failures
   (RPC refused, host vanished) degrade silently to an empty share list.

8. **Name resolution, three ways at once.** `resolveName` fires NetBIOS,
   mDNS, and reverse-DNS (PTR) lookups in three goroutines and takes the
   first non-empty answer by priority NetBIOS → mDNS → PTR. Each probe gets
   its own bounded context (NetBIOS/mDNS ~900 ms, PTR 1.5 s), so the
   *slowest* lookup bounds the wait, not the sum — the same trick as
   `Wait-Any` on three jobs.

9. **Enrichment.** MAC address via `SendARP`; vendor by looking the OUI
   prefix up in an **embedded** database (compiled into the binary from
   `internal/oui/oui_data.go` — no download, ever). If MAC resolution needs
   an elevated process and you're not elevated, the MAC field becomes the
   sentinel `"needs elevation"` and the GUI shows an Elevate button.

10. **Events stream to the GUI.** Nothing above writes shared state.
    Everything is an `Event` sent down one channel: `EventPortFound` and
    `EventShareFound` stream live during probing, `EventHostFound` carries
    the single canonical, fully-enriched record per alive host,
    `EventProgress` drives the progress bar, `EventDone` ends the sweep.

### Why events on a channel instead of a shared list?

Because of the GUI thread rule. Fyne, like WPF or WinForms, requires that UI
objects be touched from exactly one thread. In Go the marshaling primitive is
`fyne.Do(func(){ ... })` — functionally `Dispatcher.Invoke` in C# or
`$Dispatcher.Invoke()` in PowerShell WPF: "run this on the UI thread."

The engine runs on background goroutines and must never touch widgets
directly. So it sends immutable `Event` values down a channel; the GUI side
has one goroutine receiving them and forwarding each to `fyne.Do`, which is
the only place table rows get updated. The channel is the single seam between
"concurrent scanning world" and "single-threaded UI world" — no locks on
widgets, no race conditions in the table.

## Concurrency, in your terms

**The rate limiter is a semaphore.** `internal/probe/limiter.go` implements a
`Limiter` as a buffered channel of size *n* — think `SemaphoreSlim(n, n)`.
To run work you "park" a token (`l.sem <- struct{}{}`), do the dial, then
release it. The whole scan shares one limiter, created in `scan.Run`. That includes the
64 host workers, the discovery dials, and the selected port dials:

```go
lim := probe.NewLimiter(opts.Concurrency) // default 128, hard cap 256
```

`NewLimiter` clamps: ask for 0 or negative → 128; ask for more than 256 →
256. The cap lives in exactly one place, so a bug anywhere upstream — a bad
flag, a GUI slider wired wrong — cannot flood the LAN. One parking lot with
one gate.

**Cancellation is Ctrl-C that reaches into every socket.** Every dial takes
the sweep's `context.Context`. Clicking Stop cancels that context; every
`DialContext` in flight returns immediately, workers see `ctx.Err() != nil`
and exit instead of taking new jobs, and blocked channel sends bail out via
`select { case ch <- ev; case <-ctx.Done() }`. In PowerShell terms: one
`CancellationTokenSource.Cancel()` that every `Wait-Any` in the system
respects — including the ones queued behind the semaphore.

The `emitter` type (engine.go) deserves a mention: it wraps the outbound
event channel in a mutex so concurrent workers can't interleave sends, and
its send is `select`-ed against ctx so a cancelled sweep never wedges a
worker on a dead consumer.

## Talking to the OS without .NET (Windows and macOS)

You know P/Invoke: `Add-Type` a C signature, marshal structs by hand, hope
the packing matches. Go's `golang.org/x/sys/windows` is the same idea
shipped as a typed, maintained library — the Win32 prototypes already
declared and tested, so `internal/winutil` reads like a thin adapter rather
than a FFI project.

LanCartographer deliberately does **not** use cgo (the `ctypes` path) for
these calls. cgo would drag a C toolchain into every build and complicate
cross-compilation; the `x/sys/windows` approach compiles with plain `go build`.
That's why the build needs gcc *only* for Fyne (the GUI library), not for
Windows APIs.

On macOS the same seam exists, filled by different tools. Local subnets come
from the stdlib `net.Interfaces()` instead of `GetAdaptersAddresses`; peer
MAC addresses are read from the ARP cache (the equivalent of running
`arp -a`) instead of `SendARP`; share enumeration shells out to `smbutil`
(argv list, never a shell string) instead of calling `NetShareEnum`; and
GUI-adjacent desktop actions go through `osascript` where `mstsc`/Explorer
would have been the Windows answer. The build tags keep the split clean:
each `internal/winutil` wrapper has a `_windows.go` implementation and a
`!windows` stub, so the engine and every test compile identically on both
OSes.

The one place the code touches raw memory is the `NetShareEnum` buffer walk
in `internal/winutil/share_windows.go`: Windows hands back a pointer to a
packed array of `ShareInfo1` structs plus a count, and the code steps
pointer-sized strides through it, reading `unsafe.Pointer` fields. Think
`struct.unpack_from` on a `ctypes` buffer — same risks (offset arithmetic,
alignment, lifetime), same discipline required. That is exactly why it is
isolated in `internal/winutil`, behind a typed Go API returning
`[]ShareInfo1`, with a non-Windows stub (`share_other.go`) so the rest of
the codebase — and every test — never sees the unsafe parts. Treat that
package like you would treat memory-unsafe code anywhere: change it rarely,
review it hard.

## The security model in plain terms

- **Fail-closed gate.** `internal/authz/authz.go` asks one question:
  does this range need confirmation? Yes if **any** of: outside your local
  subnets, not RFC 1918 private space, or broader than a `/22`. It's an OR,
  not an AND — any suspicious property triggers the gate. The GUI shows the
  notice in a confirmation dialog; the CLI requires `--i-am-authorized`.
  Refuse, and nothing is scanned at all (the CLI checks every target before
  the first packet).
- **Shares are name/type/remark only, on purpose.** The enumeration runs
  under your existing Windows token; LanCartographer never asks for
  credentials and never stores any. When you double-click a share and
  Explorer opens `\\host\share`, *Windows* re-authorizes the access with
  your token — the tool's listing told you the share exists, nothing more.
  "Access denied" from the host is displayed as-is, not worked around.
- **argv-only launches.** Right-click actions build `[]string` argument
  vectors — `explorer \\1.2.3.4\share`, `mstsc /v:1.2.3.4`, `ping -n 4 …` —
  and pass them to `exec.Command(name, args...)`, which (like Python's
  `subprocess.run` with a list) does not go through a shell. There is no
  string to inject into.
- **No telemetry.** No analytics, no crash reporting, no runtime OUI lookup.
  The vendor database is baked into the binary. The only packets the program
  sends are the scans you initiated.

## Where to tweak things

| You want to change… | Edit |
|---|---|
| Default port list (GUI) | `specPorts` in `internal/gui/actions.go:19` |
| Default port list (CLI) | `defaultPorts` in `internal/cli/cli.go:31` |
| Liveness / probe timeouts | defaults flow from `--timeout` (400 ms) in `internal/cli/cli.go:59`; the engine's 2× relationship is set in `probe.Options` |
| Discovery ports (liveness) | `DiscoveryPorts` in `internal/discover/liveness.go:15` |
| Column widths / minimums | `colFractions` / `colMins` in `internal/gui/table.go:24` — fractions must sum to 1.0 |
| OUI vendor data | update `internal/oui/data.csv`, then run `go generate ./internal/oui` (rewrites `oui_data.go`) |
| Icon / manifest / version resources | `tools/icongen`, `cmd/lancartographer/app.manifest`, `versioninfo.json`; `go generate ./...` rebuilds `resource.syso` |
| Authorization policy | `NeedsConfirmation` in `internal/authz/authz.go` |
| Dial concurrency cap | `DefaultConcurrency` / `MaxConcurrency` in `internal/probe/limiter.go:9` |

## Reading the tests

The suite runs hermetically — safe on any network, no live LAN traffic:

- **Loopback fixtures.** Where a test needs a real answering socket, it
  starts an HTTP server on `127.0.0.1:18080` (e.g.
  `internal/cli/cli_test.go`). Loopback is in-process, deterministic, and
  touches no interface but your own.
- **TEST-NET-1 for the dead cases.** Negative tests use `192.0.2.x`
  addresses — RFC 5737's documentation range, guaranteed unroutable. Asking
  `SendARP` for the MAC of `192.0.2.1` must fail; if it ever "succeeds,"
  something is very wrong. It's the networking equivalent of
  `example.com`: reserved, never live.
- **Fakes at the seam.** Both the engine and the CLI define a `deps` struct
  — a bundle of function fields binding every external effect (dial, ARP,
  share enumeration, name lookups, MAC lookup, OUI, cache I/O):

  ```go
  type deps struct {
      dial       discover.DialFunc
      arp        func(netip.Addr) bool
      scan       func(context.Context, netip.Addr, probe.Options, *probe.Limiter, func(model.Port))
      listShares func(context.Context, netip.Addr) ([]model.Share, error)
      // ...
  }
  ```

  Production wires the real functions; tests wire fakes. If you know Python's
  `unittest.mock.patch`, this is the same idea — but instead of monkeypatching
  after the fact, the seam is a designed-in constructor parameter. No test
  framework magic, no patching builtins; the compiler checks both sides.

  That's also why `context.Context` shows up in every signature: a test can
  cancel it and assert the engine unwinds, the same way you'd assert a
  runspace stops on token cancellation.

Run the suite exactly as CI would:

```bash
CGO_ENABLED=1 go test ./... -race
```

(`-race` is Go's race detector — the build tags make it cheap, and the
event/channel design is exactly what it exists to catch.)
