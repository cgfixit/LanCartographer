# Contributing to LanCartographer

Thanks for your interest. A few rules keep this repo reviewable and safe.

## Prerequisites

- **Go 1.27+** (module targets Go 1.27).
- **MSYS2 `mingw-w64-ucrt-x86_64-gcc`** — CGO is required
  (`CGO_ENABLED=1 CC=gcc`); use the MSYS2 **ucrt64** gcc, not plain mingw.
- **goversioninfo** — one-time install for the Windows resource embedding
  (`go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest`).
  Note: `cmd/lancartographer/resource.syso` is an **intentionally checked-in**
  binary — do not delete it and do not add `*.syso` to `.gitignore`.

## The local gate

Everything must pass locally before you open a PR. The one-liner:

```bash
bash tests/ci-gate.sh
```

or, equivalently, run the steps yourself:

```bash
gofmt -l .              # must print nothing
go vet ./...            # must pass
go generate ./...       # must produce no drift (git status clean after)
CGO_ENABLED=1 CC=gcc bash build.sh                 # CGO build
CGO_ENABLED=1 CC=gcc go test ./... -race           # race-enabled tests
```

The generated-file drift check also covers `internal/oui/oui_data.go`, which
is intentionally committed — regenerate it only when you mean to change it.

## Workflow

- **Branch naming:** `kimi/<slug>` (or the prefix your tool uses: `grok/`,
  `claude/`, `codex/`).
- **One concern per PR.** Do not mix a fix, a refactor, and a feature.
- **All PRs are opened as drafts.** Mark ready only when the local gate is
  green and the template is complete.
- **The PR template is required.** Every section must be filled, including
  exact test commands and their results — "not run" is not acceptable.
- Never commit generated noise, build outputs, logs, or secrets. `go.sum`
  is committed and stays committed.

## Style

- `gofmt` is implicit — if `gofmt -l .` prints your file, the PR is red.
- Test-driven development: a behavior change ships with the test that proves
  it.
- Code using `unsafe` (especially in `internal/winutil`) carries a comment
  explaining the memory-safety invariant it relies on and why it is sound.

## Areas requiring extra justification

- **`internal/winutil`** — the memory-unsafe Windows API surface. Any change
  here needs a careful review of pointer lifetimes, buffer sizes, and the
  `unsafe.Pointer` / `uintptr` conversions involved. Explain the safety
  argument in the PR.
- **`internal/authz` and engine routing** — this is the authorization gate.
  It fails closed by design. Any change that could broaden access, skip a
  gate, or fail open needs explicit justification of why the new behavior is
  still fail-closed, plus a test proving it.

## The authorization notice

Anything touching scan behavior must preserve the CFAA authorization
sentence verbatim wherever it appears:

> Scan only networks you own or have explicit authorization to inventory. Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws.

Removing or weakening this notice is not acceptable in a routine PR; if you
believe it should change, raise that as its own dedicated discussion first.

## Trademarks and credits

LanCartographer is original work inspired by Advanced IP Scanner (Famatech),
Angry IP Scanner, and Nmap. Keep the credits accurate and never imply
affiliation with those projects.
