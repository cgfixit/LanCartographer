# Inventory table refresh verification (2026-11-16)

Baseline `61034552707844b18ded76bb893bb82906a44209`. The candidate
was built from that revision with the three GUI source changes in this PR.
Both builds used Go 1.27.4, Fyne 2.8.2 and CGO on an Apple Silicon Mac.
The candidate app executable's SHA-256 was
`e02b243f79f802633c88c376625b075e4dcec95266d766bd260a06ae20f9a4c1`.
The loaded process path, bundle signature, and hash of the copy placed in
the local development `dist/` were checked.

## Measurement

Five alternating baseline/candidate runs on the same machine used
`go test -c -trimpath -overlay <fixture.json> ./internal/gui` and the resulting
test binaries with `-test.run '^$' -test.bench '^BenchmarkInventory'`
`-test.benchtime=150ms -test.benchmem -test.cpu=1`. Fixtures used 254 and
1,022 unique IPv4 hosts, repeated mixed-case names, and three port records per
host. Each case checked row count, sort order and numeric IP tie-breaks. Row
preparation covered the store copy plus the table sort. The full refresh
also ran the production refresh method with Fyne's test driver.

| Case | Baseline median bytes/op | Candidate median bytes/op |
| --- | ---: | ---: |
| 254 hosts, IP order | 82,352 | 40,960 |
| 1,022 hosts, IP order | 295,344 | 147,456 |
| 1,022 hosts, Name order | 572,336 | 485,555 |

The IP case at 1,022 hosts went from eight allocations per operation to
one. Name sorting allocated fewer bytes but made more small allocations,
consistent with lowercasing names inside the comparator. A separate
baseline CPU profile showed lowercasing and memory moves in that path.
Timing ranges overlapped for row preparation and the full refresh. This
run supports an allocation reduction, with native UI and scan speed
unmeasured.

For 1,022 hosts in IP order, row preparation took a median of 349 µs
(270–1,717 µs) on the baseline and 329 µs (233–2,568 µs) on the candidate.
The full Fyne test-driver refresh took 8.15 ms (4.55–24.08 ms) and
6.17 ms (5.04–47.03 ms), respectively.

## Local acceptance

- `bash deploy/build.sh` built the native app, CLI and arm64 ZIP.
  `python3 tests/macos-smoke.py dist` verified both packaged entry points
  against an owned loopback HTTP fixture, including authorization refusal,
  an open port and page title, and matching JSON file and stdout.
- Native Computer Use loaded the same three synthetic favorites in the
  baseline and candidate. On the candidate, numeric IP order, ascending
  and descending Name order with mixed-case ties, row highlight and details,
  and totals were correct across light and dark themes. CSV export and Save
  Favorites both wrote ascending IP order during a descending Name sort.
  An unsupported export extension showed an error and created no file.
- In the candidate app, declining the confirmation preserved the loaded
  inventory. Accepting an authorized `127.0.0.1/32` scan against an owned
  HTTP fixture replaced it with one host and one open port. Sessions showed
  complete coverage, and the saved session contained the loopback host and
  HTTP title. App data was disposable.
- `gofmt`, `git diff --check`, and `go vet ./internal/gui` passed. Ten
  focused GUI tests exercised sort order, selection, favorites and snapshot
  ordering, refresh throttling and session completion.

Windows and Intel macOS native interaction, broader scan behavior and the
full local suite were outside this run. Hosted CI covers platform builds and
broader regression checks. LanCartographer has a native GUI and CLI.
