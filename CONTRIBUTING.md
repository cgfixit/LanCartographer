# Contributing to LanCartographer

LanCartographer requires Go 1.27.1 and CGO. Windows builds use MSYS2 UCRT64
GCC and `goversioninfo` v1.7.0; macOS builds use Xcode Command Line Tools and
system clang. See the [README](README.md) for setup details.

Keep each branch and pull request to one concern, and open pull requests as
drafts. Run the changed behavior and the relevant formatting, vet, build, or
workflow checks locally; record exact commands, results, and anything left to
hosted CI. Use `bash deploy/build.sh` for native packaging. On macOS, exercise
the package with `python3 tests/macos-smoke.py dist` when packaging or runtime
behavior changes. The full Windows-oriented gate is `bash tests/ci-gate.sh`,
but it is not required for every local change.

Session work must keep full saved JSON distinct from selected, redacted reports.
Use synthetic records and disposable data directories for parser/comparison
checks; exercise changed dialogs in the native app. Preserve rejection of
incompatible or incomplete comparisons, and record screenshots without real
LAN data. Update the [User Guide](USER-GUIDE.md), README and relevant privacy/
update guidance when changing the workflow or file format. Keep dated verification
records tied to what was actually exercised; add new evidence rather than
rewriting old results as current acceptance.

Do not accidentally modify or remove the generated, tracked files
`cmd/lancartographer/resource.syso` and `internal/oui/oui_data.go`. Never
commit build output, logs, caches, credentials, or real LAN inventory.

Changes must preserve the authorization gate, zero telemetry, platform
security boundaries, and this notice wherever it is referenced:

> Scan only networks you own or have explicit authorization to inventory. Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws.

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).
