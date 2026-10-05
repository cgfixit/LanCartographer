# LanCartographer

LanCartographer is a Windows LAN inventory tool written in Go (1.27) with a
Fyne v2.8.1 GUI. It discovers the hosts on your local network and inventories
what each one exposes — open TCP ports, service identities, SMB shares, names,
MAC vendors — and presents the result as a live, sortable table with a CLI for
scripting. It ships as a single portable `.exe`: no installer, no services, no
background processes, and no telemetry of any kind.

## Authorized use only

Scan only networks you own or have explicit authorization to inventory. Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws.

LanCartographer is an inventory aid for your own infrastructure. It is not a
vulnerability scanner, an exploitation framework, or a reconnaissance tool for
networks you do not control.

## Features

| Area | What you get |
|---|---|
| Live scan | One-click sweep of all local physical subnets; hosts and open ports appear in the table while the scan is still running, with a progress bar and per-host events |
| Sortable table | Seven columns (status, Name, IP, MAC, Vendor, Ports, Shares); click any header to sort; column widths are computed from window width with per-column minimums |
| Port states | Every probed port reports `open`, `closed`, or `timeout`; open ports carry a service label (smb, http, rdp, …), and open ports with an unrecognized service get a 1-second HTTP probe that promotes web devices and captures the page `<title>` |
| Share enumeration | SMB shares listed as name, type (disk/print/ipc), and remark via `NetShareEnum` level 1; a host that denies the enumeration shows an explicit "access denied" marker instead of an empty list |
| Name resolution | NetBIOS, mDNS, and reverse DNS (PTR) queried concurrently for each host, highest-priority answer wins (NetBIOS → mDNS → PTR; probes run concurrently); resolved names are persisted in a MAC-keyed cache so silent devices keep their names across scans |
| Right-click actions | Open host or share in Explorer, open its web interface in the default browser, start a Remote Desktop session, run a visible Ping, or send Wake-on-LAN (WOL asks for confirmation first) |
| Export | Save the inventory as CSV or JSON from the File menu (the CLI has a `--json` flag) |
| Favorites | Save and reload the current host list as favorites |
| Themes | Dark and light themes, persisted across launches |
| First-run notice | The authorization notice is shown in a dialog on first launch; a marker file records that you have seen it |
| Elevation button | When the process is not elevated, the toolbar offers an Elevate button; MAC lookup needs elevation to return addresses, and the UI says so instead of failing silently |

## How it works

- **Subnet discovery:** local IPv4 subnets are enumerated from your adapters via `GetAdaptersAddresses`, then expanded into candidate host addresses.
- **Concurrent sweep:** the sweep runs up to 64 host workers at once, and a separate global dial budget (default 128, hard cap 256) bounds how many TCP dials may be in flight — two different knobs.
- **Port probes:** each port gets a TCP connect with an exact result — `open`, `closed`, or `timeout`; open ports carry a service label, and unrecognized ones get a 1-second HTTP probe that captures the page `<title>`.
- **Enrichment:** SMB shares are listed via `NetShareEnum` level 1, and names are resolved by querying NetBIOS, mDNS, and DNS PTR concurrently (highest-priority answer wins: NetBIOS → mDNS → PTR), persisted in a MAC-keyed cache.
- **Live events:** the engine streams per-host events over a channel, so the Fyne GUI table fills in while the scan is still running.

## Why LanCartographer

- **Offline-first, zero telemetry:** no analytics, no phone-home, no runtime downloads — the only traffic is the scan you asked for.
- **Single portable exe:** no installer, no services, no background processes; run it from anywhere.
- **No admin required:** the manifest is `asInvoker`; elevation is an opt-in button for the few lookups that need it, never a demand.
- **Authorization-gated ranges:** out-of-scope, non-RFC1918, or overly broad targets require explicit confirmation before a single packet is sent.
- **Original UI:** a purpose-built live table and workflow, not a reskin of an existing scanner.

## CLI reference

`lancartographer-cli.exe` drives the same engine as the GUI and prints a table
(IP, NAME, MAC, VENDOR, PORTS, SHARES) to stdout. Diagnostics go to stderr.
The CLI build auto-detects from its own file name (any exe whose base name
contains "cli", case-insensitive); passing `--cli` as the first argument also
selects CLI mode and is accepted on both binaries.

| Flag | Meaning | Default |
|---|---|---|
| `--range CIDR` | IPv4 prefix to scan, e.g. `192.168.1.0/24` | omitted: every local physical subnet |
| `--ports` | Comma-separated TCP ports to probe (1–65535) | `21,22,23,80,135,139,443,445,515,631,3389,5357,8080,8443,9100` |
| `--timeout` | Per-host dial timeout; the per-dial probe timeout is 2× this value | `400ms` |
| `--concurrency` | Global concurrent-dial budget (engine clamps to 256) | `128` |
| `--json PATH` | Also write the inventory as JSON to `PATH` | off |
| `--i-am-authorized` | Confirm authorization for ranges that require confirmation | off |

A range requires confirmation — and therefore `--i-am-authorized` — when it is
outside your local subnets, **or** not RFC 1918 private space, **or** broader
than a `/22`. Every target is checked before anything is scanned, so a refusal
exits without touching the network.

Exit codes:

- `0` — scan completed (JSON export, if requested, succeeded)
- `2` — bad arguments, an authorization-gate refusal, a scan failure, or a failed `--json` export
- `3` — no usable local interfaces (only reachable when `--range` is omitted)

Examples:

```bash
# Inventory every local subnet (no admin needed)
lancartographer-cli.exe --cli

# Sweep one subnet, save JSON
lancartographer-cli.exe --cli --range 192.168.1.0/24 --json inventory.json

# Wider sweep with explicit authorization
lancartographer-cli.exe --cli --range 10.0.0.0/20 --i-am-authorized

# Just a few ports, faster timeout
lancartographer-cli.exe --cli --range 192.168.1.0/24 --ports 22,80,443,3389 --timeout 250ms

# Same invocations via the GUI binary's --cli token
lancartographer.exe --cli --range 192.168.1.0/24 --json inventory.json
```

## Quick start

1. Download `LanCartographer.exe` from the [GitHub Releases page](https://github.com/cgfixit/LanCartographer/releases)
   (built by the release workflow on version tags) — or build it below — and
   double-click it. Note that `dist/` is build output, not committed to the
   repository. There is no installer and no admin rights are needed to start —
   the executable runs `asInvoker`.
2. Read and acknowledge the authorization dialog on first launch.
3. Click **Scan**. Local subnets are discovered from your network adapters and
   swept; rows appear live.
4. Right-click any host for Explorer / browser / RDP / Ping / Wake-on-LAN, or
   use **File → Export…** for CSV/JSON.

Per-user data lives under `%AppData%\LanCartographer\`:

- `settings.json` — theme preference
- `favorites.json` — saved host lists
- `names.json` — the persistent MAC-keyed hostname cache
- `firstrun_seen` — marker that the first-run authorization dialog was shown

## Build from source

Requirements: Go 1.27 or later, and a CGO C compiler — on Windows use the
MSYS2 mingw-w64 gcc:

```bash
pacman -S mingw-w64-ucrt-x86_64-gcc
```

(Fyne requires CGO; the CLI is the same binary built without the GUI subsystem
flag, so both builds use `CGO_ENABLED=1 CC=gcc`.)

One-time setup for the version-resource tool used by `go generate`:

```bash
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
```

Then, from a Git Bash shell:

```bash
bash build.sh
```

`build.sh` runs `go generate ./...` (regenerates the icon via `tools/icongen`
and the `versioninfo.json`-driven `resource.syso` via goversioninfo, which
consumes the checked-in `app.manifest` — the manifest itself is not
regenerated) and builds:

- `dist/LanCartographer.exe` — the GUI (`-trimpath -ldflags "-s -w -H windowsgui"`)
- `dist/lancartographer-cli.exe` — the console build for scripting and CI

## Security model

- **TCP connect scans only.** Probing is `net.Dialer.DialContext` against
  specific ports — no SYN scans, no raw sockets, no crafted packets. The
  closest thing to "raw" is `SendARP` for MAC lookup, which is a documented
  Windows API call.
- **Fail-closed authorization gate.** A scan target must be confirmed when it
  is outside your local subnets, non-RFC1918, or broader than a `/22` — in the
  GUI via a confirmation dialog, in the CLI via `--i-am-authorized`. Refusal
  means nothing is scanned. The engine also refuses IPv6 and anything broader
  than a `/12` outright.
- **No credentials.** LanCartographer never asks for, uses, or stores
  credentials. Share enumeration runs under your current Windows token; if
  Windows denies the request, the UI shows "access denied" and moves on. When
  Explorer opens a share UNC, Windows enforces the token — not us.
- **No shell strings.** External programs (Explorer, browser, RDP, Ping,
  Wake-on-LAN broadcast) are launched as argv lists only. The Ping and
  browser actions use `cmd /c start` argv vectors whose variable parts are
  validated IP literals from `netip` — never composed command strings — so
  there is no injection surface.
- **No phone-home.** Zero telemetry, zero analytics, no OUI-database downloads
  at runtime — the MAC-vendor (OUI) database is embedded in the binary and was
  current when generated. The only network traffic the tool generates is the
  scanning you asked for.
- **Runs unelevated by design.** The manifest requests `asInvoker` (never
  `requireAdministrator`); elevation is offered as a button, not demanded.

## Platform notes

- **Target:** Windows 10 and Windows 11 (and Windows Server equivalents). The
  application manifest declares only the Windows 10/11 `supportedOS` GUID.
- **Windows 7 / 8.1:** best-effort only. Go 1.21+ dropped support for those
  releases, so Go-built binaries — including this one — cannot support them;
  the manifest intentionally does not claim Windows 7 compatibility.
- **Authenticode:** release builds should be code-signed so SmartScreen and AV
  heuristics do not flag an unsigned network-scanning binary. Recommended, not
  required — signing is not a build blocker.
- **Elevation:** some lookups (MAC via `SendARP`) only return data from an
  elevated process. LanCartographer degrades to an explicit "needs elevation"
  marker rather than failing silently, and offers an Elevate button.

## Repository layout

One line per top-level package; all application code is under `internal/`
(Go's convention for "not importable from outside this module").

| Path | Role |
|---|---|
| `cmd/lancartographer` | `main` entry point plus `go:generate` directives (icon, manifest, version resources) |
| `internal/discover` | Local subnet enumeration (`GetAdaptersAddresses`), host-address expansion, liveness probing |
| `internal/probe` | TCP connect port scans, HTTP title detection, rate limiter, MAC lookup, mDNS/NetBIOS/DNS name probes, share listing |
| `internal/scan` | The central sweep engine: worker pool, event stream, enrichment, MAC-keyed name cache |
| `internal/model` | Inventory records (`Host`, `Port`, `Share`) and the CSV/JSON writers |
| `internal/authz` | The authorization notice and the scan-range confirmation policy |
| `internal/oui` | Embedded MAC-vendor database, generated from `data.csv` |
| `internal/winutil` | Typed Windows API wrappers (`NetShareEnum`, `SendARP`, elevation check) with non-Windows stubs |
| `internal/gui` | The Fyne GUI: toolbar, live table, side panel, context menus, dialogs, settings |
| `internal/cli` | Flag parsing, authorization gate, table output, JSON export, exit codes |
| `internal/maplayout` | Position calculation for the 2D map view (v1.1) |
| `internal/wol` | Wake-on-LAN magic-packet construction and broadcast send |
| `internal/favorites` | Favorites persistence under `%AppData%\LanCartographer\` |
| `internal/export` | File export dispatch (CSV/JSON) |
| `internal/hostcache` | The persistent MAC-keyed hostname cache (`names.json`) |
| `tools/icongen` | Icon generator run by `go generate` to produce `app.ico` |

## Testing

```bash
CGO_ENABLED=1 go test ./... -race
```

Tests are hermetic: fixtures listen on loopback (e.g. `127.0.0.1:18080`) and
use `192.0.2.x` (TEST-NET-1, RFC 5737 — the documentation range, guaranteed
unroutable) for dead-address cases. No test performs a live LAN scan, so the
suite is safe to run anywhere, on any network.

## Roadmap

- **v1.1 — 2D map view:** spatial layout of discovered hosts (positioning
  logic already lives in `internal/maplayout`).
- **No 3D.** A 3D visualization is intentionally absent: it adds GPU/driver
  complexity and installer weight for zero inventory value.

## Inspiration and similar tools

LanCartographer's feature set was inspired by well-known tools in the LAN
scanning space — thank you to their authors and communities:

- [Advanced IP Scanner](https://www.advanced-ip-scanner.com/) (Famatech)
- [Angry IP Scanner](https://angryip.org/)
- [Nmap](https://nmap.org/)

LanCartographer is an independent, original implementation. No code, artwork,
branding, or layout is borrowed from these projects, and LanCartographer is
not affiliated with, sponsored by, or endorsed by any of them.

## License

MIT — see [LICENSE](LICENSE).
