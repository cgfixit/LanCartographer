# Documentation and verification

Use the [README](../README.md) for features/builds, the
[User Guide](USER-GUIDE.md) for scan, session, Check Service and Internet Quality workflows,
[update guide](update-guide.md) for replacing builds, and
[contributor guide](../CONTRIBUTING.md#dependency-maintenance) for dependency
maintenance. [Support](../SUPPORT.md) and [security reporting](../SECURITY.md)
describe what to include and what to keep private.

## Verification records

Each record describes the source, artifact, platform and checks actually used.
Historical passes remain evidence of that run; they do not accept a newer
dependency set, native platform or downloaded release. Records dated before
2026-11-20 name `deploy/`, `packaging/`, `tests/` and `tools/`; those files
now live in [`build/`](../build), and the README images in
[`screenshots/`](screenshots).

| Record | Coverage |
| --- | --- |
| [Scan bounds, 2026-11-21](verification/scan-bounds-2026-11-21.md) | CLI rejects a zero or negative `--timeout` on a loopback fixture; Stop skips name and MAC lookups (focused test fails on `main`, passes here); Stop timing in the native app not measured |
| [Scan certificate expiry, 2026-11-20](verification/scan-cert-expiry-2026-11-20.md) | Loopback TLS fixtures on Linux: untrusted, expiring, expired and locally trusted certificates, equal connection counts before and after; native GUI not run |
| [Session certificate validation, 2026-11-21](verification/session-cert-validation-2026-11-21.md) | Crafted certificate text rejected on read instead of printed in a redacted report (focused test fails on `main`, passes here); staticcheck pinned to the `go.mod` toolchain; native GUI not run |
| [Session address move, 2026-11-20](verification/session-address-move-2026-11-20.md) | Synthetic sessions saved, reloaded, compared and reported on Linux; MAC pairing rules and redaction; native GUI not run |
| [macOS Connections, 2026-11-20](verification/connections-2026-11-20.md) | Owned IPv4/IPv6 sockets, manual refresh, exact inventory bridge and packaged smoke |
| [macOS Route & DNS, 2026-11-20](verification/route-dns-2026-11-20.md) | Native IPv4/IPv6 route reads, resolver configuration, observed localhost answers and Check Service |
| [macOS diagnostics closeout, 2026-11-20](verification/diagnostics-closeout-2026-11-20.md) | Combined package, approved live Apple measurement, inventory export and Sessions |
| [Saved-session size limits, 2026-11-20](verification/session-size-limits-2026-11-20.md) | Native Apple Silicon save/reload, both size ceilings, rejection, selected export, themes and Safari handoff |
| [golang.org/x update, 2026-11-16](verification/golang-x-2026-11-16.md) | x/sys, x/image, x/net, x/text to current; govulncheck clean, packaged smoke, full gate, native Check Service and loopback scan in light/dark |
| [Dependency refresh, 2026-11-01](verification/dependencies-2026-11-01.md) | Go 1.27.3/Fyne 2.8.2 package, owned CLI smoke, focused checks and native session dialogs/themes |
| [Main window flow, 2026-11-15](verification/ui-flow-2026-11-15.md) | Before/after inventory with the same sample data, first-run notice, stop/finish states, favorites/export regressions and the unchanged CLI table |
| [First-run marker write failure, 2026-11-16](verification/firstrun-marker-2026-11-16.md) | Native light/dark error dialog after a real click (redacted), focused test (fails on `main`, passes here) and Fyne-rendered `main` comparison |
| [Inventory table refresh, 2026-11-16](verification/inventory-refresh-2026-11-16.md) | Row preparation allocations, native sort and selection, IP-ordered persistence, and packaged loopback smoke |
| [Check Service, 2026-11-15](verification/check-service-2026-11-15.md) | Owned loopback fixtures for every stage outcome, rendered X11 GUI interaction (decline, prefill, Stop, close, quit), race-checked tests and stated platform limits |
| [Internet Quality, 2026-11-07](verification/internet-quality-2026-11-07.md) | Live Apple measurement, native stop/close, browser handoff and packaged loopback smoke |
| [Wi-Fi diagnostics, 2026-11-14](verification/wifi-diagnostics-2026-11-14.md) | Native before/after screenshots, live gateway replies, cancellation and packaged loopback smoke |
| [Indirect Go dependency refresh, 2026-11-14](verification/indirect-dependencies-2026-11-14.md) | Patched module selection, signed arm64 bundle, packaged CLI, and native theme, inventory and file-picker checks |
| [Saved sessions, 2026-11-01](verification/sessions-2026-11-01.md) | Owned service change, save/reload, coverage comparison, selected redaction/export, light/dark native interaction and explicit platform limits |
| [macOS packaging and native workflow, 2026-10-08](verification/macos-verification-2026-10-08.md) | Packaged CLI and native GUI results at the recorded revisions |
| [CSV export, 2026-10-13](verification/csv-verification-2026-10-13.md) | Historical CSV escaping/formula-prefix verification |
| [Discovery, 2026-10-13](verification/discovery-verification-2026-10-13.md) | Historical host-discovery verification |
| [HTTP probes, 2026-10-13](verification/http-verification-2026-10-13.md) | Historical bounded HTTP/TLS probe verification |

## Pending work

- [#22: roadmap](https://github.com/cgfixit/LanCartographer/issues/22), including first-slice progress and later proposals.
- [Network workbench plan](network-workbench-roadmap.md): macOS Connections, Route & DNS, Check Service and subsequent diagnostics.
- [#40](https://github.com/cgfixit/LanCartographer/issues/40): the second wave of macOS diagnostics.
- [#80](https://github.com/cgfixit/LanCartographer/issues/80): native Windows 10/11 desktop acceptance, due before the first `v*` tag.

Check the linked issue/PR head before continuing work. Keep synthetic fixtures
and redacted reports separate from raw session files containing inventory and
source-interface addresses.
