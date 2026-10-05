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

**Toolchain:** Go 1.27 or later, plus a CGO C compiler — on Windows use the
MSYS2 `mingw-w64` gcc (`pacman -S mingw-w64-ucrt-x86_64-gcc`; the `ucrt64`
shell puts it on PATH). The GUI (Fyne) requires CGO; the CLI is the same
binary driven by a flag, so both builds use `CGO_ENABLED=1 CC=gcc`.

One-time setup for the version-resource tool used by `go generate`:

```bash
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
```

Then build both binaries from a Git Bash shell:

```bash
./build.sh
```

`build.sh` runs `go generate ./...` (regenerates the icon, `app.manifest`,
and `versioninfo.json`-driven `resource.syso` version resources) and then
produces, in `dist/`:

- **`dist/LanCartographer.exe`** — the GUI build
  (`-trimpath -ldflags "-s -w -H windowsgui"`); double-click it, or run it
  from a shell. It runs asInvoker (no admin required to start).
- **`dist/lancartographer-cli.exe`** — the console build (same flags without
  `-H windowsgui`), for scripting and CI.

### CLI usage and exit codes

```bash
lancartographer-cli.exe --cli [--range CIDR] [--ports 21,22,80,...] \
    [--timeout 400ms] [--concurrency 128] [--json out.json] [--i-am-authorized]
```

With no `--range`, every local physical subnet is inventoried. Ranges broader
than your local subnets require `--i-am-authorized` confirmation. Exit codes:
`0` scan completed; `2` bad arguments, an authorization-gate refusal, or a
failed `--json` export; `3` no usable local interfaces (only reachable on the
omitted-`--range` path).

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

Application code complete: discovery, probing, scan engine, GUI, CLI, WOL,
export, and build packaging all live on `main`.
