# LanCartographer network workbench roadmap

LanCartographer helps people inventory a network and verify what changed after
maintenance. The next step is to connect that workflow to what the operator's
own computer is doing. Start on macOS, then qualify the same workflow on Windows.
**NetClawViewer** is the working name for a small **Connections…** window inside
LanCartographer. All phases below are proposals.

## What exists today

This assessment covers `main` at `3c64e48` (November 7, 2026). Saved sessions
landed in [#23](https://github.com/cgfixit/LanCartographer/pull/23). The project
had GitHub Actions preview packages and no tagged GitHub Release at review time.
The macOS arm64 preview passed the owned loopback smoke through both packaged
CLI entry points on macOS 27.2. The session workflow has separate native
verification at the revision recorded there.
[Preview build](https://github.com/cgfixit/LanCartographer/actions/runs/42187544799),
[build instructions](../README.md#build-from-source),
[session verification](verification/sessions-2026-11-01.md).

| Workflow | What the current source supports | Where the observation stops |
|---|---|---|
| Inventory a LAN | GUI scans a selected IPv4 CIDR. The CLI accepts a CIDR or finds local physical subnets. Both use TCP discovery and port probes, ARP fallback and, on macOS, a bounded ICMP fallback. | Discovery can miss silent devices. The scan engine accepts IPv4 only. It does not continuously monitor the network. |
| Inspect a host | View per-port states, service labels and bounded web titles, SMB shares, names, MAC and vendor where available. Use Browser, Finder/Explorer, Ping and Wake-on-LAN actions. Remote Desktop launches the Windows client. | Names can come from cache. MAC and shares depend on platform and visibility. Reverse mDNS naming does not browse Bonjour service instances. |
| Keep and compare results | Export CSV/JSON in the GUI or JSON in the CLI. The GUI can reload favorites, explicitly save a finished scan, compare compatible complete sessions and export selected, redacted evidence. | Favorites and raw inventory lack coverage metadata. Differences describe observations at an IP. Full session JSON is unredacted. |
| Inspect this computer | The scan records a local interface snapshot in GUI sessions and provides host shortcuts. | There is no process/socket view, structured route/DNS diagnosis, service watch, or capture of which application used a remote endpoint. |

The boundaries are visible in the [scan engine](../internal/scan/engine.go),
[GUI actions](../internal/gui/menu.go), [CLI](../internal/cli/cli.go),
[mDNS probe](../internal/probe/mdns.go), and
[session comparison](../internal/session/compare.go).

## What to borrow from the workflow

| Tool | What it already does | Implication |
|---|---|---|
| [Advanced IP Scanner](https://www.advanced-ip-scanner.com/) | Portable Windows LAN discovery, MACs, shared folders, CSV, RDP/Radmin and remote power controls. | Discovery and quick actions are established. LanCartographer's current advantage for a Mac operator is inventory paired with coverage-aware comparison and selected evidence. |
| [Microsoft Sysinternals TCPView](https://learn.microsoft.com/en-us/sysinternals/downloads/tcpview) | Windows TCP/UDP endpoint and owner visibility, periodic refresh, change highlighting, CLI and established TCP connection closure. | Borrow the glanceable endpoint table and refresh cues. Keep the initial viewer read-only and connect it to the inventory where the evidence permits. |
| [Netiquette](https://objective-see.org/products/netiquette.html) and [Sloth](https://sveinbjorn.org/sloth) | macOS already has process/socket inspectors. Netiquette offers filters, refresh and JSON. Sloth also explores open files and other descriptors. | A socket list alone adds little differentiation. Make it easy to move from a local observation to a relevant LAN record and a reproducible check. |

The intended workflow is **see the endpoint → inspect the peer → verify the
service → share selected evidence**. For example, after a NAS move, open
Connections to see whether a local application is connected to the old or new
address. Open the matching inventory row, then explicitly test the new service
and compare it with the maintenance baseline. A socket observation alone cannot
explain why a connection was made or establish that another computer can reach
the service. The comparison above uses product documentation.

## Phases

### 0. Qualify the baseline and socket source

- Carry forward the open native and size-limit acceptance in
  [#24](https://github.com/cgfixit/LanCartographer/issues/24). Record the actual
  package, platform, visibility limits, fixture and results. Keep baseline
  maintenance separate from the viewer. The current review tracks failed
  inventory replacement and cancellation of in-flight SMB enumeration in
  [#28](https://github.com/cgfixit/LanCartographer/issues/28) and
  [#29](https://github.com/cgfixit/LanCartographer/issues/29).
- Try the bundled `/usr/sbin/lsof` on the minimum and a current macOS version
  from a packaged app running as the current user. Use numeric endpoints and
  machine fields (`-nP -iTCP -iUDP -F0pcftPnT`). Measure latency and output
  size on idle and busy hosts. Verify own-process visibility, wildcard binds,
  scoped IPv6, malformed output, and the behavior for inaccessible owners and
  empty results. Prefer this source if its measured coverage and cost suffice.
  [lsof field format](https://github.com/lsof-org/lsof/blob/master/docs/manpage.md)
- On macOS 27.2 arm64, the command returned two TCP listeners, four established
  endpoints and two UDP binds across IPv4 and IPv6 without elevation, using
  `-a -p <fixture PID>` to restrict the sample. This validates fields for an
  owned fixture. System-wide visibility,
  polling cost, minimum-OS behavior and app integration still need measurement.

**Exit criterion.** Capture the owned fixtures from the packaged app, document
the visibility it can honestly promise, and measure a refresh budget before
choosing the default cadence. If visibility is limited, name the limitation
in the UI.

### 1. Ship the macOS Connections window

- Add one toolbar or Tools menu entry that opens a single modeless window
  alongside the scan. Show process name/PID when available, protocol, numeric
  local and remote endpoint, TCP state and sample age. Include **Listening**,
  **Connected**, **UDP** and text filters, plus **Refresh** and **Pause**.
  Preserve selection across refreshes when the same row remains present.
  Show wildcard binds and UDP rows without a peer explicitly.
- Take one snapshot on open. Start with manual refresh, then enable opt-in
  periodic sampling at a measured interval (try three seconds first). Allow one
  collection at a time. Use a deadline, byte/row limits and cancellable
  subprocess execution. Stop on pause/close, discard results from cancelled
  generations and update Fyne on its UI thread. Keep samples in memory and
  distinguish a successful empty result from stale, partial or failed data.
- Highlight newly observed, changed and no-longer-observed rows between
  successful samples. Suppress removal cues across incomplete samples.
  Short-lived sockets can be missed between samples, and reused PID/socket
  tuples are not durable identities. Keep address resolution off by default.
  The window adds no packet capture, connection termination or elevation path.

**Exit criterion.** With owned TCP/UDP loopback fixtures, show the expected
IPv4/IPv6 rows, a connection change and an honest empty/failure state. Verify
filtering, selection, narrow/light/dark rendering, concurrent scan interaction,
and that a close or pause during collection leaves no polling or child process.

### 2. Link Connections to inventory and add focused tools

- From a connection, offer **Show in inventory** when the numeric peer has an
  exact IP match in the current authorized IPv4 scan, with the inventory's
  context and observation time visible. From a host, offer **Filter
  connections to this address**. Both are local view operations. Show ambiguity
  when the active site or network changes. No MAC/name inference or implicit
  scan, even when a public peer appears in Connections.
- Add a small **Tools** panel. Start with **Route to address** (`/sbin/route
  -n get` with a validated literal) and **Resolver configuration**
  (`/usr/sbin/scutil --dns`) as local reads. Show command, exit state and
  bounded output. Add a deliberate DNS check with resolver provenance and
  reuse the existing Ping action where suitable. Evaluate `dig` as an explicit
  query shortcut and `dns-sd` as a time-boxed discovery diagnostic later.
  A `dig` result does not necessarily reflect the system resolver's selection.
- Add one **Check service…** workflow, starting with a single authorized
  endpoint and structured TCP, TLS and HTTP outcomes. Preview the target before
  any active check. For a hostname, authorize the explicit lookup, show the
  results, authorize the concrete destination and connect to that approved
  address. Keep the original DNS name for TLS validation. Respect the scan
  policy and bounded HTTP behavior. Record the stage, elapsed time and
  observed result. Permit a redacted preview of selected observations without
  changing the existing scan session schema.

**Exit criterion.** Reproduce a listener bound to loopback and show why its
bind does not establish LAN reachability. Use an owned endpoint to distinguish
DNS failure, TCP refusal, certificate failure and HTTP response. Verify that
opening/filtering the views does not probe, and that declining a check does
not connect to its destination. Show that output limits and cancellation
hold even when a child stalls or output contains control characters.

### 3. Bring the workflow to Windows

- Keep the same view and snapshot semantics. Evaluate IPv4/IPv6 owner-PID
  tables from `GetExtendedTcpTable` and `GetExtendedUdpTable`; cap buffer
  growth and retries, and treat process-name lookup as best effort. Render
  UDP as local binds when the provider supplies no remote endpoint.
  Stay within the existing `asInvoker` contract. Port the useful diagnostics
  after defining their Windows inputs and outputs.
  [Microsoft TCP table](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedtcptable),
  [Microsoft UDP table](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedudptable)
- Run the packaged app on Windows 10/11 as a standard user. Verify owned
  sockets, a process that exits between enumeration and name lookup,
  access-denied attribution, refresh cancellation, and the inventory bridge.
  Keep OS-specific missing fields visible in the shared UI.

**Exit criterion.** The same inspect → verify workflow works in a Windows
fixture. Coverage and permission differences are documented per platform.

### 4. Add discovery and limited monitoring where they earn a place

- Browse a small set of Bonjour/DNS-SD service types on demand, starting with
  printers and SMB. Time-box browsing, cap records and TXT data, and show the
  service instance, interface and advertised target. Keep advertised,
  reachable and authenticated outcomes separate. Start IPv6 inventory with
  selected or discovered addresses and interface zones, with explicit scope
  policy before probing. Exercise local-network permission denial in the
  actual macOS package. [Apple local network privacy](https://developer.apple.com/documentation/technotes/tn3179-understanding-local-network-privacy)
- Add a foreground watch for one explicitly authorized service, with a
  bounded run time, conservative interval, visible last result and Stop.
  Compare a small set of observations or export selected redacted evidence.
  Keep the TCP scan CLI stable. Add a separate one-shot CLI check only once
  the outcome format and exit codes are stable.

**Exit criterion.** Discover an owned service, distinguish an advertisement
from reachability, and reproduce a service loss and recovery with a bounded
watch. Permission denial, timeout and cancellation stay distinct.

## Boundaries for every phase

Scan only networks you own or have explicit authorization to inventory. Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws.

Keep local reads separate from actions that generate traffic. The app owns the
allowlist of executables and argument layouts. No shell strings, arbitrary
commands, installer, privileged helper, persistent monitoring daemon, runtime
downloads or telemetry. Treat process names, tool output and advertised names
as untrusted display text. Keep identifiers out of default reports and make
export explicit. Preserve the existing worker/dial budgets, cancellation,
TLS verification, one-second/32-KiB HTTP bounds and no redirects.

Keep [#22](https://github.com/cgfixit/LanCartographer/issues/22) as the wider
roadmap. Make each implementation phase a separately reviewable change with
native acceptance and synthetic data. Packet capture, traffic blocking and
remote administration are outside this plan.
