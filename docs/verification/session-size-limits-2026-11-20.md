# Saved-session size acceptance on Apple Silicon

On November 20, 2026, the native macOS app loaded sessions at the 8 MiB
and 65,536-host limits, showed the two expected changes in each pair, and
exported one selected change with redaction on. Each over-limit file was
rejected and left the previously loaded comparison, selection and preview
available. This covers the Apple Silicon size pass in
[#24](https://github.com/cgfixit/LanCartographer/issues/24).

## Build and environment

- Apple Silicon arm64, macOS 27.2.1 (26B131), Go 1.27.4 and Apple clang 21.0.0.
- Built with `bash deploy/build.sh` from `cd7576f` with the deployment
  changes committed as `739a320`. Go build metadata identifies the base and
  marks the working tree modified because the build preceded the commit.
- `python3 tests/macos-smoke.py dist` found Mach-O minimum 14.0 in both
  entry points, matching the bundle. Both passed authorization refusal,
  owned HTTP title probing, JSON export and JSON-only stdout.
- `codesign --verify --deep --strict dist/LanCartographer.app` passed.
  Both executables in the arm64 ZIP matched the loose artifacts by SHA-256.

```text
a545b90c705774165fc79749b1438b4e071ff41cb50c738c4b33d731ccdd1498  LanCartographer.app/Contents/MacOS/LanCartographer
9631d1ed3c6077cb06361f98b1cd0d7e44001fe73f0e3eeddb291f004d006bee  lancartographer-cli
b7b6fc8904754c7aa1fc85ef5838230ceba4966dbd0560ac6c659294d0412b48  LanCartographer-macos-arm64.zip
```

## Native and browser exercises

The packaged executable ran with a disposable `APPDATA`. Computer Use
inspected the painted Sessions window in light and dark themes.

1. Scanned an owned HTTP service at `127.0.0.1:18080`. The inventory showed
   one open Web port, and **Open in browser** opened the exact endpoint in
   Safari, which displayed the fixture title and body.
2. Saved the completed scan as a baseline, opened it as **Before**, and
   saw zero differences against the same **After** scan.
3. Opened and compared the synthetic pairs below. Selected one name change
   and exported the redacted preview. Each rejected file left the selection
   intact. Loading a different valid pair cleared the selection and disabled
   export until a change was selected again.
4. Confirmed replacement of the disposable report with the 65,536-host
   report. On disk it contained one aliased change out of two and no raw
   fixture addresses or names, with mode `0600`.
5. Stopped an authorized loopback-only scan. Sessions marked it cancelled
   and refused completed comparison evidence.

Each generated fixture used schema 1 and unique synthetic IPv4 addresses
within the same /15. The pairs shared source context and a one-port probe
profile. Only two names changed in each pair. To exercise byte decoding
independently of host count, the byte pair used equal-length synthetic
names to fill the payload. The oversized file appended one byte.

| Boundary | Accepted inputs per session | Rejected input |
| --- | --- | --- |
| File bytes | 8,388,608 bytes, 20,000 host records | 8,388,609 bytes, same host count |
| Host records | 65,536 hosts, about 2.30 MB | 65,537 hosts, about 2.30 MB |

## Timing and memory

A temporary Go driver called the production `session.Read`, `Compare`,
`Report` and `WriteFile` paths in three one-shot runs per pair. Medians
below are rounded to milliseconds. They include reading the fixtures from
disk but exclude UI rendering. **Preview** includes the comparison that
`Report` performs.

| Fixture pair | Open Before / After | Compare | Preview one change | Write report | Reject over-limit file |
| --- | ---: | ---: | ---: | ---: | ---: |
| Byte ceiling | 29 / 27 ms | 12 ms | 11 ms | 4 ms | 2 ms |
| Host ceiling | 28 / 27 ms | 49 ms | 50 ms | 4 ms | 24 ms |

The UI responded to subsequent clicks after each load and selection.
`vmmap -summary` on the candidate process reported a 406.9 MB peak
physical footprint after both pairs and exports. This includes the whole
desktop process and its previous work. The timings cover two differences
per pair and leave larger report selections unmeasured.

The Windows 10 desktop matrix and Windows 11 smoke remain open in #24.
This run used current Apple Silicon macOS. macOS 14 runtime and native
Intel interaction were outside this run.
