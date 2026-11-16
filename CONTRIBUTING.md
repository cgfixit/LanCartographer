# Contributing to LanCartographer

LanCartographer requires the Go version in [go.mod](go.mod) and CGO. Windows builds use MSYS2 UCRT64
GCC and `goversioninfo` v1.7.0; macOS builds use Xcode Command Line Tools and
system clang. See the [README](README.md) for setup details.

Keep each branch and pull request to one concern, and open pull requests as
drafts. Run the changed behavior and the relevant formatting, vet, build, or
workflow checks locally; record exact commands, results, and anything left to
hosted CI. Use `bash deploy/build.sh` for native packaging. On macOS, exercise
the package with `python3 tests/macos-smoke.py dist` when packaging or runtime
behavior changes. The full local gate (Windows or macOS) is `bash tests/ci-gate.sh`,
but it is not required for every local change.

Session work must keep full saved JSON distinct from selected, redacted reports.
Use synthetic records and disposable data directories for parser/comparison
checks; exercise changed dialogs in the native app. Preserve rejection of
incompatible or incomplete comparisons, and record screenshots without real
LAN data. Update the [User Guide](docs/USER-GUIDE.md), README and relevant privacy/
update guidance when changing the workflow or file format. Keep dated verification
records tied to what was actually exercised; add new evidence rather than
rewriting old results as current acceptance.

Do not accidentally modify or remove the generated, tracked files
`cmd/lancartographer/resource.syso` and `internal/oui/oui_data.go`. Never
commit build output, logs, caches, credentials, or real LAN inventory.

## Releases

The [macOS main prerelease workflow](.github/workflows/release-main-macos.yml)
publishes the Apple Silicon and Intel ZIPs built by CI for each pull request
merged into `main`. It waits at least five minutes after CI, Gitleaks, and
CodeQL pass on the same commit, then publishes both ZIPs under
`main-<commit SHA>`. Runner queue time and uploading can add to the delay.
Use a `v*` tag for a versioned Windows and macOS release.

GitHub can reject tag creation if a newer merge changes workflow files before
an older commit is released. In that case, create `main-<commit SHA>` on the
verified commit with a workflow-authorized account and rerun the release job.

## Dependency maintenance

`go.mod` owns the Go minimum and selected module requirements; commit its
generated `go.sum` changes together. `go.sum` verifies downloaded content and
is not a complete lockfile. CI reads the Go version from `go.mod`. Windows
resource generation separately pins `goversioninfo` in the workflows and build
instructions; GitHub Actions remain pinned to full commit SHAs. Dependabot's
weekly Go/action configuration is in [.github/dependabot.yml](.github/dependabot.yml).

Inspect current versions and upstream release notes before choosing explicit
`go get module@version` updates. Keep Fyne's related dependency changes together;
do not treat every newer transitive or upstream test-tool version as a required
desktop upgrade. After updating, run:

```bash
go mod tidy
go mod verify
go mod tidy -diff
```

Use `go list -mod=readonly -m -u all` to review remaining versions. Exercise the
native package and owned CLI fixture after Go/runtime dependency changes, and
native dialogs/themes after a Fyne update. Record source, module versions and
the executable hash in [docs](docs/README.md); prior screenshots and CI results
do not verify the updated artifact. Workflow changes need actionlint and
review of action runtime, permissions, inputs and artifact compatibility.
Remaining indirect/action major upgrades are tracked in
[issue #25](https://github.com/cgfixit/LanCartographer/issues/25).

Keep deferred cleanup proposals in
[issue #26](https://github.com/cgfixit/LanCartographer/issues/26), with a concrete
benefit and preserved behavior before implementing them. Retain historical
verification records rather than deleting them because they are old.

## Safety and reporting

Changes must preserve the authorization gate, zero telemetry, platform
security boundaries, and this notice wherever it is referenced:

> Scan only networks you own or have explicit authorization to inventory. Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws.

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).
