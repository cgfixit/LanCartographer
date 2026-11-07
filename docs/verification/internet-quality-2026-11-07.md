# Internet Quality verification

November 7, 2026. Source commit `992cef1` on `453ae90`. Subsequent evidence
and focused-test edits leave the packaged source unchanged. Go 1.27.3, Fyne
2.8.2, macOS 27.2 build 26B123 on arm64. The signed bundle executable and
maintained development copy matched SHA-256
`e9ab43f678a2e0d1b9b5cf008c071117a3e654d26deed848bef4b0bdfa94108b`.

## Local checks

| Check | Result |
| --- | --- |
| `bash deploy/build.sh` | Built and ad-hoc signed the arm64 app and built the CLI and ZIP. |
| `python3 tests/macos-smoke.py dist` | Verified bundle minimum metadata and license. Both packaged CLI entry points refused an unconfirmed loopback scan, then inventoried the owned HTTP fixture and exported JSON after authorization. |
| `go test ./internal/gui -run '^TestInternetQuality' -count=1` | Exercised real child processes with valid and legacy JSON, structured offline error with exit zero, malformed or incomplete data, missing executable, timeout and output overflow. Exercised the GUI scan entry point with a pending job and confirmed the approved scan waits for teardown. |
| `go vet ./internal/gui`, `git diff --check` | Passed. |

The shell sandbox prevented `iconutil` from creating the bundle icon from
correctly sized PNGs. The supported build command completed outside that
sandbox without any packaging changes.

## Native observations

- The rebased package opened **Tools → Internet Quality…** at Ready.
  Keyboard navigation opened and confirmed the Apple disclosure. The
  provider completed live and the window painted download, upload, idle
  latency, RPM, interface and observation time. A separate same-run Apple
  summary and JSON qualification confirmed the JSON throughput is in bits
  per second on this OS. The UI uses decimal Mbps. Apple's human summary
  divided the observed integer values by 1,048,576.
- On the earlier candidate with the same measurement and Stop logic, Stop
  rendered Cancelled and removed the observed child. On the earlier
  candidate with the final Close and browser code, Close removed an
  observed child and left inventory open. Reopening returned to Ready.
- On the rebased package, the separate Cloudflare disclosure preceded a
  successful handoff to `https://speed.cloudflare.com/` in Safari. The
  Cloudflare measurement was left idle.
- The rebased GUI accepted an owned `127.0.0.1/32` scan, showed the owned
  HTTP fixture on port 8080, reached 100%, opened the inventory export
  dialog and populated Sessions from the completed scan. GUI export was
  inspected through its save dialog. Packaged CLI smoke exercised the write.

[Before](internet-quality/before.jpg) shows the baseline inventory toolbar
cropped above the host rows. [After](internet-quality/after.jpg) shows the
fresh packaged diagnostic at Ready. Neither screenshot contains a provider
result or discovered host data.

## Remaining coverage

The command fixtures cover missing executable, offline error, malformed
output and timeout without changing host network or system binaries.
Native minimum macOS 13, Intel, Windows GUI, and downloaded release
acceptance remain unverified. The archived macOS 13 manual and the current
manual describe different throughput units, so same-run minimum-OS
calibration remains an acceptance item in
[#33](https://github.com/cgfixit/LanCartographer/issues/33). Keep the live
Apple and browser tests out of hosted CI. The hosted jobs cover their
configured build and test matrix.
