# Scan bounds verification

Run on Linux amd64 (cloud container) with Go 1.27.4 and Python 3.13.18, on
`claude/fervent-shannon-ag1wgo` cut from `main` at `d8fb33b`. The only
scanned address was `127.0.0.1`, with an owned `python3 -m http.server`
listener on `127.0.0.1:18080`.

## Method

The non-CGO binary was built before and after the change and run as:

```sh
CGO_ENABLED=0 go build -o lancartographer-cli ./cmd/lancartographer
lancartographer-cli --range 127.0.0.1/32 --ports 18080 --timeout <T> --i-am-authorized
```

## Results

| `--timeout` | Before | After |
| --- | --- | --- |
| `400ms` | `127.0.0.1` with `18080 open Web`, exit 0 | identical |
| `0s` | accepted, same row, exit 0; `net.Dialer` then has no timeout | `--timeout must be greater than zero, got 0s`, exit 2, nothing scanned |
| `-1ms` | empty table, exit 0, even though 18080 is open | `--timeout must be greater than zero, got -1ms`, exit 2, nothing scanned |

The `0s` row shows only that the value was accepted. On loopback a dial
answers at once, so the unbounded wait is not visible. It would show only
against an address that drops SYNs, and none was dialed. The no-timeout
behavior comes from the `net.Dialer` documentation.

Stop after the port scan: `TestRun_StopAfterPortScanSkipsEnrichment`
cancels the sweep from inside the port scan of a /30. Without the engine
change it fails with `lookups=[mac netbios mac] hostEvents=1`: MAC and name
lookups ran after Stop, and one host event still reached the consumer. With
the change, no lookup starts and no host event is sent.

```sh
gofmt -l .
go vet ./internal/cli/ ./internal/scan/
GOTOOLCHAIN=go1.27.4 go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./internal/cli/ ./internal/scan/
go test -race -count=1 ./internal/scan/
go test -race -count=1 -run 'TestNonPositiveTimeoutExits2|TestLoopbackEndToEnd|TestBad|TestUnknownFlag' ./internal/cli/
```

All passed.

## Not exercised

- How long Stop takes in the native app on macOS (an `arp -a` subprocess
  per in-flight host before this change) or on Windows (`SendARP`). The
  saving was not timed. The engine test shows only that the lookups no
  longer start.
- The GUI, which sets fixed 400 ms and 800 ms timeouts and does not take
  `--timeout`.
