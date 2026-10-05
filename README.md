# LanCartographer

LanCartographer is a Windows LAN/port inventory utility with both a GUI and a
CLI. It discovers hosts on your local network and inventories their open
ports, producing a map of what is actually on your LAN. It ships as a single
portable `.exe` — no installer, no services, no background processes.

- **Offline and private by design:** zero telemetry, zero phone-home, no
  analytics of any kind. All scanning happens from your machine against your
  network; nothing leaves it.
- **License:** MIT (see `LICENSE`).

## Authorized use only

Scan only networks you own or have explicit authorization to inventory. Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws.

This tool is an inventory aid for your own infrastructure. It is not a
vulnerability scanner, an exploitation framework, or a reconnaissance tool
for networks you do not control.

## Build

Build steps: placeholder — a build script and full instructions will be added
in a later task once the application code exists. For now, the repository is
a skeleton Go module (`github.com/local/lancartographer`) that builds clean:

```bash
go build ./...
go vet ./...
```

## Platform notes

- **Windows 7 caveat:** LanCartographer requires Go 1.21 or later, and Go
  1.21+ builds do **not** support Windows 7 or Windows 8 (Go dropped support
  for them in the 1.21 release). The application manifest must not claim
  Windows 7 compatibility. Windows 10/11 (and Windows Server equivalents) are
  the supported targets.
- **Elevation:** some inventory operations (e.g. certain ARP-table or raw
  socket probes) may require an elevated (Administrator) process. The CLI and
  GUI should degrade gracefully and tell you when a probe needs elevation
  rather than failing silently.
- **Authenticode signing:** for release builds, Authenticode-signing the
  `.exe` with a code-signing certificate is recommended so Windows
  SmartScreen and antivirus heuristics do not flag an unsigned
  network-scanning binary. Signing is a recommendation, not a build blocker.
- **No 3D:** a 3D visualization of the network map is intentionally absent.
  The UI is a plain 2D table/tree view — 3D adds GPU/driver complexity and
  installer weight for zero inventory value.

## Repository status

Early skeleton. Module initialized, license and authorization notice in
place. Application code arrives in later tasks.
