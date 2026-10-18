# How to update LanCartographer

LanCartographer has no installer and no in-app updater.
You replace the files you run, or you rebuild this tree.
Quit the app before you replace them.

Per-user data is not inside the binary. Leave this directory in place when you update:

- Windows: `%AppData%\LanCartographer\`
- macOS: `~/Library/Application Support/LanCartographer/`

That directory holds `settings.json`, `favorites.json`, `names.json`, and `firstrun_seen`. An absolute `APPDATA` value overrides the location. The app does not write this data into its working directory.

## Replace a downloaded build

1. Download the asset for this computer from [GitHub Releases](https://github.com/cgfixit/LanCartographer/releases).
2. On Windows, replace `LanCartographer.exe` and `lancartographer-cli.exe`.
3. On macOS, use `LanCartographer-macos-arm64.zip` on Apple Silicon or `LanCartographer-macos-amd64.zip` on Intel. Replace `LanCartographer.app` and `lancartographer-cli` from that archive.
4. If macOS blocks a downloaded app, approve it in Privacy & Security. The bundle is ad-hoc signed. It is not Developer ID signed or notarized. Do not disable Gatekeeper.

## Rebuild from this tree

The Go version is the `go` line in `go.mod` (currently 1.27.1). Windows also needs MSYS2 ucrt64 gcc and `goversioninfo`. macOS needs the Xcode Command Line Tools. Both builds enable CGO because the GUI uses Fyne.

From the repository root:

```bash
bash deploy/build.sh
```

Windows output is `dist/LanCartographer.exe` and `dist/lancartographer-cli.exe`. macOS output is `dist/LanCartographer`, `dist/lancartographer-cli`, `dist/LanCartographer.app`, and a zip named for the architecture. `deploy/build.sh` moves the checked-in Windows `resource.syso` aside on macOS and restores it on exit. `dist/` is build output and is not committed.

For the code tour, read [USER-GUIDE.md](USER-GUIDE.md).
