#!/usr/bin/env bash
# LanCartographer release build: regenerates version resources (icon +
# manifest via goversioninfo) and builds both binaries into dist/.
#
# Toolchain: Go 1.22+ on PATH, plus MSYS2 mingw-w64 gcc for CGO (Fyne).
# Install the manifest tool once with:
#   go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
set -euo pipefail

cd "$(dirname "$0")"

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
