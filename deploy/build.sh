#!/usr/bin/env bash
# LanCartographer release build: regenerates version resources (icon +
# manifest via goversioninfo) and builds both binaries into dist/.
#
# On macOS (uname -s = Darwin) it regenerates the icon only (the
# goversioninfo resource is Windows-only) and builds unflagged Mach-O
# binaries — see the Darwin branch below.
#
# Toolchain: the Go version from go.mod on PATH, plus MSYS2 mingw-w64 gcc for CGO (Fyne) on
# Windows; on macOS the system clang is used. Install the manifest tool
# once with:
#   go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.7.0
set -euo pipefail

# Run from the repository root (this script lives in deploy/).
cd "$(dirname "$0")/.."

# macOS branch: icongen is the only generator that runs (the goversioninfo
# directive is windows-tagged), and Fyne links against the system clang.
if [ "$(uname -s)" = "Darwin" ]; then
    export CGO_ENABLED=1
    # Match Info.plist instead of inheriting the build host's newer minimum.
    export MACOSX_DEPLOYMENT_TARGET=14.0
    # Explicit flags also invalidate Go's cached CGO objects built on a
    # newer host; MACOSX_DEPLOYMENT_TARGET alone is not in that cache key.
    export CGO_CFLAGS="${CGO_CFLAGS:--O2 -g} -mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
    export CGO_CXXFLAGS="${CGO_CXXFLAGS:--O2 -g} -mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
    export CGO_LDFLAGS="${CGO_LDFLAGS:--O2 -g} -mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"

    echo "==> go generate ./... (icon only; version resources are Windows-only)"
    go generate ./...

    # The checked-in resource.syso is a Windows COFF object; the go tool feeds
    # .syso files to the system linker on every OS and Apple's ld rejects
    # COFF. Move it aside for the builds and restore it afterwards.
    SYSO=cmd/lancartographer/resource.syso
    mv "$SYSO" "$SYSO.hold"
    trap 'mv "$SYSO.hold" "$SYSO"' EXIT

    echo "==> building dist/LanCartographer (GUI)"
    go build -trimpath -ldflags "-s -w" -o dist/LanCartographer ./cmd/lancartographer

    # Both entry points have identical flags on macOS; argv[0] selects CLI.
    # Replace the inode so macOS does not reuse an old cached code signature.
    install -S -m 755 dist/LanCartographer dist/lancartographer-cli

    echo "==> packaging dist/LanCartographer.app"
    CONTENTS=dist/LanCartographer.app/Contents
    mkdir -p "$CONTENTS/MacOS" "$CONTENTS/Resources" dist/AppIcon.iconset
    cp dist/LanCartographer "$CONTENTS/MacOS/LanCartographer"
    cp packaging/macos/Info.plist "$CONTENTS/Info.plist"
    cp LICENSE "$CONTENTS/Resources/LICENSE"
    for size in 16 32 128 256 512; do
        sips -z "$size" "$size" cmd/lancartographer/app_preview.png \
            --out "dist/AppIcon.iconset/icon_${size}x${size}.png" >/dev/null
        double=$((size * 2))
        sips -z "$double" "$double" cmd/lancartographer/app_preview.png \
            --out "dist/AppIcon.iconset/icon_${size}x${size}@2x.png" >/dev/null
    done
    iconutil -c icns dist/AppIcon.iconset -o "$CONTENTS/Resources/AppIcon.icns"
    plutil -lint "$CONTENTS/Info.plist"
    # Ad-hoc signing makes the bundle internally valid; this is NOT a
    # Developer ID signature or notarization and does not bypass Gatekeeper.
    codesign --force --sign - dist/LanCartographer.app
    codesign --verify --deep --strict dist/LanCartographer.app

    ARCH="$(go env GOARCH)"
    cp LICENSE dist/LICENSE
    (cd dist && shasum -a 256 LanCartographer lancartographer-cli LICENSE > SHA256SUMS.txt)
    # -c creates a fresh archive, avoiding stale entries from older builds.
    PACKAGE="$(mktemp -d)"
    cp -R dist/LanCartographer.app dist/lancartographer-cli dist/LICENSE "$PACKAGE/"
    # Checksums refer to the executable inside the shipped app bundle.
    (cd "$PACKAGE" && shasum -a 256 LanCartographer.app/Contents/MacOS/LanCartographer lancartographer-cli LICENSE > SHA256SUMS.txt)
    ditto -c -k --sequesterRsrc "$PACKAGE" "dist/LanCartographer-macos-${ARCH}.zip"
    rm -rf "$PACKAGE"

    echo "==> done"
    ls -l dist/
    echo
    echo "NOTE: the app is ad-hoc signed, not Developer ID signed or notarized."
    echo "Downloaded builds may require approval in macOS Privacy & Security."
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

cp LICENSE dist/LICENSE

echo "==> done"
ls -l dist/
