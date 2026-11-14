# Documentation and verification

Use the [README](../README.md) for features/builds, the
[User Guide](../USER-GUIDE.md) for scan, session and Internet Quality workflows,
[update guide](../update-guide.md) for replacing builds, and
[contributor guide](../CONTRIBUTING.md#dependency-maintenance) for dependency
maintenance. [Support](../SUPPORT.md) and [security reporting](../SECURITY.md)
describe what to include and what to keep private.

## Verification records

Each record describes the source, artifact, platform and checks actually used.
Historical passes remain evidence of that run; they do not accept a newer
dependency set, native platform or downloaded release.

| Record | Coverage |
| --- | --- |
| [Dependency refresh, 2026-11-01](verification/dependencies-2026-11-01.md) | Go 1.27.3/Fyne 2.8.2 package, owned CLI smoke, focused checks and native session dialogs/themes |
| [Internet Quality, 2026-11-07](verification/internet-quality-2026-11-07.md) | Live Apple measurement, native stop/close, browser handoff and packaged loopback smoke |
| [Wi-Fi diagnostics, 2026-11-14](verification/wifi-diagnostics-2026-11-14.md) | Native before/after screenshots, live gateway replies, cancellation and packaged loopback smoke |
| [Indirect Go dependency refresh, 2026-11-14](verification/indirect-dependencies-2026-11-14.md) | Patched module selection, signed arm64 bundle, packaged CLI, and native theme, inventory and file-picker checks |
| [Saved sessions, 2026-11-01](verification/sessions-2026-11-01.md) | Owned service change, save/reload, coverage comparison, selected redaction/export, light/dark native interaction and explicit platform limits |
| [macOS packaging and native workflow](macos-verification.md) | Packaged CLI and native GUI results at the recorded revisions |
| [CSV export, 2026-10-13](csv-verification-2026-10-13.md) | Historical CSV escaping/formula-prefix verification |
| [Discovery, 2026-10-13](discovery-verification-2026-10-13.md) | Historical host-discovery verification |
| [HTTP probes, 2026-10-13](http-verification-2026-10-13.md) | Historical bounded HTTP/TLS probe verification |

## Pending work

- [#22: roadmap](https://github.com/cgfixit/LanCartographer/issues/22), including first-slice progress and later proposals.
- [Network workbench plan](network-workbench-roadmap.md): proposed macOS-first Connections window and later phases; no implementation yet.
- [#24: native platform and size-limit acceptance](https://github.com/cgfixit/LanCartographer/issues/24).
- [#25: indirect dependency and action-major review](https://github.com/cgfixit/LanCartographer/issues/25).
- [#26: legacy cleanup decisions](https://github.com/cgfixit/LanCartographer/issues/26).

Check the linked issue/PR head before continuing work. Keep synthetic fixtures
and redacted reports separate from raw session files containing inventory and
source-interface addresses.
