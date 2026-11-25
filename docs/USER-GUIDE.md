# LanCartographer User Guide

Use the native GUI for an authorized inventory, saved before/after sessions,
and selected evidence reports. The CLI shares the scan engine; its flags and
examples are in the [README](../README.md#cli-reference). Build and update steps
are in [README](../README.md#build-from-source) and [update-guide.md](update-guide.md).
The second half of this guide is a code tour for Python and PowerShell users.

## Scan and inspect

> Scan only networks you own or have explicit authorization to inventory. Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws.

1. Open the Windows executable or macOS app, read the first-run notice (if the
   app cannot save that you saw it, an error says so and it returns next
   launch), and
   select or enter an authorized IPv4 range. Set **TCP ports** to the services
   you want to inventory, then click **Scan** and confirm if the
   range requires it. **Stop** cancels an active scan; partial results are not
   evidence of complete coverage.
2. Select a host to inspect it; its whole row stays highlighted. The panel
   lists open ports first, with page titles where the HTTP probe found one,
   and each open port has **Check…**, which fills in
   [Check Service](#check-a-service) without contacting anything. Closed and
   unanswered ports are summarized below them. A dim `—` means the scan did
   not learn that value. Right-click for available Browser, Finder/Explorer,
   Ping and Wake-on-LAN actions; Remote Desktop is Windows-only. These actions
   are explicit and may cause network traffic or open another application.
   The status bar shows `Ready`, `Scanning · 3s`, the duration of a finished
   scan, or `Stopped · 2s`; the progress bar appears only while scanning.
3. Use **File → Export…** for a raw inventory in CSV or JSON. Enter an absolute
   path ending in `.csv` or `.json` and confirm replacement if it exists. A
   failed write leaves the prior export intact. **File → Save Favorites** and
   **Load Favorites** keep a host list for later viewing. Neither contains the coverage metadata needed
   for session comparison; loading favorites also clears the current scan's
   session snapshot and is blocked while a scan is running.

### Choose ports for a development service

Enter comma-separated port numbers from 1 to 65535, such as `3000,5432,8080`
for a development web server, PostgreSQL and an alternate HTTP service.
**Default ports** restores the standard inventory list. Empty fields, ranges
such as `3000-3010` and non-numeric values are rejected before scanning; the
previous inventory stays visible. The choice lasts for the current app run.

The field chooses the ports whose states appear in the inventory. Discovery
still uses its usual TCP ports plus your selected ports, followed by the
existing ARP/ICMP fallbacks; name and SMB enrichment are unchanged. A scan is
not limited to traffic on the listed ports. Port controls are locked while a
scan runs or waits for Internet Quality to stop. Saved sessions record the
chosen profile, so comparisons with different port lists are incompatible.

When an open custom port is identified as Web by the existing HTTP
probe, **Browser** opens that HTTP endpoint with its port number. Standard
web ports keep their existing preference. An open TCP port alone does not
enable Browser, and this is not a TLS or application-health check.

For a service on this computer, scan `127.0.0.1/32` with its listening port.
A successful loopback result does not establish LAN reachability: repeat from
an authorized LAN client against the server's LAN address to check that path.
See the [native verification record](verification/developer-ports-2026-11-13.md)
for the exercised workflow and platform limits.

## Save a baseline and compare a change

1. Finish an authorized GUI scan, then open **Sessions** from the toolbar or
   File menu. The finished scan is copied into **After**. Enter **After
   context** (a site/project/network name) and an optional **After label**.
   Click **Save after session…** and review the absolute file path. Nothing is
   saved automatically, and replacing an existing file requires confirmation.
2. Make your intended network or service change and scan the same range again.
   Return to the Sessions window and choose **Use current scan**; an already
   open window does not automatically replace its copy. Set the same context
   and save the new session. **Open before…** loads your saved baseline;
   **Open after…** loads a saved second session if needed. Opening a session
   does not alter the live inventory or send probes.
3. Read the compatibility result before selecting changes. Both sessions must
   be complete and have matching context, range, probe profile, application
   version and available source-interface/platform snapshots. After must not
   predate Before. The context is your explicit label, not detected network
   identity; keep it consistent only when the two scans belong together.
4. Check the relevant differences, or use **Select all**. Review **Selected
   evidence preview**, add optional notes and click **Export selected
   evidence…**. The plain-text file contains that preview: selected changes
   plus coverage context. **Clear selection** disables export. Reopening a
   session or changing its context resets the selection.

![Selected comparison evidence in the native macOS app](verification/sessions-2026-11-01/after.jpg)

The comparison is by IP address. A newly absent address is **not observed**,
not proven offline; an empty field is **not recorded**, not proof of removal.
A host missing from one scan does not imply its individual ports closed.
One exception: when a single hardware address moved to an address unseen in
the other scan, the pair is one **Address (matched by MAC)** difference and
its ports are compared. Private (randomized) and repeated MACs never pair.
Names can come from the MAC-keyed cache. The source snapshot records local
interfaces, including other active interfaces; sockets still follow OS routes.
Routes and DNS configuration are not captured, and addresses are not permanent
device identities. Session JSON is editable, not authenticated evidence.

### What is saved and what is shared

Full session JSON includes the unredacted inventory, target, context/label,
application version, effective ports/timeouts/concurrency/methods, local source
addresses/platform, timestamps, completion state and progress. Cancelled,
failed and incomplete scans may be saved, but cannot form a completed comparison.
Files use schema version 1, with an 8 MiB and 65,536-host limit; unsupported or
inconsistent files are rejected, including a certificate expiry that is not a
scan-recorded UTC date on an open port. Favorites and CLI JSON cannot be imported as
sessions. Session save/compare/export is currently GUI-only.

**Redact identifiers and discovered text** is on by default for evidence
reports. It withholds site/scope/source identifiers and replaces selected
addresses and discovered values with aliases local to that report. It retains
timestamps, counts, port numbers, numeric probe settings and certificate
expiry dates and trust; this is not anonymization. The comparison list and saved JSON remain unredacted. Turning
redaction off includes selected discovered values and source/site metadata.
Notes are included as entered (with control characters sanitized); inspect
notes and screenshots before sharing. Reports are plain text with no remote
assets or active HTML. See [SECURITY.md](../SECURITY.md#saved-sessions-and-evidence).

Default storage is `%AppData%\LanCartographer\sessions\` on Windows and
`~/Library/Application Support/LanCartographer/sessions/` on macOS. An absolute
`APPDATA` override changes the per-user root; each save also lets you choose
another absolute path. Files are written through a temporary file and rename;
new directories/files use modes `0700`/`0600` on macOS. Back up files you want
to keep, including saves outside the default folder. There is no autosave,
background monitoring, automatic upload or session migration.

### When comparison or export is unavailable

| Message or condition | What to do |
|---|---|
| No finished scan | Finish or stop a GUI scan, then choose **Use current scan**, or open saved sessions. Favorites are not a scan. |
| Missing site/context | Enter the real site/project context in After, then save. Before retains the context in its file. |
| Incomplete scan | Keep the partial session if useful, then run a fresh authorized scan to completion for comparison. |
| Scope/profile/source mismatch | Read the stated reason. A changed VPN, local address, interface or platform can change the source snapshot. Take a new baseline appropriate to the investigation; do not relabel unrelated scans just to enable export. |
| After predates Before | Reopen the files on the correct sides. |
| Zero differences or none selected | Export requires at least one selected difference. A compatible pair with no differences has nothing to select. |
| Unsupported/invalid file | Use a saved session from a compatible build, within the file limits. Raw inventory JSON is not a session. |
| Session over 8 MiB or 65,536 hosts | The file is rejected and the current comparison and selection stay available. Use a session within both limits. |
| Notes too long | Keep report notes within 2,000 UTF-8 bytes. |

## Inspect local connections on macOS

Open **Tools → Connections…** and select **Refresh** for a local socket
snapshot. Choose **Connected**, **Listening**, or **UDP** to narrow the table.
Filter by process, PID, address or port. A numeric IP matches either endpoint
exactly. Select a column heading to sort, and select a row for its details.

- A wildcard bind appears as `*`. A UDP bind with no recorded peer shows a dash.
- **Show in inventory** selects an exact IPv4 peer match from the current
  completed scan. The details show the scan scope and finish time. From an
  inventory row, **Filter connections to this address…** fills the filter.
- A partial or failed refresh keeps the previous sample time when there is
  a prior display. macOS and account permissions limit what is visible.

The view reads socket metadata, without connecting to peers. The snapshot
stays in memory. **Copy selected row** puts its process and endpoint details
on the clipboard. Review these details before sharing them.

## Inspect routes and resolver configuration on macOS

Open **Tools → Route & DNS…**. In **Route**, enter a numeric IPv4 or IPv6
address and select **Inspect route**. A scoped IPv6 address uses a current
interface name, for example `fe80::1%en0` when `en0` exists. The displayed
route describes the selected interface and gateway when present. It gives
no measurement of reachability.

In **Resolver configuration**, select **Read resolver configuration** and
expand **Configured resolver scopes**. These settings describe configured
resolvers, including per-interface scopes. For observed answers from the
system resolver, enter a name in **Check Service** and select **Resolve**.
Both Route & DNS actions are local reads. Results are cleared when you
close the window.

## Check a service

Use **Tools → Check Service…** on Windows or macOS to answer "can this
computer reach my NAS, printer or web app, and where does it fail?" You can
also right-click a host in the inventory and choose **Check service…**. That
fills in the host's address and its most likely web port; filling in the form
sends nothing.

1. Enter a host name or IP address and a port. Choose **TCP** (connect only),
   **TLS** (connect and verify the certificate), **HTTP** (one `GET /`) or
   **HTTPS** (verified TLS, then one `GET /`). Common ports suggest a protocol
   until you choose one yourself.
2. For a name, select **Resolve**. The lookup uses this computer's resolver:
   the hosts file, the configured DNS servers or, for `.local` names on macOS,
   multicast DNS on the local network. Pick one of the returned addresses (at
   most 16 are shown).
3. Select **Check…** or press Return. The confirmation names the exact address,
   port and traffic, checks the current local subnets for its scope warning,
   and repeats the authorization notice. **No** sends nothing.

![Check Service with a resolved name, a connection and an HTTP answer](verification/check-service/resolve-http-light.jpg)

| Stage | Passed means | Common failures and what to try |
| --- | --- | --- |
| Resolve | The name returned at least one address. | *Name not found*: check spelling, the search domain, or whether this network's DNS knows the name. *Did not answer within 5 s*: the resolver is unreachable. |
| Connect | A TCP connection opened to the chosen address. The source address shows which local address this computer used. | *Refused*: nothing listens on that port, or a firewall rejected it. *No answer within 3 s*: the host is offline, a firewall drops the traffic, or the address is wrong. *No route*: this computer has no path to that network. |
| TLS | The certificate is trusted by this computer and valid for the name you entered. Shows the TLS version, certificate names, issuer and expiry. Expiry within 14 days is a warning. | *Not trusted*: self-signed and private-CA certificates fail; the app does not bypass verification. *Not valid for*: the certificate names a different host. *Expired* or *not valid until*: renew it, or check this computer's clock. *Did not answer with TLS*: try HTTP or TCP. |
| HTTP | The service answered with an HTTP status. Shows the `Server` header and page title. A redirect is shown and not followed. 401 or 403 means the service answered and wants sign-in. A 5xx status is a warning. | *No HTTP response within 1 s*, or *not HTTP*: the port may run another protocol or need TLS. |

What a check does and does not do:

- It opens one TCP connection to the one address you confirmed, without a
  proxy. It never looks the name up again, so a DNS answer that changes after
  you confirm cannot redirect the check.
- TLS verifies against this computer's trust store using the name you entered
  (also sent as SNI). HTTP sends that name as `Host`. The HTTP stage keeps the
  inventory limits: one second, 32 KiB of body and no redirects. Response
  headers are capped at 32 KiB.
- IPv4 and IPv6 addresses work. A link-local IPv6 address needs its
  interface, for example `fe80::1%en0` on macOS or `fe80::1%12` (the interface
  index) on Windows. Enter internationalized names in their
  `xn--` form.
- A result is an observation from this computer at the time shown. A failure
  does not prove the server or a firewall is at fault, and a success does not
  show what another client, VPN or container sees.
- One check runs at a time. **Stop** or closing the window cancels it and
  closes the connection. A check is independent of inventory scans.
- Results stay in the window. They are not saved in sessions, exports or
  logs. **Copy results** puts the host, address and stage text on the
  clipboard only when you select it.

## Measure Internet Quality on macOS

1. Choose **Tools → Internet Quality…**.
2. Choose **Run Apple test…** and confirm the Apple traffic and data-use
   disclosure. The test can saturate the connection. Use an unmetered link.
3. Read download and upload capacity in decimal Mbps, idle latency in
   milliseconds, and responsiveness under load in round trips per minute
   (RPM). Lower idle latency and higher RPM indicate a more responsive path.
   The result shows the observed interface when Apple reports it.

The window shows elapsed time during the measurement. Use **Stop** or close
the window to cancel. The Apple test has a 45-second time limit and can use
substantial data. Apple tests and Wi-Fi captures run one at a time and are
unavailable during an inventory scan. Starting an authorized scan ends the
current measurement before the scan starts.

If Apple returns an error, check that the Mac can reach the Internet and
try again. **Open Cloudflare in browser…** shows a separate disclosure and
hands the test to your default browser. Cloudflare receives your public IP,
shares anonymized measurement data and says a run can use up to 200 MB.
The browser manages its own results. No tools need to be installed.
[Apple Network Quality](https://support.apple.com/en-us/101942),
[Cloudflare Speed Test](https://speed.cloudflare.com/).

### Compare Wi-Fi before and after a change

1. Choose **Tools → Internet Quality… → Wi-Fi** and select **Capture baseline**.
2. Make one change, such as moving the Mac closer to the access point. Stay
   on the same Wi-Fi network and select **Capture after**.
3. Compare the two samples. Use **Capture after** again to repeat the follow-up
   measurement or **Replace baseline** to start over.

Each sample reads link metrics from macOS and sends up to five ICMP echo
requests to the default gateway through the Wi-Fi interface. This is a
read-only diagnostic. Results stay in the window until you close it.

| Reading | How to interpret it |
| --- | --- |
| Signal and signal to noise | Signal closer to zero in dBm is stronger. Higher signal to noise in dB means more separation from the background noise. |
| Band, channel and transmit link rate | The rate describes the negotiated Wi-Fi link. Use the Internet tab to measure internet throughput. |
| Gateway replies and average round trip | Check the reply count alongside the average. Some gateways ignore or limit ICMP replies. With zero replies, there is no round-trip average. |
| Signal and latency changes | Signal deltas require matching interfaces and gateway addresses. Latency deltas also require a usable average in each sample. Verify that you stayed on the same Wi-Fi network because two networks can use the same gateway address. |

Repeat a surprising result before drawing a conclusion. Scroll below the
samples to see the deltas if the bottom of the comparison is out of view.
[View the captured comparison](verification/wifi-diagnostics/after.jpg).

## Code tour for Python and PowerShell users

The rest of this guide connects Go concepts to familiar Python and PowerShell
ideas, then follows the actual scan and session boundaries.

## Analogy map

| Go construct | You know it as |
|---|---|
| `go.mod` / `go.sum` | `go.mod` declares the module, Go version and dependency requirements; `go.sum` verifies downloaded module content with checksums, rather than acting as a complete lockfile. The module path is `github.com/local/lancartographer`, and `internal/` packages can only be imported by this module — like a `src/` layout with an enforced boundary. |
| goroutine (`go func()`) | A thread or PowerShell runspace that costs almost nothing to start. Thousands, not tens. `go f()` is roughly `Start-ThreadJob { f }` with microsecond overhead and no job object to clean up. |
| channel (`chan Event`) | `System.Collections.Queue` with blocking handoff built in. Sending blocks when it's full; receiving blocks when it's empty. The compiler and runtime own the locks — you never write them. |
| `context.Context` | A `CancellationToken`. Passed through the scan to cancel dials and stop waiting for work; OS calls may have separate timeout/cancellation limits. |
| `struct` | A Python `@dataclass` or a `PSObject` with typed fields and no ceremony: `type Host struct { IP netip.Addr; Name string; ... }`. |
| `interface` | Python duck typing, but declared and checked at compile time. A function that takes `io.Writer` accepts anything with a `Write` method — like a protocol/ABC, enforced before runtime. |
| struct embedding | Mixin / inheritance-lite. Embedding `sync.Mutex` in a struct promotes its methods — the Go idiom for "this struct has a lock," comparable to `$this.Lock()` conventions you'd hand-roll. |
| pointer (`*Host`) | Passing a list by reference instead of a copy. Go is pass-by-value everywhere; a pointer is how a function mutates the caller's struct. Most of this codebase passes structs by value unless mutation is the point. |
| `cgo` | `ctypes` / `CDLL` — calling C from Go. This project avoids cgo for OS APIs except the macOS Wi-Fi diagnostics (CoreWLAN); see "Talking to the OS without .NET" below. |
| single compiled exe | PyInstaller output, but first-class: one static-ish binary, no interpreter, no venv, no DLL hell, ~instant startup. |

## What happens when you click Scan

The walk below is the actual call chain in the code, starting at
`internal/scan/engine.go` (`scan.Run`).

1. **Target selection.** The GUI asks `internal/discover` for your local
   usable subnets through Windows `GetAdaptersAddresses` or macOS
   `net.Interfaces()`, filtering virtual adapters and masking addresses to
   their on-link prefixes. **Scan** refreshes the suggestions and checks the
   range you entered against the current interfaces. If discovery fails, every
   range needs confirmation. The CLI uses `--range` or all discovered local
   physical subnets when omitted.

2. **Address expansion.** `discover.HostsIn` enumerates every usable host
   address in the prefix — for a `/24`, the 254 addresses between the network
   and broadcast addresses. The engine rejects IPv6 and anything broader than
   a `/12` before this step, so a typo can't materialize a billion-address
   list and OOM your own machine.

3. **Worker pool.** The addresses are pushed onto a `jobs` channel (a
   queue with blocking handoff), and **64 goroutines** consume them — a
   worker pool, like 64 runspaces pulling from a shared `BlockingCollection`.
   The pool sizes down automatically for small ranges. This is why a `/24`
   can overlap waiting on different hosts; elapsed time still depends on
   responses, timeouts and enrichment, not only the number of addresses.

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
   No liveness answer → the worker moves on; the address is not reported.
   This does not establish that a device is powered off.

5. **Port probe.** For each live host, `probe.ScanHost` dials every port in
   the port list with a TCP connect. The GUI and CLI share 15 defaults from
   `probe.DefaultPortsCSV`. `net.Dialer.DialContext` is the Go equivalent of
   `socket.create_connection()` with a timeout. Each result is classified
   `open`, `closed`, or `timeout` (`internal/model/model.go`). The per-dial
   budget is 800 ms by default (the CLI's `--timeout` sets a 400 ms
   liveness budget; the probe budget is 2×).

6. **Service identification.** Open ports get a label from a classifier
   (`internal/probe/classify.go`): 445 → `smb`, 3389 → `rdp`, and so on. If
   the label is `Web`, known web ports receive HTTP/HTTPS title enrichment
   (HTTPS on 443/8443). An `unknown` port receives an HTTP `GET /` and can
   be promoted to `Web`. Requests have a one-second/32-KiB cap, no redirects
   and verified TLS; titles are capped at 80 runes. On 443/8443 the same
   handshake also yields the certificate's expiry. Most LAN devices fail
   verification, so that date is then read from the failed handshake and
   shown as **not trusted**: a claim by the device, not a checked fact. Failure to obtain a title
   does not change an open port to closed.

7. **Share enumeration — only if 445 answered.** `probe.ListShares` calls
   `winutil.NetShareEnum`: the Windows implementation walks native
   `ShareInfo1` records; macOS runs `smbutil view` with a five-second cap.
   Both return name, type (`disk`/`print`/`ipc`) and remark. The host
   may answer "access denied" under your existing token/session;
   the engine synthesizes a single marker share with remark
   `"shares: access denied"` so the UI can say so honestly. Other failures
   (RPC refused, host vanished) degrade silently to an empty share list.

8. **Name resolution, three ways at once.** `resolveName` fires NetBIOS,
   mDNS, and reverse-DNS (PTR) lookups in three goroutines and takes the
   highest-priority non-empty answer: NetBIOS → mDNS → PTR. The engine gives
   NetBIOS/mDNS a 900 ms context and PTR 1.5 s; the UDP probes also have an
   800 ms internal deadline. A winning answer cancels lower-priority lookups, so a slow PTR need not hold up a NetBIOS
   result. Names can also come from the persistent MAC-keyed cache.

9. **Enrichment.** MAC address via Windows `SendARP` or the macOS ARP cache;
   vendor by looking the OUI prefix up in an **embedded** database (compiled
   into the binary from
   `internal/oui/oui_data.go` — no runtime download). On Windows, a failed
   MAC lookup while unelevated becomes the sentinel `"needs elevation"`; the
   GUI offers optional elevation. macOS leaves a missing MAC empty.

10. **Events stream to the GUI.** Scan observations reach the UI through
    one event channel: `EventPortFound` and
    `EventShareFound` stream live during probing, `EventHostFound` carries
    the single canonical, fully-enriched record per alive host,
    `EventProgress` drives the progress bar, and `EventDone` marks full
    completion. Cancellation returns a context error instead of a normal
    completion result. The GUI records status and progress separately, so a
    cancelled scan is not treated as complete even if progress reached 100%.

### Why events on a channel instead of a shared list?

Because of the GUI thread rule. Fyne, like WPF or WinForms, requires that UI
objects be touched from exactly one thread. In Go the marshaling primitive is
`fyne.Do(func(){ ... })` — functionally `Dispatcher.Invoke` in C# or
`$Dispatcher.Invoke()` in PowerShell WPF: "run this on the UI thread."

The engine runs on background goroutines and must never touch widgets
directly. So it sends immutable `Event` values down a channel; the GUI side
has one goroutine receiving them and forwarding each to `fyne.Do`, which is
the path used for live scan updates. Favorites and session controls also run
on the UI thread; keep worker code behind that boundary.

## Concurrency, in your terms

**The rate limiter is a semaphore.** `internal/probe/limiter.go` implements a
`Limiter` as a buffered channel of size *n* — think `SemaphoreSlim(n, n)`.
To run work you "park" a token (`l.sem <- struct{}{}`), do the dial, then
release it. The whole scan shares one limiter, created in `scan.Run`, for
discovery and selected-port work across the host workers:

```go
lim := probe.NewLimiter(opts.Concurrency) // default 128, hard cap 256
```

`NewLimiter` clamps: ask for 0 or negative → 128; ask for more than 256 →
256. The limiter bounds the concurrent work using it; it is not a
packets-per-second limit and does not cover every enrichment or external desktop action.

**Cancellation reaches in-flight TCP dials.** TCP dials take
the sweep's `context.Context`. Clicking Stop cancels that context; every
`DialContext` in flight can return, workers see `ctx.Err() != nil`
and exit instead of taking new jobs, and blocked channel sends bail out via
`select { case ch <- ev; case <-ctx.Done() }`. In PowerShell terms: one
`CancellationTokenSource.Cancel()` that every `Wait-Any` in the system
respects — including the ones queued behind the semaphore. This does not
mean every OS operation is interruptible. On macOS, Stop cancels the
`smbutil` child and waits for it to exit. On Windows, the app waits up to
five seconds for each enumeration and Stop ends that wait early. An in-flight
`NetShareEnum` call keeps its slot until Windows returns. If all 64 slots
are occupied across scans, new hosts skip share enumeration.

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
The build needs a C compiler for Fyne (the GUI library), not for Windows APIs.
The one OS-API exception is macOS Wi-Fi diagnostics:
`internal/gui/wifi_quality_darwin.go` reads link metrics from CoreWLAN through
cgo; other platforms get the `wifi_quality_other.go` stub.

On macOS the same seam exists, filled by different tools. Local subnets come
from the stdlib `net.Interfaces()` instead of `GetAdaptersAddresses`; peer
MAC addresses are read from the ARP cache (the equivalent of running
`arp -a`) instead of `SendARP`; share enumeration shells out to `smbutil`
(argv list, never a shell string) instead of calling `NetShareEnum`; and
Browser/Finder actions use `open`, Ping uses Terminal via `osascript`, and
the Windows-only Remote Desktop action is unavailable. Build tags keep the
split clean:
`internal/winutil` provides `_windows.go`, `_darwin.go` and other-platform
fallbacks. Shared APIs let the engine compile across platforms; native behavior
and OS-specific tests still need their respective platforms.

One raw-memory boundary is the `NetShareEnum` buffer walk
in `internal/winutil/share_windows.go`: Windows hands back a pointer to a
packed array of `ShareInfo1` structs plus a count, and the code steps
by the size of each struct, reading through `unsafe.Pointer`. Think
`struct.unpack_from` on a `ctypes` buffer — same risks (offset arithmetic,
alignment, lifetime), same discipline required. That is exactly why it is
isolated in `internal/winutil`, behind a typed Go API returning
`[]ShareInfo1`, with a macOS implementation and other-platform stub
(`share_other.go`), so ordinary callers do not handle the native buffer. Treat
that package like you would treat memory-unsafe code anywhere: change it rarely,
review it hard.

## The security model in plain terms

- **Fail-closed gate.** `internal/authz/authz.go` asks one question:
  does this range need confirmation? Yes if **any** of: outside your local
  subnets, not RFC 1918 private space, or broader than a `/22`. It's an OR,
  not an AND — any suspicious property triggers the gate. The GUI shows the
  notice in a confirmation dialog; the CLI requires `--i-am-authorized`.
  Refuse, and nothing is scanned at all (the CLI checks every target before
  the first packet).
- **Shares use existing access.** Enumeration uses the current Windows token
  or macOS SMB session. The app does not collect or store passwords; Finder or
  Explorer handles authentication when opening a share. An access-denied
  marker is reported without retrying alternate credentials.
- **Bounded desktop launches.** Actions build argument arrays. Windows Browser
  and Ping use `cmd /c start`; macOS Ping uses a Terminal AppleScript. Their
  targets are constructed from parsed IP literals and validated ports; macOS
  share names are URL-escaped. Passing argv does not itself eliminate every
  shell boundary: preserve the target builders when changing these actions.
- **No telemetry.** No analytics, crash reporting or runtime OUI download.
  Scans include DNS/name-resolution traffic that can reach off-LAN resolvers.
  Explicit Ping, Wake-on-LAN and desktop actions can also send traffic. Saved
  session open/compare/export adds no probes or uploads.
- **Check Service contacts one confirmed address.** `internal/servicecheck`
  dials only the address the operator confirmed, never re-resolves the name,
  uses no HTTP transport (so no proxy or redirect can apply) and never skips
  TLS verification. Text from the server is reduced to printable single-line
  text before display.
- **Local evidence has privacy limits.** Full session JSON is unredacted;
  selected reports default to redaction. Notes and screenshots need separate
  review. See the session workflow above and [SECURITY.md](../SECURITY.md).

## Session ownership in the code

`internal/gui/gui.go` starts a session with the scan's actual options and tracks
progress/completion on the UI thread. `endSession` in `internal/gui/sessions.go`
clones the finished inventory and records complete/cancelled/failed/incomplete
status. Each Sessions window works on its own copies; importing a file does
not replace the active table. Loading favorites invalidates the current scan
snapshot so an old coverage claim cannot attach to a different host list.

`internal/session` owns schema validation, bounded reads, explicit file writes,
comparison and plain-text report generation. It reads local interface metadata
when capturing a scan, but does not invoke discovery or network probes. The
GUI freezes the displayed report before asking for an export path, so changes
made while a save dialog is open do not change the file being approved.

## Where to tweak things

| You want to change… | Edit |
|---|---|
| Default port list (GUI and CLI) | `DefaultPortsCSV` in `internal/probe/ports.go` |
| Liveness / probe timeouts | defaults flow from `--timeout` (400 ms) in `internal/cli/cli.go`; GUI/CLI callers set `probe.Options.ProbeTimeout` to 2× the discovery timeout |
| Discovery ports (liveness) | `DiscoveryPorts` in `internal/discover/liveness.go` |
| Column widths / minimums | `colFractions` / `colMins` in `internal/gui/table.go` — fractions must sum to 1.0 |
| OUI vendor data | update `internal/oui/data.csv`, then run `go generate ./internal/oui` (rewrites `oui_data.go`) |
| Icon / manifest / version resources | `build/icongen`, `cmd/lancartographer/app.manifest`, `versioninfo.json`; `go generate ./...` rebuilds `resource.syso` on Windows |
| Session schema, comparison and redaction | `internal/session`; GUI ownership and dialogs in `internal/gui/sessions.go` |
| Authorization policy | `NeedsConfirmation` in `internal/authz/authz.go` |
| Dial concurrency cap | `DefaultConcurrency` / `MaxConcurrency` in `internal/probe/limiter.go` |

## Reading the tests

Checks use a mixture of fakes, loopback listeners and real network/OS calls.
Do not assume the whole suite is hermetic:

- **Loopback fixtures.** Where a test needs a real answering socket, it
  starts an HTTP server on `127.0.0.1:18080` (e.g.
  `internal/cli/cli_test.go`). Loopback is in-process, deterministic, and
  uses your own host. Full CLI scans may also invoke name resolution; a
  loopback target alone does not isolate every side effect. Environments
  that forbid binding loopback sockets, such as some agent sandboxes, fail
  these tests with `bind: operation not permitted`; that is the environment,
  not a code defect.
- **Negative network cases.** Some probe tests call real dial, MAC or name
  lookup functions with `192.0.2.1`. A documentation address is not a fake
  transport and does not guarantee that no packets or DNS queries are sent.
  Inspect the selected check and its dependencies before running it.
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

For a local change, run the affected behavior and focused checks. For example,
the session parser/comparison/report checks use synthetic records and temporary
files:

```bash
go test -race ./internal/session -run '^TestSession' -count=1
```

This checks library behavior, not native dialogs. GUI changes also need actual
native interaction; see the [session verification record](verification/sessions-2026-11-01.md)
for the exercised workflow and limits, and the
[size-limit record](verification/session-size-limits-2026-11-20.md) for the
8 MiB and 65,536-host acceptance. Native Windows desktop acceptance is pending in
[issue #80](https://github.com/cgfixit/LanCartographer/issues/80). Fyne builds need CGO and the platform
C toolchain. Use `bash build/build.sh` for packaging, especially on macOS where
the checked-in Windows resource must be moved aside and restored.

Hosted [CI](../.github/workflows/ci.yml) runs the broad Windows and native
arm64/Intel macOS build, vet and race checks, plus packaged CLI smoke checks on
macOS; `bash build/ci-gate.sh` runs the same gate locally on Windows or macOS.
[Static checks](../.github/workflows/static-checks.yml) add govulncheck,
actionlint and zizmor on every PR and weekly. A cross-compile or packaged CLI pass does not verify native GUI behavior.
For dependency/toolchain updates, follow
[dependency maintenance](../CONTRIBUTING.md#dependency-maintenance) and record
new results against the actual rebuilt artifact.
