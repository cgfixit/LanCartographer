# TCP port observation verification, 2026-11-27

Verified on Apple Silicon with macOS 27.2.1 (26B131), Go 1.27.4 and Apple
clang 21.0.0. The baseline was
`5e1868c82867ece12c44fe0540b4a9810f6a6969`. The candidate used that
revision with these exercised source blobs:
`9f1de830aaee8a87ba6c54975d523022c605c1bd` (probe),
`ab5dadda82f3a0158bdc1a7ef03d5b4286e38ba1` (panel) and
`496486f5bc4b306952385be992dfc373b1266bb1` (session).

| Artifact | SHA-256 |
| --- | --- |
| Baseline app executable | `b8cb4d10377179d17d8f7b7a0f26cd336f4643987ed62eb50c38208d1aeb3780` |
| Baseline CLI | `e874840ef78f6dbe63c0889d6c2219547972387fd2c790624cef46e074a117b5` |
| Candidate app executable | `92db562ac365e29d489b84c3dc4b561466a29825c7f263b0f8f15f91b46beb3c` |
| Candidate CLI | `32f47729561adf7fc45fd92422348f053ddacb3f04958cc46120da2ab09b2684` |

## Packaged CLI

Used separate, disposable `APPDATA`. Both builds met an open expectation
on the owned HTTP listener at `127.0.0.1:8080` and a closed expectation on
a loopback port verified to return connection refused. With consent omitted,
the candidate returned 2 with empty stdout and zero fixture connections.

For the resource-failure check, a Python fixture opened 96 extra listeners
on `127.0.0.1`, each holding HTTP responses briefly. It ran the packaged
CLI with a 48-file-descriptor soft and hard limit on the child process only.
Each of the 96 listening ports was declared closed in the expectations
file. Scans used `--range 127.0.0.1/32 --timeout 400ms --concurrency 128`
and the fixture's port list.

| Run | Observed result |
| --- | --- |
| Baseline, JSON stdout | 27 listening ports incorrectly met a closed expectation. The other ports made the overall run exit 4. |
| Candidate, JSON stdout | Exit 4, 39 unconfirmed results, and zero closed expectations met. JSON contained all probed port records. |
| Candidate, JSON file | Exit 4, 21 unconfirmed results, and zero closed expectations met. The file contained all probed port records. |

The counts describe individual runs under concurrent scheduling. A focused
CLI check also verified exit 4 when the sole assertion expected a port
closed and its state was unconfirmed.

## Native app and browser

Loaded the same synthetic favorites in both packaged apps. The baseline
grouped the unconfirmed port under **Closed**. The candidate displayed
**Closed**, **No answer** and **Unconfirmed** on separate lines, with the
open port and its Check action above them.

| Baseline | Candidate, light | Candidate, dark |
| --- | --- | --- |
| [Port 9001 listed as closed](port-observation/before.jpg) | [Port 9001 unconfirmed](port-observation/light.jpg) | [Port 9001 unconfirmed](port-observation/dark.jpg) |

Opened a synthetic legacy schema-1 session and a session with an
unconfirmed port. The app opened both and refused to compare their different
probe profiles. Comparing two current-profile fixtures showed TCP 8080
changing from `open` to `unconfirmed`. Exporting selected evidence retained
that outcome and used the redacted address alias.

Declining a live loopback scan preserved the current inventory and produced
zero HTTP requests. The approved scan found the fixture's Web port. Its
Browser button opened the rendered fixture in Safari, and its prefilled
Check Service returned TCP connected and HTTP 200.

## Focused checks

```bash
bash build/build.sh
codesign --verify --deep --strict dist/LanCartographer.app
python3 build/macos-smoke.py dist
gofmt -l .
CGO_ENABLED=1 CC=clang go vet ./...
CGO_ENABLED=1 CC=clang go test ./internal/{probe,cli,servicecheck,gui,session} \
  -run 'Test(ScanHost_(OpenWebPort|ClosedPort)|ScanPort_ExpiredDeadline|DialFailureState|ExpectationsReport|ExpectationsRequireAuthorizedCompleteScan|CheckConnectOutcomes|PanelListsOpenPortsFirstAndSummarizesTheRest|SessionUnconfirmedPortAndLegacyProfile|SessionReadRejectsUntrustedFiles)' -count=1
STATICCHECK_CACHE="$scratch/static-cache" CGO_ENABLED=1 CC=clang \
  "$scratch/staticcheck" ./internal/{probe,cli,model,servicecheck,gui,session}
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c ./internal/probe -o "$scratch/probe.test.exe"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c ./internal/servicecheck -o "$scratch/servicecheck.test.exe"
```

All passed, including staticcheck 0.8.2. The packaged smoke exercised both CLI
entry points and confirmed the deployment target and included license.
Windows cross-compilation covered the Winsock code path. Native Windows and
Intel Mac interaction remain outside this run.
