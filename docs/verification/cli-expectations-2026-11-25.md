# CLI expectations verification, 2026-11-25

The packaged Apple Silicon build used macOS 27.2.1 (26B131), Go 1.27.4
and Apple clang 21.0.0. The build was based on
`984d1fcbce81fce20e9c707c0239f61f678323f0` with the CLI patch applied.
The Go build metadata marks the tree as modified because the build preceded
the commit. The application source blobs exercised were
`e824fbd2fb32909ea0eec2abc3233c9d36876c76` (`cli.go`) and
`566b7be67fa5ecfbd962ecb5e199a70697f1c8a3` (`expectations.go`).

| Artifact | SHA-256 |
| --- | --- |
| `LanCartographer.app/Contents/MacOS/LanCartographer` | `4659a8782f28104127107233908fa5f7ea59029a9a93a338cdb1bc26c16fec64` |
| `lancartographer-cli` | `e2faba4fb746a68e7f4a3313d1bde1688269427a3c837a6e23333dfaf4901d24` |

## Packaged CLI

Built the baseline CLI and packaged the candidate with `bash build/build.sh`.
The candidate passed `codesign --verify --deep --strict dist/LanCartographer.app` and
`python3 build/macos-smoke.py dist` on the standalone CLI and the bundled
app's `--cli` path. The smoke test also verified the Mach-O minimum of macOS
14 and packaged license files.

Used an owned HTTP listener on `127.0.0.1:8080`, a loopback port confirmed
to refuse connections, and disposable `APPDATA`. The refused test port was
65103 on this run. The baseline rejected `--expect` as an unknown flag.
With the flag omitted, the baseline and candidate returned the same parsed
inventory for the loopback fixture.

```bash
dist/lancartographer-cli --range 127.0.0.0/30 --ports 8080,65103 \
  --timeout 300ms --i-am-authorized --expect met.json --json -
dist/LanCartographer.app/Contents/MacOS/LanCartographer --cli \
  --range 127.0.0.0/30 --ports 8080,65103 --timeout 300ms \
  --i-am-authorized --expect met.json --json -
```

| Fixture or input | Observation |
| --- | --- |
| `met.json` expects 8080 open and 65103 closed on `127.0.0.1`, and `127.0.0.2` absent | Both entry points returned 0 and printed three met results on stderr. Stdout matched the candidate's plain JSON inventory byte for byte. |
| Expect 8080 closed, or expect `127.0.0.1` absent | Exit 4 with the open port or observed host reported. JSON stdout and file output remained usable. |
| `--range 127.0.0.1/32 --ports 65103 --timeout 1ns`, expect 65103 closed | macOS ICMP discovery reached the host. The intentionally short TCP deadline yielded `timeout` in JSON and exit 4. |
| Outside IPv4 address, broadcast address, unselected port, unknown JSON field, same input and export path, or omitted authorization | Exit 2 with empty stdout and zero HTTP fixture requests. The same-path refusal preserved the input SHA-256. |
| SIGINT sent after the fixture received the HTTP probe and before it answered | The process terminated by SIGINT with empty stdout and no expectation verdict. |

## Native app and browser

Launched the verified signed bundle from the development output directory
with isolated `APPDATA` and confirmed the running executable path. Used
Computer Use to acknowledge the first-run notice, decline a scan of
`127.0.0.1/32`, and then approve it. The declined run kept the inventory
empty. The approved run displayed one host and one open web port in light
and dark themes. The host's Browser action rendered the fixture in Safari at
`127.0.0.1:8080`. Its port-level Check Service action prefilled the same
endpoint and returned TCP connected and HTTP 200 after confirmation.

## Focused checks

```bash
go test ./internal/cli -run 'TestExpectations|TestJSONStdout|TestConfirmationRefusal' -count=1
CGO_ENABLED=1 go vet ./...
GOTOOLCHAIN=$(go env GOVERSION) CGO_ENABLED=1 go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./internal/cli
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c ./internal/cli -o /tmp/lancartographer-cli.test.exe
actionlint .github/workflows/*.yml
```

All checks passed. The focused cases also verified that invalid expectations
never called the scan runner and that a cancelled or incomplete scan never
issued a verdict. Native desktop acceptance here covers Apple Silicon on
the listed OS build. The Windows cross-compile establishes compilation.
Hosted CI covers Windows and both Mac architectures. Windows desktop
acceptance is tracked in #80.
