#!/usr/bin/env bash
# LanCartographer release build: regenerates version resources (icon +
# manifest via goversioninfo) and builds both binaries into dist/.
#
# On macOS (uname -s = Darwin) it regenerates the icon only (the
# goversioninfo resource is Windows-only) and builds unflagged Mach-O
# binaries — see the Darwin branch below.
#
# Toolchain: Go 1.27+ on PATH, plus MSYS2 mingw-w64 gcc for CGO (Fyne) on
# Windows; on macOS the system clang is used. Install the manifest tool
# once with:
#   go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
set -euo pipefail

cd "$(dirname "$0")"

# macOS branch: icongen is the only generator that runs (the goversioninfo
# directive is windows-tagged), and Fyne links against the system clang.
if [ "$(uname -s)" = "Darwin" ]; then
    export CGO_ENABLED=1

    echo "==> go generate ./... (icon only; version resources are Windows-only)"
    go generate ./...

    echo "==> building dist/LanCartographer (GUI)"
    go build -trimpath -ldflags "-s -w" -o dist/LanCartographer ./cmd/lancartographer

    echo "==> building dist/lancartographer-cli (CLI)"
    go build -trimpath -ldflags "-s -w" -o dist/lancartographer-cli ./cmd/lancartographer

    echo "==> done"
    ls -l dist/
    echo
    echo "NOTE: these binaries are unsigned. If downloaded via browser, Gatekeeper"
    echo "will quarantine them — right-click > Open, or run:"
    echo "  xattr -d com.apple.quarantine dist/LanCartographer dist/lancartographer-cli"
    exit 0
fi

# goversioninfo must be resolvable by `go generate`.
export PATH="$PATH:$(go env GOPATH | tr '\\' '/')/bin"

export CGO_ENABLED=1
export CC=gcc

echo "==> go generate ./... (icon + version resources)"
go generate ./...

echo "==> building dist/LanCartographer.exe (GUI)"
go build -trimpath -ldflags "-s -w -H windowsgui" -o dist/LanCartographer.exe ./cmd/lancartographer

echo "==> building dist/lancartographer-cli.exe (CLI)"
go build -trimpath -ldflags "-s -w" -o dist/lancartographer-cli.exe ./cmd/lancartographer

echo "==> done"
ls -l dist/
