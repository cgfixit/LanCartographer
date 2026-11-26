# LanCartographer network workbench roadmap

LanCartographer helps people inventory a network and verify what changed after
maintenance. The next step is to connect that workflow to what the operator's
own computer is doing. Start on macOS, then qualify the same workflow on Windows.
**Connections…** is the macOS window for inspecting a local socket snapshot.
Connections, Route & DNS, Check Service and Internet Quality each have a
direct entry in the **Tools** menu.

## What exists today

Saved sessions support coverage-aware comparison and selected evidence.
Check Service verifies an explicitly approved endpoint. The macOS
Connections window shows a manual snapshot of visible local sockets and links
an exact IPv4 peer match to the current completed inventory.
[Session verification](verification/session-size-limits-2026-11-20.md),
[Check Service verification](verification/check-service-2026-11-15.md),
[Connections verification](verification/connections-2026-11-20.md).

| Workflow | What the current source supports | Where the observation stops |
|---|---|---|
| Inventory a LAN | GUI scans a selected IPv4 CIDR. The CLI accepts a CIDR or finds local physical subnets. Both use TCP discovery and port probes, ARP fallback and, on macOS, a bounded ICMP fallback. | Discovery can miss silent devices. The scan engine accepts IPv4 only. It does not continuously monitor the network. |
| Inspect a host | View per-port states, service labels and bounded web titles, SMB shares, names, MAC and vendor where available. Use Browser, Finder/Explorer, Ping and Wake-on-LAN actions. Remote Desktop launches the Windows client. | Names can come from cache. MAC and shares depend on platform and visibility. Reverse mDNS naming does not browse Bonjour service instances. |
| Keep and compare results | Export CSV/JSON in the GUI or JSON in the CLI. The GUI can reload favorites, explicitly save a finished scan, compare compatible complete sessions and export selected, redacted evidence. | Favorites and raw inventory lack coverage metadata. Differences describe observations at an IP. Full session JSON is unredacted. |
| Verify expected inventory | The CLI checks declared host absence and TCP port states over a completed scan. | Results describe observations from that run. A silent device can evade discovery. |
| Inspect this computer | Connections shows visible process and socket metadata. Route & DNS reads routing and resolver configuration. | Socket snapshots can miss short-lived or inaccessible processes. A route describes local selection and gives no reachability measurement. |

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

- Keep the [saved-session acceptance record](verification/session-size-limits-2026-11-20.md)
  distinct from socket qualification. Record the actual package, platform,
  visibility limits, fixture and results. Keep baseline
  maintenance separate from the viewer. Failed inventory replacement
  ([#28](https://github.com/cgfixit/LanCartographer/issues/28)) and
  cancellation of in-flight SMB enumeration
  ([#29](https://github.com/cgfixit/LanCartographer/issues/29)) were fixed by
  #31 and #32 (merged 2026-11-08); their native acceptance limits stay with
  those PRs.
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

**Tools → Connections…** opens a modeless window. **Refresh** reads one
bounded snapshot, and **Stop** cancels the helper. The table shows process,
PID, protocol, numeric endpoints and TCP state. **Listening**, **Connected**,
**UDP**, and the text filter work on the sample already in memory.

Periodic sampling and change highlighting remain follow-ups. Short-lived
sockets can disappear between samples. A reused PID or socket tuple can
describe a different observation.

**Exit criterion.** With owned TCP/UDP loopback fixtures, show the expected
IPv4/IPv6 rows, a connection change and an honest empty/failure state. Verify
filtering, selection, narrow/light/dark rendering, concurrent scan interaction,
and that a close or pause during collection leaves no polling or child process.

### 2. Link Connections to inventory and add focused tools

- **Show in inventory** selects an exact numeric IPv4 peer in the current
  completed scan and shows its scope and finish time. **Filter connections
  to this address** prefills the snapshot filter from a host row.
- **Tools → Route & DNS…** reads local routes for numeric destinations and
  macOS resolver configuration separately. **Resolve** in **Check Service**
  shows observed system lookup answers.
  [Native verification](verification/route-dns-2026-11-20.md)
- *(Shipped: Tools → Check Service…, `internal/servicecheck`.)* Add one
  **Check service…** workflow, starting with a single authorized
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

--

# issue #95 updated roadmap:

# LanCartographer consolidated network workbench roadmap

Reviewed 2026-11-26 against `origin/main` at [`3181efb508d75dac65c48dc254b7aba8dec5c985`](https://github.com/cgfixit/LanCartographer/commit/3181efb508d75dac65c48dc254b7aba8dec5c985).

This is a proposed replacement for `docs/network-workbench-roadmap.md`, consolidated with open issues #22, #40, #80, #89 and #90. It also adds five researched workflow candidates and carries forward later proposals that the earlier consolidation omitted. It is not an owner decision, an implementation authorization, or a record of repository changes. The review changed no branch, PR, release, source file or Cloudflare resource.

## Decision summary

Keep the product thesis narrow: **inventory an authorized network, verify what changed, diagnose a selected service, and share only the evidence the operator chose**. The local computer is another observation point, not a reason to turn the app into a general security suite.

Recommended delivery order:

1. Reconcile stale planning text and disposition the remaining #89 audit findings.
2. Finish the owner-approved macOS diagnostics wave in #40.
3. Run native Windows acceptance in #80, resolve version compatibility, then make the first tag decision.
4. Add the one-shot CLI service check after its existing evidence gate is met.
5. Refactor the connection row model and add Windows Connections.
6. Admit identity corrections, bounded Connections refresh, IPv6, DNS-SD, service watch and imports only against concrete operator tasks.
7. Add the new workflows in this order: network-context checks, manual socket diffs, cross-vantage exposure reconciliation, authorized external-exposure retests, and local integrity diffs.

The first release should not wait for phases 5 through 9. Those are later product options, not release criteria.

The five additions are differentiated combinations for this application. None is claimed to be an industry first. Vantage-aware tests, snapshot comparison, attack-surface discovery and persistence enumeration already exist elsewhere. The useful distinction is a small, local, manually operated workflow with explicit scope and no collector fleet, daemon or account.

## Evidence and status

### Current source baseline

The current main commit is the merge of PR #94, "Add CLI inventory expectations", dated 2026-11-25. The repository uses Go 1.27.4 and Fyne 2.8.2. It supports macOS 14 or later and Windows 10/11, with a shared GUI and CLI inventory engine.

Source facts below come from the named files at that commit. Existing verification records are cited as records, not rerun here.

### Verification limits

This review read source, issue bodies and comments, the earlier consolidated draft, and current external documentation. It did not execute the program:

- The review environment has no Go toolchain, so no build, test, vet, staticcheck or loopback CLI run was performed.
- No native macOS or Windows desktop was available. Existing native records remain separate evidence and were not treated as new test results.
- No real LAN, public domain, Cloudflare account or user data was accessed.
- Competitor claims come from official product documentation. No competitor application was hands-on tested.

### Shipped work that older plans still list as future work

| Item | Current evidence | Roadmap treatment |
|---|---|---|
| Same device across an address change, #82 | PR #85 merged 2026-11-20. `internal/session/compare.go` pairs a moved device only through a unique, universally administered MAC and still compares its ports. | Shipped. Remove from open work. |
| Certificate expiry on scanned TLS ports, #84 | PR #86 merged 2026-11-20. `model.Port` carries expiry and trust observations; session validation and comparison cover them. | Shipped. Remove from open work. |
| CLI inventory expectations, #83 | PR #94 merged 2026-11-25. `internal/cli/expectations.go` implements strict version-1 expectations, and `docs/verification/cli-expectations-2026-11-25.md` records verification. | Shipped. Keep only the separate one-shot service check as future work. |
| macOS SMB names containing spaces, audit finding #89-2 | PR #93 merged 2026-11-25. The latest #89 comment still calls #93 a draft, but main contains the merge. | Shipped. Refresh the issue status when the owner next edits it. |

## What exists today

| Workflow | Current source supports | Where the observation stops |
|---|---|---|
| Inventory a LAN | GUI scans a selected IPv4 CIDR. CLI accepts a CIDR or finds local physical subnets. Both use bounded TCP discovery and probes, ARP fallback and, on macOS, bounded ICMP fallback. | Discovery can miss silent devices. The engine rejects IPv6 and ranges broader than `/12`. It does not continuously monitor a network. |
| Inspect a host | Per-port states, service labels, bounded web titles, SMB shares, names, MAC and vendor where available. Browser, Finder/Explorer, Ping and Wake-on-LAN actions exist. Remote Desktop launches the Windows client. | Names can come from cache. MAC and shares depend on platform and visibility. Reverse mDNS naming does not browse Bonjour service instances. |
| Keep and compare results | GUI CSV/JSON export, CLI JSON, favorites, explicit saved sessions, compatible-session comparison and selected redacted evidence. Comparison covers address moves through qualified MAC evidence, ports, certificate observations and SMB shares. | Favorites and raw inventory lack coverage metadata. Missing addresses do not prove absence. Full session JSON is unredacted. |
| Verify expected inventory | CLI checks declared host absence and TCP port states over a completed scan. Strict JSON rejects duplicate and unknown fields. Expectations cannot widen scan scope. | Results describe observations from that run. Timeouts do not prove closed ports, and silent devices can evade discovery. |
| Diagnose one service | Check Service performs explicit DNS, TCP, verified TLS and bounded HTTP stages for one approved endpoint. It resolves a hostname once and checks the selected address without re-resolving it. | GUI only. One host observation does not establish what another machine, container, VM or remote client sees. |
| Inspect this computer | macOS Connections shows visible process and socket metadata. Route & DNS reads route selection and resolver configuration. Internet Quality includes the shipped Wi-Fi comparison. | Socket snapshots miss short-lived or inaccessible processes. A route describes local selection, not reachability. Windows does not yet have Connections or Route & DNS. |
| Share evidence | Operators select observations, preview redaction and export a report locally. | Redaction is not anonymity. Remaining context can identify a site. |

The main constraints are visible in [`internal/scan/engine.go`](https://github.com/cgfixit/LanCartographer/blob/3181efb508d75dac65c48dc254b7aba8dec5c985/internal/scan/engine.go), [`internal/servicecheck/servicecheck.go`](https://github.com/cgfixit/LanCartographer/blob/3181efb508d75dac65c48dc254b7aba8dec5c985/internal/servicecheck/servicecheck.go), [`internal/servicecheck/check.go`](https://github.com/cgfixit/LanCartographer/blob/3181efb508d75dac65c48dc254b7aba8dec5c985/internal/servicecheck/check.go), [`internal/session/session.go`](https://github.com/cgfixit/LanCartographer/blob/3181efb508d75dac65c48dc254b7aba8dec5c985/internal/session/session.go), [`internal/session/compare.go`](https://github.com/cgfixit/LanCartographer/blob/3181efb508d75dac65c48dc254b7aba8dec5c985/internal/session/compare.go), [`internal/macdiag/command.go`](https://github.com/cgfixit/LanCartographer/blob/3181efb508d75dac65c48dc254b7aba8dec5c985/internal/macdiag/command.go) and [`internal/cli/expectations.go`](https://github.com/cgfixit/LanCartographer/blob/3181efb508d75dac65c48dc254b7aba8dec5c985/internal/cli/expectations.go).

## Issue consolidation map

| Issue | Current state | Where its remaining work lives here |
|---|---|---|
| [#22](https://github.com/cgfixit/LanCartographer/issues/22), product roadmap | Open. Its shipped list and owner decisions remain useful, but its open list still contains shipped #82, #83 and #84. | Product direction, phases 3 through 5, later identity/import decisions, and the new candidate gates. |
| [#40](https://github.com/cgfixit/LanCartographer/issues/40), macOS diagnostics wave 2 | Open. Wi-Fi Link shipped through CoreWLAN. The owner rejected a native Cloudflare speed client, dropped nearby-channel counts and timed Wi-Fi sampling, and kept Path Trace. | Phase 1, in the owner-approved order. |
| [#80](https://github.com/cgfixit/LanCartographer/issues/80), native Windows acceptance | Open. Windows 10 full matrix and Windows 11 smoke gate the first `v*` tag. Version strings disagree. | Phase 2 and the release decision. |
| [#89](https://github.com/cgfixit/LanCartographer/issues/89), 23-finding audit | Open. Findings 1, 2, 3, 4 and 13 are resolved on main. Eighteen findings remain to verify or disposition. | Phase 0, with finding 22 coordinated with phase 2. |
| [#90](https://github.com/cgfixit/LanCartographer/issues/90), audit remediation plan | Open. It contains research, PR grouping and decisions D1 through D10. Only D2 is clearly evidenced as an owner choice acted on by #92. | Phase 0 implementation detail and pending decisions. |
| [`docs/network-workbench-roadmap.md`](https://github.com/cgfixit/LanCartographer/blob/3181efb508d75dac65c48dc254b7aba8dec5c985/docs/network-workbench-roadmap.md) | Phases 0 through 2 describe shipped work. Phase 3 still needs Windows Connections, while other Windows diagnostics need separate admission. Phase 4 remains task-gated. Periodic Connections sampling and change highlighting are still unimplemented follow-ups. | Shipped-work removal, phases 3 through 5, and the deferred-boundary list. |

Keep #22 as the human-facing product roadmap. If the owner adopts this structure, replace the repository roadmap through a documentation-only PR and link to it rather than copying acceptance checklists into more issues. Keep issue comments for dated status and decisions.

## Feature selection and evidence design

### What to borrow without cloning

| Existing product or project | Documented capability | Implication |
|---|---|---|
| NirSoft CurrPorts and TcpLogView | Local TCP/UDP process views and connection event history. | A process/socket table alone is not enough. A manual, coverage-aware socket diff tied to inventory and service evidence is the useful next step. |
| NirSoft TableTextCompare | Compares snapshots exported by other utilities. | Snapshot comparison is established. The application's value is typed evidence, context compatibility and a direct next check, not another generic table diff. |
| Microsoft TCPView | Windows endpoint ownership, refresh and change highlighting. | Windows Connections should preserve the same semantics as macOS rather than grow a separate product workflow. |
| Objective-See Netiquette and Sloth | macOS process, socket and descriptor inspection. | Do not compete on the number of socket columns. Connect local observations to inventory and reproducible checks. |
| OWASP Amass | Broad external asset discovery and attack-surface mapping using passive and active reconnaissance. | Do not build another subdomain discovery engine. Import an operator-approved scope and help verify selected exposure changes. |
| Objective-See KnockKnock and BlockBlock | Broad persistence enumeration and continual persistence monitoring. | Do not build a KnockKnock clone or background malware monitor. A narrow, manual diff of approved persistence and network configuration evidence fits this app better. |
| Nmap and Ndiff | Scan evidence, port-state observations and comparison. | Reachability is an observation from one location, not proof of vulnerability or isolation. Nmap XML remains a possible import, not a reason to recreate Nmap. |

### Evidence-model candidates

Two structures were considered before sequencing the new work.

**Candidate A: expand saved sessions into a general evidence container.** Inventory, socket snapshots, service checks, external observations and integrity records would all live in the session JSON. This gives one file to save and compare. It also couples unrelated validation, privacy and redaction rules to schema version 1. Current sessions have an 8 MiB limit, a 65,536-host limit, strict IPv4 inventory validation and no migration framework. Comparison also requires matching app versions. Every unrelated artifact would inherit those constraints.

**Candidate B: keep inventory sessions stable and add separate versioned observation artifacts.** Each artifact declares its kind and schema, plus a small shared context for application version, source platform, source interfaces, capture interval, completion state and an optional operator-supplied site label. Comparison stays within one artifact kind. Reports can select across artifacts without pretending their evidence is equivalent.

**Recommendation: Candidate B.** It reduces migration and redaction coupling, lets each workflow ship independently, and matches the earlier #22 recommendation to keep service-check JSON separate from sessions. The accepted cost is a small artifact index and explicit report correlation. Do not build a universal plugin framework or a graph database to support it.

A minimal envelope sketch, not an approved schema, is:

```go
type Envelope struct {
    Schema     int
    Kind       string
    AppVersion string
    Context    Context
    StartedAt  time.Time
    FinishedAt time.Time
    State      string
}
```

Each kind owns its payload, strict decoder, limits, comparison rules and redaction policy. Missing evidence remains unknown. A cancelled or partial capture cannot establish absence.

## Numbered delivery plan

### Phase 0. Reconcile the plan and stabilize current main

**Sources:** #89 and #90. **Priority:** before new feature work. **Release impact:** finding 22 is also part of phase 2.

First update stale planning material in a documentation-only change:

- Replace the old roadmap with this consolidated structure after owner review.
- Mark #82, #83 and #84 shipped in #22.
- Add #89 and #90 to the open-work list in `AGENTS.md`, which currently names only #22, #40 and #80.
- Refresh #89 so PR #93 is no longer described as a draft.
- Preserve dated research and verification records. Do not rewrite old records to make them look current.

Then disposition the eighteen remaining audit findings. These are not eighteen confirmed defects. Some need reproduction, some need an owner decision, and some are cleanup proposals.

| Cluster | Findings | Required outcome |
|---|---|---|
| macOS ARP and SMB behavior | 7, 8 | Qualify numeric ARP lookup and SMB no-prompt behavior on a Mac. Record coverage, timing and error mapping. Keep the timeout and prompt effects unconfirmed until reproduced. |
| Table selection and live status | 5, 6, 14 | Exercise same-cell selection after sorting, trailing table repaint and stale scan status in native GUI evidence. |
| Probe behavior | 9, 18 | Decide repeated-port semantics. If adding a 32 KiB inventory response-header cap, measure it and call out the new limit. |
| Local gate and CI | 10, 11, 21 | Prove local gate failures cannot pass silently. Decide the published-build cache policy and either cover macOS with `govulncheck` or document the accepted gap. |
| Desktop actions and shutdown | 12, 15, 16, 17 | Decide whether to delete dead macOS elevation code. Verify Wake-on-LAN destination selection, helper reaping and bounded shutdown during a scan. |
| Persistence hygiene | 19, 20 | State durability guarantees and decide whether saving prunes expired name-cache entries. |
| Version and cleanup | 22, 23 | Resolve the version source in phase 2. Record a disposition for every small cleanup in finding 23. |

Preserve #90's one-concern PR discipline. Stacked branches are appropriate where several findings touch `internal/gui/gui.go`, but each PR still needs its own evidence. D2 is already reflected in merged #92. Do not represent D1 or D3 through D10 as decided without an owner statement.

**Exit criterion.** Every finding is fixed on main, explicitly deferred with a linked follow-up, or rejected with a reason. Required native and CI evidence is recorded by tier. #89 can close only after that disposition is complete, including finding 22.

### Phase 1. Finish macOS diagnostics wave 2

**Source:** #40 and its owner decisions dated 2026-11-16 and 2026-11-20. **Order:** runner hardening and qualification, Wi-Fi checklist, local reads, App Traffic, Path Trace, conditional Apple metrics.

1. **Harden `macdiag.Run` without changing behavior.** Require a caller-independent deadline in the run specification. Add typed missing, timeout and exit errors. Move the existing diagnostic-interface validator into the shared package. Add a bounded line-callback variant only when Path Trace needs it. Keep output caps, fixed environment, `WaitDelay`, cancellation and child reaping.
2. **Record Phase 0 qualification on the M5.** Capture exact argv, exit codes, timing and redacted output for `ipconfig`, `scutil --proxy`, `networksetup`, `socketfilterfw`, `nettop` and `traceroute`. Drop anything that needs elevation or cannot be bounded. Inspect Apple `networkQuality -c` output for defensible per-probe data.
3. **Run the shipped Wi-Fi checklist.** Cover Wi-Fi off, disconnected, no default gateway, an IPv6 default gateway, Stop during capture and before/after comparison. Record whether each state starts a gateway ping.
4. **Add Addressing & Proxy to Route & DNS.** Show DHCP lease, static or self-assigned state, proxy and PAC configuration, and service order. Display PAC URLs without fetching them. Make no settings changes.
5. **Add Firewall posture to Connections.** Read global, block-all and stealth states. Do not infer that a firewall is the proven cause of a failed remote connection. Note possible configuration-profile enforcement.
6. **Add App Traffic to Connections.** Difference two bounded `nettop` samples. Join on process name and PID, discard decreasing counters, and state that process totals and socket snapshots are sampled separately. No background poller.
7. **Add Path Trace as a fifth direct Tools entry.** Resolve once, require selection and confirmation of one numeric destination, and pass only that literal to the fixed helper argv. Stream bounded output, label silent hops "no reply", support Stop and close, and reap the child. Update `INVARIANTS.md` and `SECURITY.md` in the same PR because this is a new egress path.
8. **Add Apple jitter or loaded latency only if qualification found usable per-probe data.** This conditional default was recorded in #40, but was not an explicit owner choice. Otherwise record the absence and skip the work.

Native evidence for this issue is current macOS on Apple Silicon. The owner waived the macOS 14 runtime check for this issue, so do not claim minimum-OS runtime verification. Hosted builds and Mach-O minimum checks remain different evidence.

**Exit criterion.** Each shipped tool has a new dated record from the packaged app, synthetic or redacted data, cancellation evidence and the stated traffic behavior. README, USER-GUIDE and SECURITY match the implementation. Windows entries remain hidden and `asInvoker` is unchanged.

### Phase 2. Run native Windows acceptance and decide the first tag

**Source:** #80 and audit finding 22. **Gate:** first `v*` tag, not every later PR.

Test one named commit with a disposable `%APPDATA%` and owned loopback fixtures. Record executable SHA-256 hashes, OS builds, exact commands and synthetic screenshots.

- Run the full Windows 10 standard-user matrix: launch, authorization refusal and approval, inventory, Stop, exports, favorites, sessions, comparison, redacted evidence, Check Service, Tools visibility, quick actions and CLI JSON behavior.
- Run the shorter Windows 11 smoke: launch, loopback scan, session save/reload, redacted preview, one service check, and light/dark themes.
- Keep hosted `windows-latest` build and test results separate from native desktop evidence.
- Align the GUI, macOS package and Windows resource versions from one chosen source.
- Decide the saved-session consequence before changing the app version. Comparison currently requires matching `AppVersion`, and there is no migration path. Either accept and document that old `0.1.0` sessions will not compare with `1.0.0`, or choose a tag/version path that preserves the intended behavior.

**Exit criterion.** Merge `docs/verification/windows-acceptance-YYYY-MM-DD.md` with a result per matrix item, or record the owner's written deferral of a named check. The owner then decides whether to tag. A roadmap entry does not authorize a release.

### Phase 3. Add a one-shot CLI service check

**Source:** #22. **Existing gate:** native Check Service evidence on both operating systems and at least one recorded real operator use. **Code base:** reuse `internal/servicecheck`; do not create another network engine.

Agree on semantics before the CLI entry point:

- Define distinct usage, authorization refusal, pass, fail and unknown/incomplete exit codes without destabilizing the existing inventory CLI contract.
- Require a pinned numeric address, or fail when a hostname has multiple answers. Do not silently pick one.
- Apply the same fail-closed authorization policy as other active checks.
- Return strict, versioned JSON through a separate artifact. Keep it out of saved inventory sessions.
- Preserve one dial per check, current timeouts, verified TLS, no proxy, no redirects and the one-second/32-KiB HTTP bounds.

**Exit criterion.** A loopback fixture demonstrates success, refusal, timeout, TLS failure, HTTP result, cancellation and authorization refusal. Multi-address input follows the agreed rule. GUI and CLI results agree for the same pinned target, and native Windows evidence is included.

### Phase 4. Bring Connections to Windows

**Source:** #22 and `docs/network-workbench-roadmap.md` phase 3. **Prerequisite:** neutral row model before backend code.

Move `Endpoint`, `Connection` and `Snapshot` out of the package named `macdiag` in a no-behavior-change PR. Keep the `lsof` parser platform-neutral, but do not pretend it is a Windows parser. Make the shared Fyne view consume neutral rows while platform collectors remain separate.

For Windows, evaluate bounded `GetExtendedTcpTable` and `GetExtendedUdpTable` owner-PID enumeration. Cap buffer growth and retries, treat process-name lookup as best effort, render UDP as local binds when no peer exists, and keep the Windows manifest at `asInvoker`.

**Exit criterion.** The same inspect, filter and inventory-link workflow works against owned Windows fixtures. Process exit during attribution, access denial, refresh cancellation, empty results and OS-specific missing fields are visible and honest. Native Windows interaction is required.

### Phase 5. Admit identity, discovery and bounded observation only against concrete tasks

**Source:** #22 and `docs/network-workbench-roadmap.md` phases 1 and 4. These are independent options, not a bundle to implement automatically.

- **Identity corrections beyond #82:** consider user corrections and reversible manual links scoped to one chosen network context. Preserve conflicting observations. Do not create stable cross-site identifiers or silently merge devices.
- **Connections refresh and change highlighting:** consider bounded foreground sampling only if the explicit snapshot and phase 6 diff do not solve a named operator task. Keep Stop and Close reliable, show capture coverage, and preserve PID or socket-tuple reuse ambiguity. No background poller.
- **DNS-SD:** browse a small approved service-type set on demand. Bound duration, records and TXT data. Record instance, interface and advertised target. Keep advertised, reachable and authenticated outcomes separate. Never let an advertisement silently widen probe scope.
- **IPv6 inventory:** define destination authorization, interface zones and artifact semantics before probing. Start from selected addresses or discovered evidence, not exhaustive `/64` enumeration. This is a policy and schema change, not parser cleanup.
- **Foreground service watch:** observe one approved service for a bounded period with a conservative interval, visible last result and Stop. No scheduler, daemon or automatic remediation.
- **Nmap XML import:** choose it only for a real assessment or infrastructure task. A bounded parser must not probe, retrieve external XML resources or treat stale producer assertions as fresh observations. It may also serve phase 8, but that does not make it mandatory there.

**Exit criterion for each admitted slice.** One named operator task justifies the slice, native permission and cancellation behavior is recorded, and its evidence cannot be mistaken for a complete network view. Unadmitted items remain deferred without blocking the release or other phases.

### Phase 6. Add network-context checks and manual socket diffs

**New proposals 1 and 2.** **Prerequisite:** the separate evidence envelope from the design section. **Release dependency:** none.

**6A. Network-context compatibility checks.** Answer "were these observations taken from the same operating context?" Capture an operator-supplied site label, source platform and interfaces, capture times, completion state, and relevant route, resolver, proxy or VPN evidence where already available. Use the new local readouts from phase 1 when present, but do not block this slice on them.

Comparison should explain material differences and leave incompatible conclusions unknown. Identical private subnets at different sites must not merge automatically. Do not create a persistent cross-site device identifier from MACs, SSIDs or hashed identifiers. A hash can still enable correlation.

**6B. Manual socket baseline and diff.** Let the operator save two explicit Connections snapshots and compare listeners, local binds, established peers and owning process observations. Match only on documented fields. If process start identity is unavailable, label PID reuse and process restart as ambiguity rather than a clean lifecycle history. Link an exact IPv4 peer to the current completed inventory only where the existing bridge supports it.

Both slices perform no network traffic when saving, opening or comparing artifacts. Full artifacts stay local. Redacted export remains a separate explicit action.

**Exit criterion.** Synthetic fixtures cover same-context comparison, a site change, VPN or resolver changes, incomplete captures, duplicate rows, missing process identity and a loopback listener that is not LAN-reachable. Native macOS covers the socket UI. Windows comparison waits for phase 4 unless it only imports artifacts, in which case that limitation is stated.

### Phase 7. Reconcile local listeners with other vantages

**New proposal 3.** **User task:** "The service works on this machine but fails from a LAN client, VPN or another runtime context." **Prerequisites:** phases 3 and 6. Windows local-listener coverage also needs phase 4.

Combine a local listener observation, an inventory observation and one or more explicitly authorized service-check artifacts. The first version imports a check artifact produced on another operator-controlled machine. It does not ship a remote agent or contact a second host automatically.

Useful results include:

- A loopback-only listener explains why a local success cannot establish LAN reachability.
- A wildcard listener plus a failed remote check isolates a discrepancy to investigate. It does not prove a firewall cause.
- Different resolver answers across contexts justify checking split DNS. They do not prove the VPN is defective.
- A missing positive control makes an isolation or failure conclusion unknown.
- A container or VM path that was not measured remains untested. A native host check cannot speak for that runtime.

Keep every source, interface, address, protocol and timestamp visible. Do not collapse the result into a single health score.

**Exit criterion.** Owned fixtures reproduce loopback-only and wildcard binds, remote success and refusal, different DNS answers, stale artifacts and a missing positive control. Each displayed conclusion names its evidence and its untested paths. No real client inventory appears in records or screenshots.

### Phase 8. Add authorized external-exposure review and remediation retests

**New proposal 4.** **User task:** "For this domain I own or am explicitly authorized to assess, which selected endpoints were observable before, what changed, and did the remediation retest produce the intended result?"

Start with evidence import, not Internet-wide discovery:

- Accept a strict, user-supplied scope file and a user-exported BIND zone file. Cloudflare's dashboard can export DNS records as a zone file, and its export can carry `cf-proxied` tags. No Cloudflare token or live account connection is needed.
- Treat CNAME targets, third-party hosted addresses and other referenced infrastructure as unapproved until the operator explicitly includes them. Importing a file must not resolve or probe it. A scope file records an operator assertion; it is not proof of ownership or permission.
- Preserve source, record type, TTL, proxy metadata where supplied, import time and parse coverage. Unknown or unsupported records stay visible as unverified evidence, not silently dropped.
- Display AAAA evidence without silently activating IPv6 checks. Active IPv6 probing waits for the authorization policy in phase 5 or a separate explicit design.
- For an active retest, present the concrete selected endpoints and traffic profile for confirmation. Reuse `servicecheck` with a small sequential or otherwise tightly bounded run. No brute-force names, wide port sweep, crawling, exploitation or vulnerability scoring.
- Store results in an external-observation artifact and compare only compatible scope and methods. A failed connection alone does not prove remediation, isolation or absence.
- When a Cloudflare record is proxied, label checks as observations of the Cloudflare edge. They do not establish origin reachability or origin exposure.

OWASP Amass already covers broad passive and active asset discovery. Do not reproduce it. An Amass or Nmap importer can be considered later when a concrete assessment task supplies real format requirements. The first slice needs neither.

**Exit criterion.** Synthetic zone and scope fixtures cover duplicates, malformed records, unsupported types, CNAME scope boundaries, proxied metadata, oversized input and cancellation. Active acceptance uses an owned domain or owned fixture with explicit authorization and records exactly what was contacted. Declining the active step sends nothing. Tests and records contain no third-party target data.

### Phase 9. Add narrow local persistence and network-configuration integrity diffs

**New proposal 5.** **User task:** "Which approved startup or network-configuration items changed since my baseline?" This is a manual integrity workflow, not malware detection.

Use a fixed, documented evidence set approved by the owner before implementation. A reasonable first spike covers user-writable launch-agent definitions and explicitly selected shell startup files, plus route, resolver, proxy/PAC and service-order observations already exposed by the diagnostics. Read system-wide locations only if the approved design demonstrates unelevated, privacy-acceptable behavior.

For each file entry, record only approved metadata such as relative path, size, mode, modification time and SHA-256. Parse selected plist fields such as `Label`, `Program` and `ProgramArguments` as untrusted data. Open only regular files without following links, reject special files, use capped reads, and mark an observation unknown if file identity changes during capture. Do not read referenced program contents by default. Cap files, bytes and total work. Mark unreadable, missing and unsupported evidence as unknown rather than unchanged.

Comparison reports added, removed, changed, unchanged and unknown observations. "Changed" never becomes "malicious". A locally stored baseline is not tamper-proof and provides no cryptographic attestation. No reputation lookup, VirusTotal submission, blocking, deletion, quarantine or automatic remediation.

Apple Endpoint Security is a C API for monitoring system events and is packaged through a system extension. BlockBlock already monitors persistence locations and requires system privileges and Full Disk Access for its Endpoint Security-based monitoring. That design conflicts with this application's lightweight, unelevated, no-daemon contract. Do not quietly grow this feature into continuous monitoring. If continuous event monitoring is ever wanted, it needs a separate product and security decision.

**Exit criterion.** Synthetic directory fixtures cover content changes with preserved timestamps, additions, removals, unreadable files, symlinks, malformed plists, control characters and every cap. Native macOS evidence uses disposable directories, never the operator's real startup files. Network-configuration comparisons distinguish unavailable evidence from no change. No background process remains after the window closes.

## Cross-phase impact analysis

A phase is not done when its checkbox lands. These downstream effects must be checked before it is called done.

| Change | Downstream impact | Required response |
|---|---|---|
| Phase 0 GUI and subprocess fixes | Changes the shared baseline used by #40, Windows acceptance and every later UI workflow. | Rebase affected work onto the merged fixes. Do not build new windows on stale GUI or runner code. |
| Phase 0 version finding 22 | Affects session comparison and release identity. | Coordinate with phase 2. Record old-session behavior in release notes. |
| Phase 1 runner hardening | Becomes a dependency for later local collectors and integrity capture. | Require deadlines and typed errors before adding helpers. Keep streaming separate from buffered reads. |
| Phase 1 Path Trace | Adds a new network path outside the original local-read diagnostics. | Update `INVARIANTS.md`, `SECURITY.md`, user documentation and the PR egress justification together. |
| Phase 2 first tag | Publishes Windows binaries and establishes version-comparison behavior. | Do not let later proposals delay it. Do not tag before the owner accepts the Windows and version evidence. |
| Phase 3 service-check JSON | Supplies reusable evidence for phases 7 and 8. | Freeze address-selection, completion and exit semantics before building importers or batch retests on it. |
| Phase 4 neutral connection model | Enables shared Windows/macOS socket views and later socket diffs. | Keep platform collectors behind the neutral model. Do not leak `lsof` fields or Windows table types into the GUI. |
| Phase 5 IPv6 | Changes authorization and evidence semantics beyond the current IPv4 inventory. | Update policy before code. External-domain AAAA display must not bypass this gate. |
| Phase 5 DNS-SD | Advertisements can name addresses and targets outside the selected scope. | Discovery never authorizes a probe. TXT and names remain bounded, untrusted input. |
| Phase 5 identity corrections | User links can outlive the evidence that justified them and can join equal private addresses at different sites. | Scope links to one explicit context, make them reversible, and preserve conflicting observations. |
| Phase 5 Connections refresh | Repeated capture can leave child processes running or imply lifecycle history that sampled sockets cannot prove. | Keep capture foreground and bounded. Reap helpers on Stop and Close. Label reused identity as ambiguous. |
| Phase 6 shared context | Becomes the compatibility basis for new comparison artifacts. | Keep it small and kind-neutral. Do not retrofit unrelated payloads into session schema 1. |
| Phase 7 imported second-vantage evidence | Creates a correlation and privacy risk across sites and machines. | Require explicit artifact selection and visible provenance. Never auto-merge equal private addresses. |
| Phase 8 external observations | Introduces public-target authorization and third-party infrastructure boundaries. | Separate passive import from active retest. Confirm concrete destinations. Keep Cloudflare edge and origin claims distinct. |
| Phase 9 file hashing and startup metadata | Handles more sensitive local data than inventory alone. | Use a fixed allowlist, strict caps, disposable fixtures and separate redaction rules. No real startup files in evidence. |

## Pending owner decisions

The following decisions remain open unless a merged implementation or quoted owner comment is named. Recommendations here do not convert them into decisions.

| Decision | Current state | Recommendation |
|---|---|---|
| Adopt this consolidated roadmap and phase order | New proposal. | Adopt the order, but keep phases 5 through 9 out of the first-release gate. |
| Separate observation artifacts instead of expanding sessions | Recommended design, not approved. | Approve candidate B before any new evidence schema is implemented. |
| #90 D1, audit plan shape | No clear owner choice found. | Keep phased, one-concern PRs rather than 23 tiny PRs or three mixed bundles. |
| #90 D3, repeated ports | Open. | Prefer first-seen deduplication, but record the CLI behavior change. |
| #90 D4, published-build Go cache | Open. | Use cold caches for published main builds unless evidence supports another policy. |
| #90 D5, dead macOS elevation path | Open. | Delete it unless a real elevated macOS requirement appears. |
| #90 D6, inventory HTTP header cap | Open. | Add 32 KiB only with fixture evidence, a PR callout and documentation of the measured limit. |
| #90 D7, macOS ARP/SMB evidence | Partially advanced by the #93 SMB fixture, not settled for all findings. | Capture owned, redacted ARP and SMB no-prompt evidence before claiming runtime fixes. |
| #90 D8, version source | Open. | Choose one tag-driven or checked-in source before the first tag. |
| #90 D9, ARP implementation | Open. | Start with numeric targeted lookup. A native route-table reader needs its own measured spike. |
| #90 D10, expired name-cache entries | Open. | Prefer pruning on save if the privacy-positive behavior is acceptable. |
| #80 first tag and old-session compatibility | Open. | Decide after native Windows acceptance. State the comparison consequence in release notes. |
| #22 one-shot service-check CLI semantics and real-use gate | Open. | Record one real operator use and native evidence on both OSes before implementation. |
| Phase 5 task admission | Open per item, including identity corrections and Connections refresh. | Admit only the item tied to a concrete task. Do not treat this list as blanket approval. |
| Phase 6 context fields and socket identity | Open. | Minimize identifiers and accept ambiguous process identity where start evidence is unavailable. |
| Phase 8 source formats and active-run bounds | Open. | Start with scope JSON and BIND zone import. No live Cloudflare API or subdomain brute force. |
| Phase 9 evidence locations | Open. | Start with disposable user-scoped fixtures. Expand only after privacy and unelevated reads are proven. |

## Boundaries for every phase

Scan only networks you own or have explicit authorization to inventory. Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws.

Keep local reads separate from actions that generate traffic. Every active target and traffic purpose must be visible before contact. Passive import performs no DNS lookup, HTTP request or external resource retrieval. New egress requires an explicit justification and matching security documentation.

The app owns the allowlist of executables and argument layouts. Use fixed absolute paths, typed arguments, deadlines, output and row caps, cancellation and child reaping. No shell strings, arbitrary commands, installer, privileged helper, persistent monitoring daemon, runtime downloads or telemetry. Keep macOS unelevated and the Windows manifest at `asInvoker`.

Treat process names, helper output, plist values, DNS names, TXT records, URLs and imported assessment data as untrusted input. Keep identifiers out of default reports. Export is explicit, with a preview and artifact-specific redaction. Full session JSON and other raw artifacts remain sensitive.

Preserve the existing worker and dial budgets, verified TLS, no redirects, and one-second/32-KiB HTTP service-check bounds. Changing a measured limit needs a measurement and a PR callout. A timeout, silent host, missing file, denied read or failed connection is not proof of absence, isolation or compromise.

Verify by running the changed behavior. Use owned loopback or synthetic fixtures and disposable application data. Keep focused tests, CLI fixtures, native GUI interaction, packaged smoke tests, hosted CI and release acceptance as separate evidence tiers. Write new dated verification records. Never place real inventory, session JSON, startup files, credentials, MACs, SSIDs, hostnames or third-party targets in tests, screenshots or records.

Each implementation slice is one concern per draft PR, cut from fresh `origin/main`. Never push main, force-push, merge or publish a release without the required explicit owner action. Keep #22 as the product index and update issue status as slices land.

## Explicitly deferred or rejected

- General subdomain brute force, Internet-wide scanning, exploitation, vulnerability scoring and an Amass replacement.
- A general EDR, whole-disk anomaly detector, KnockKnock clone, BlockBlock clone or continuous persistence monitor.
- Packet capture, ChmodBPF, traffic blocking, remote administration and automatic remediation. A read-only viewer for user-supplied capture files is a separate later proposal from #40, not approved work in this roadmap.
- Path Trace `Repeat ×5`. The owner kept the single confirmed-destination trace, while repeated runs remain a later checkbox.
- Windows Route & DNS and other Windows diagnostic ports beyond Connections. Each needs a concrete task and a separate design. Phase 4 does not approve them.
- A native Cloudflare speed client, in-app installers, Ookla and `mach`. The owner already chose Apple `networkQuality` plus browser handoff. Third-party speed CLI adapters return only on a concrete user request.
- Direct Cloudflare account integration and application credential storage for the first external-exposure slice. User-exported files are sufficient.
- Stable cross-site device tracking, automatic merging of equal private subnets and identity claims based only on a MAC or IP.
- A general plugin framework, graph database, guessed topology map, 3D map, cloud account service or AI-chat headline feature.
- Container, VM or remote-runtime claims without evidence collected from that context.

## Research sources

Sources were checked on 2026-11-26 unless a date is stated. They support capability and boundary claims, not demand, market-share or performance claims.

### Repository and issue evidence

- [Current main commit](https://github.com/cgfixit/LanCartographer/commit/3181efb508d75dac65c48dc254b7aba8dec5c985), PR #94 merge.
- [Issue #22](https://github.com/cgfixit/LanCartographer/issues/22), including owner decisions dated 2026-11-16 and 2026-11-20.
- [Issue #40](https://github.com/cgfixit/LanCartographer/issues/40), including the revised order and native-evidence waiver.
- [Issue #80](https://github.com/cgfixit/LanCartographer/issues/80), Windows acceptance and version compatibility.
- [Issue #89](https://github.com/cgfixit/LanCartographer/issues/89), audit findings and dated resolution comments.
- [Issue #90](https://github.com/cgfixit/LanCartographer/issues/90), remediation research and D1 through D10.
- [AGENTS.md](https://github.com/cgfixit/LanCartographer/blob/3181efb508d75dac65c48dc254b7aba8dec5c985/AGENTS.md), [INVARIANTS.md](https://github.com/cgfixit/LanCartographer/blob/3181efb508d75dac65c48dc254b7aba8dec5c985/INVARIANTS.md), [README.md](https://github.com/cgfixit/LanCartographer/blob/3181efb508d75dac65c48dc254b7aba8dec5c985/README.md) and the current `docs/verification/` records at the same commit.

### External workflow and platform evidence

- [OWASP Amass](https://github.com/owasp-amass/amass) documents broad attack-surface mapping and external asset discovery using passive and active reconnaissance.
- [Cloudflare DNS import and export](https://developers.cloudflare.com/dns/manage-dns-records/how-to/import-and-export/), last updated 2026-04-16, documents dashboard zone export, API export permissions and `cf-proxied` export tags. No API call was made for this roadmap.
- [NirSoft network tools](https://www.nirsoft.net/network_tools.html) and [TableTextCompare in the utility list](https://www.nirsoft.net/utils/) document process/socket views, connection history and snapshot comparison.
- [Apple Endpoint Security](https://developer.apple.com/documentation/endpointsecurity) documents a C API for monitoring system events and packaging through a system extension.
- [Objective-See BlockBlock](https://objective-see.org/products/blockblock.html) documents continual persistence monitoring, Endpoint Security, system privileges and Full Disk Access.
- [Objective-See KnockKnock](https://github.com/Objective-See/KnockKnock) documents broad persistence enumeration. Its existence is why this roadmap does not propose a clone.
- [Nmap port-state semantics](https://nmap.org/book/man-port-scanning-basics.html) and [Nmap output formats](https://nmap.org/book/man-output.html) support observation-limited conclusions and a possible bounded XML import.
- [Docker Desktop networking](https://docs.docker.com/desktop/features/networking/) supports treating container and VM context as material rather than inferring it from a native host check.
- [Microsoft `GetExtendedTcpTable`](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedtcptable) and [`GetExtendedUdpTable`](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedudptable) are the candidate Windows connection sources to qualify.
- [Apple TN3179](https://developer.apple.com/documentation/technotes/tn3179-understanding-local-network-privacy), [RFC 7707](https://www.rfc-editor.org/rfc/rfc7707.html) and [RFC 6763](https://www.rfc-editor.org/rfc/rfc6763.html) constrain local-network permission, IPv6 discovery and DNS-SD work.
- [NetBeez](https://netbeez.net/network-monitoring/) and [Batfish](https://github.com/batfish/batfish), carried forward from #22, are counterevidence to any claim that distributed-vantage tests or network verification are new ideas.

## Final review notes

This roadmap deliberately removes shipped #82, #83 and #84 from future work and corrects the audit count to five resolved and eighteen remaining. It restores reversible identity links and bounded Connections refresh as task-gated options from #22 and the repository roadmap. It leaves capture-file viewing, repeated Path Trace runs and other Windows diagnostic ports unapproved. It does not convert plausible audit findings into confirmed runtime defects.

The new workflows do not require changing session schema 1. They do not authorize new background behavior, privileged macOS components, direct Cloudflare credentials or broad reconnaissance. External-exposure review begins with local evidence import and narrowly confirmed retests. Local integrity reports changes, not maliciousness.

The release path remains short: disposition the audit, finish the already approved macOS wave, perform native Windows acceptance, resolve versioning, then let the owner decide the first tag. Everything after that must earn its place through a concrete operator task and a separately reviewable slice.


