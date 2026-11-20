# macOS diagnostics closeout

Verified the combined Connections and Route & DNS branch at `6a76f9c`
on macOS 27.2.1 (26B131) arm64 with Go 1.27.4. The native signed
executable SHA-256 was
`a366c9d9c584851f6e1c9c57c9c1f50e9d586ae4fe30e5eee18a1a8ba4350090`.
The app ran with a disposable `APPDATA`.

- **Tools** showed direct entries for Check Service, Internet Quality,
  Connections, and Route & DNS. The owner chose to retain those entries
  for [#33](https://github.com/cgfixit/LanCartographer/issues/33).
- Internet Quality opened at Ready. Declining the disclosure returned to
  Ready. After approval on an unmetered connection, one run reached the
  app's deadline and showed Timed out. The app reaped its
  `/usr/bin/networkQuality -c` child. A bounded direct invocation of the
  same helper completed in 27.9 seconds with the required metrics.
- A second approved run in a fresh instance reached Complete. The window
  displayed download and upload in Mbps, idle latency in ms, and
  responsiveness in RPM. The helper exited. The installed manual specifies
  bits per second for both throughput fields. Earlier same-run unit
  calibration and native Stop and Close evidence are recorded separately.
  [Internet Quality evidence](internet-quality-2026-11-07.md)
- Declining the owned loopback scan left inventory at Ready. On confirmation
  the app found the local HTTP service on port 18080 and wrote a CSV with
  exactly one loopback row. Sessions opened with the same completed scan.
  The preceding feature verification covers the Safari handoff and light
  and dark rendering. [Connections evidence](connections-2026-11-20.md)
  [Route & DNS evidence](route-dns-2026-11-20.md)

```sh
bash deploy/build.sh
python3 tests/macos-smoke.py dist
go test -race ./internal/gui ./internal/macdiag ./internal/servicecheck -run '^(TestInternetQuality|TestDiagnosticCommand|TestConnections|TestRoute|TestCheckConnectOutcomes|TestCheckDoesNotFollowRedirects|TestCheckTLSFailuresNameTheCause|TestResolveFiltersAndBounds)' -count=1
```

The package and focused checks passed. Both packaged entry points passed
the smoke test and declared macOS 14.0. For #33 the owner waived the macOS
14 runtime check and deferred Windows desktop acceptance. The hosted
macOS Intel result covers build and CI checks. Native Intel interaction
was unavailable.
