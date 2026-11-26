# Inventory row selection verification, 2026-11-26

Both packages were built on Apple Silicon with macOS 27.2.1 (26B131),
Go 1.27.4 and Apple clang 21.0.0. The baseline was
`efac140218721dd98ea943ee548e9693e61044e0`. The candidate used that revision
with the selection fix applied. The exercised Go source blobs were
`abeed813cab8c58cbeb7db217ff509049b92625b` (gui.go) and
`e2ab65435eb5128651fedc6911fb6570b6339147` (table.go).

| Artifact | SHA-256 |
| --- | --- |
| Baseline app executable | `b4d28e51390ce4e03526a32e123eea010b7f9342e8f1a0039491a7e09b35fe6c` |
| Candidate app executable | `c52036081bc6afdd97176305c343bdef781f25834fd7efcee74bb554c130bc2e` |
| Candidate CLI | `3efb1ca0757bd311427eb96667bfc3a8c59a88a14f1b29a0e1bb2e22e9926a22` |

## Native app

Launched each packaged executable with a disposable `APPDATA` and used
Computer Use to load two synthetic IPv4 favorites. Clicked the first row's
Name cell (Alpha), reversed the IP sort and clicked the same cell (Bravo).
The baseline kept Alpha's details and showed a separate marker on Bravo's
cell. The candidate showed Bravo's details and highlighted only Bravo's row
in light and dark themes.

| Baseline | Candidate, light | Candidate, dark |
| --- | --- | --- |
| [Two highlights, Alpha details](inventory-row-selection/before.jpg) | [Bravo selected](inventory-row-selection/light.jpg) | [Bravo selected](inventory-row-selection/dark.jpg) |

Declining a scan of `127.0.0.1/32` preserved the favorites and produced zero
fixture HTTP requests. Approving the scan found the owned HTTP fixture at port
8080 and reset the selection. Selecting the result showed its port and HTTP
title. The Browser action reached the fixture. In a private Safari window,
the fixture title and heading rendered at `http://127.0.0.1:8080/`.
The port's Check Service action prefilled the same endpoint and showed a
successful TCP connection and HTTP 200 after confirmation.

## Packaged and focused checks

```bash
bash build/build.sh
codesign --verify --deep --strict dist/LanCartographer.app
python3 build/macos-smoke.py dist
gofmt -l .
CGO_ENABLED=1 CC=clang go vet ./...
CGO_ENABLED=1 CC=clang go test ./internal/gui \
  -run 'TestTableTapSelects|TestLoadFavoritesAllowsReselectingSameCell|TestConnections.*Inventory' -count=1
STATICCHECK_CACHE="$scratch/staticcheck-cache" CGO_ENABLED=1 CC=clang \
  "$scratch/staticcheck" ./internal/gui
```

The candidate passed these checks with staticcheck 0.8.2. The same-cell
regression failed against the baseline source with Alpha still in the
details pane. Packaged smoke covered both CLI entry paths, authorization
refusal, JSON stdout and file export, the deployment target and the license.
Native Windows and Intel macOS were outside this run. Hosted CI covers builds
and automated checks for those platforms. Windows desktop acceptance is
tracked in [#80](https://github.com/cgfixit/LanCartographer/issues/80).
