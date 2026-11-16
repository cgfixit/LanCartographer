#!/usr/bin/env bash
# ci-gate.sh — local emulation of the CI gate for LanCartographer.
# Run from the repository root: bash tests/ci-gate.sh
set -euo pipefail

cd "$(dirname "$0")/.."

# Toolchain paths (Windows dev machine): Go + MSYS2 ucrt64 gcc for CGO.
# Harmless on macOS, where these directories do not exist.
export PATH="/c/Program Files/Go/bin:$PATH"
export PATH="/c/msys64/ucrt64/bin:$PATH"

FAILED=()

# resource.syso is a Windows COFF object; Apple's ld rejects it, so the macOS
# leg of ci.yml moves it aside. Do the same here and always put it back.
# macOS uses system clang (AGENTS.md §4); Windows uses MSYS2 gcc.
GATE_CC=gcc
SYSO=cmd/lancartographer/resource.syso
if [ "$(uname -s)" = "Darwin" ]; then
    GATE_CC=clang
fi
if [ "$(uname -s)" = "Darwin" ] && [ -f "${SYSO}" ]; then
    SYSO_BACKUP="$(mktemp "${TMPDIR:-/tmp}/resource.syso.XXXXXX")"
    mv "${SYSO}" "${SYSO_BACKUP}"
    trap 'mv "${SYSO_BACKUP}" "${SYSO}"' EXIT
fi

# Non-GUI packages: Fyne needs CGO, so only these cross-compile (matches ci.yml).
cross_compile() {
    local packages
    packages="$(go list ./... | grep -vE 'internal/gui|cmd/lancartographer')"
    # shellcheck disable=SC2086 # word-split the package list on purpose
    CGO_ENABLED=0 GOOS="$1" GOARCH="$2" go build ${packages}
}

step() {
    local name="$1"
    shift
    echo "==> ${name}"
    if "$@"; then
        echo "PASS: ${name}"
    else
        echo "FAIL: ${name}"
        FAILED+=("${name}")
    fi
}

gofmt_check() {
    local out
    out="$(gofmt -l .)"
    if [ -n "${out}" ]; then
        echo "gofmt found unformatted files:"
        echo "${out}"
        return 1
    fi
}

# Generated artifacts whose bytes must be reproducible from the checked-in
# sources. Hash them instead of diffing the tree so the check works on a
# dirty working copy (uncommitted edits are not "drift").
# NOTE: resource.syso is intentionally NOT listed — its bytes depend on the
# goversioninfo build version, which varies between developer machines and
# CI. It is still exercised: `go generate` must succeed and the Windows
# build embeds it. The drift check guards the deterministic Go-generated
# files below.
GENERATED=(
    internal/oui/oui_data.go
    cmd/lancartographer/app.ico
)

generate_drift_check() {
    local before after
    before="$(sha256sum "${GENERATED[@]}")"
    go generate ./...
    after="$(sha256sum "${GENERATED[@]}")"
    if [ "${before}" != "${after}" ]; then
        echo "go generate ./... changed checked-in artifacts:"
        diff <(echo "${before}") <(echo "${after}") || true
        echo "Regenerate and commit, or fix the generator for byte-stable output."
        return 1
    fi
}

step "gofmt -l . is empty" gofmt_check
step "go vet ./..." go vet ./...
step "go generate ./... produces no drift" generate_drift_check
step "CGO go build ./..." env CGO_ENABLED=1 CC="${GATE_CC}" go build ./...
step "CGO go test ./... -race" env CGO_ENABLED=1 CC="${GATE_CC}" go test ./... -race
step "cross-compile darwin/arm64 (non-GUI)" cross_compile darwin arm64
step "cross-compile linux/amd64 (non-GUI)" cross_compile linux amd64

echo
if [ "${#FAILED[@]}" -eq 0 ]; then
    echo "CI GATE: PASS (all 7 steps)"
    exit 0
else
    echo "CI GATE: FAIL — failed steps:"
    printf '  - %s\n' "${FAILED[@]}"
    exit 1
fi
