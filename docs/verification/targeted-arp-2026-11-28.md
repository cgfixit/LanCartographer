# Targeted macOS ARP lookup verification

This change addresses finding 7 in [issue #89](https://github.com/cgfixit/LanCartographer/issues/89). The candidate replaces the complete formatted cache listing with `/usr/sbin/arp -n <IPv4>`. It preserves the two-second deadline, read-only cache lookup, and ordinary cache-miss result. Apple's implementation still reads the underlying table. The measured improvement is in the helper invocation, including numeric output without reverse DNS and reduced output. [Apple ARP source](https://github.com/apple-oss-distributions/network_cmds/blob/main/arp.tproj/arp.c#L568-L621)

## Populated-cache benchmark

On an arm64 M5 Pro host running macOS 27.2.1 (26B131), an owned Lima Alpine VM supplied one primary address and 16 aliases. Each mode used 35 interleaved observations per variant, split into seven rounds of five. The warmed hit was checked against the expected MAC. An unused address supplied the miss.

| Cache case | Main `arp -a` median | Numeric full table `arp -an` median | Targeted `arp -n <IPv4>` median |
| --- | ---: | ---: | ---: |
| Hit | 72.041 ms | 14.417 ms | 6.738 ms |
| Miss | 57.863 ms | 12.731 ms | 6.101 ms |

Round medians ranged from 51.198–254.862 ms for main hits and 5.643–22.387 ms for targeted hits. Main misses ranged from 42.582–102.012 ms and targeted misses from 4.913–8.245 ms. Median captured stdout was 2,620 bytes for the main listing and 69 bytes for targeted hits or 43 bytes for targeted misses, excluding trailing newlines. Both variants matched the expected MAC for all 17 owned addresses. All measured hits and misses returned the expected outcome within two seconds. These are helper measurements.

## Packaged CLI and native desktop

- In an owned macOS 26.6.2 VM (25G83), seven paired scans from signed packages found the expected virtual-bridge MAC and HTTP title in both variants. The comparison used base `5e1868c` and the same ARP implementation as this draft, before the upstream #98 merge.
  - Main CLI SHA-256: `e874840ef78f6dbe63c0889d6c2219547972387fd2c790624cef46e074a117b5`
  - Targeted CLI SHA-256: `26740f45aaabbbcca9e72c4e1fc3396439832ef21da73ada6cd58628dbffd084`
- The owner explicitly waived the macOS 26 native desktop check for this draft. MAC display in the macOS 26 desktop app remains unverified.
- On macOS 27.2.1, earlier owned-peer desktop scans found the HTTP endpoint in both variants while their MAC fields stayed blank. The interactive system shell repeatedly saw the cached MAC. The same lookup failed from the locally built process, consistent with the observed macOS 27 routing-socket visibility restriction. [Issue investigation](https://github.com/cgfixit/LanCartographer/issues/89#issuecomment-7233870022) · [XNU routing-socket source](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/net/rtsock.c#L1792-L1806)
- After integrating base `94799ff272387463471bcab61f25419b061c4a2c`, Computer Use exercised the newly packaged app with disposable application data and a loopback HTTP fixture. A scan of `127.0.0.1/32` restricted to TCP 8080 displayed one host and one open port. Selecting the host showed its TCP discovery and web service. The Browser action opened the owned fixture at `127.0.0.1:8080` in Safari. Loopback exercised the cache-miss path.
  - Native executable SHA-256: `dad1b51407e21e5cf18c8dc9d2893708d421ef89ef80ce4412524331dafcf65c`
  - Packaged CLI SHA-256: `f5692d17c02ad98d9a1d4ebb6be211b2ccdbf3605413bb7b6b754aba5aefe7a1`

## Integrated build checks

The macOS 27 build used Go 1.27.4 and Apple clang 21.0.0. Formatting, vet, focused ARP parser and runtime miss checks, discovery cancellation and ARP-only checks, targeted staticcheck, package signing, and packaged CLI smoke checks passed.

```sh
gofmt -l .
CGO_ENABLED=1 CC=clang go vet ./...
go test ./internal/winutil -run '^(TestParseARPEntry|TestSendARPLoopbackCacheMiss)$' -count=1
go test ./internal/discover -run '^(TestAlive_ARPOnly|TestAlive_CancelSkipsARP)$' -count=1
staticcheck ./internal/winutil
bash build/build.sh
codesign --verify --deep --strict dist/LanCartographer.app
python3 build/macos-smoke.py dist
```
