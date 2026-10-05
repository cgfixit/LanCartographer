#!/usr/bin/env bash
# ci-gate.sh — local emulation of the CI gate for LanCartographer.
# Run from the repository root: bash tests/ci-gate.sh
set -euo pipefail

cd "$(dirname "$0")/.."

# Toolchain paths (Windows dev machine): Go + MSYS2 ucrt64 gcc for CGO.
export PATH="/c/Program Files/Go/bin:$PATH"
export PATH="/c/msys64/ucrt64/bin:$PATH"

FAILED=()

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
GENERATED=(
    internal/oui/oui_data.go
    cmd/lancartographer/resource.syso
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
step "CGO go build ./..." env CGO_ENABLED=1 CC=gcc go build ./...
step "CGO go test ./... -race" env CGO_ENABLED=1 CC=gcc go test ./... -race

echo
if [ "${#FAILED[@]}" -eq 0 ]; then
    echo "CI GATE: PASS (all 5 steps)"
    exit 0
else
    echo "CI GATE: FAIL — failed steps:"
    printf '  - %s\n' "${FAILED[@]}"
    exit 1
fi
