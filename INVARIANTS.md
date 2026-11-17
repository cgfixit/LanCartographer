# Security Invariants

- **TCP connect scans only.** Probes use `net.Dialer.DialContext` against
  specific ports; no SYN scans or crafted packets. Fallbacks are the documented
  Windows `SendARP` API and, on macOS, `/sbin/ping` for one ICMP echo, so the
  app itself stays unelevated.
- **Fail-closed authorization gate.** A target outside your local subnets,
  non-RFC 1918, or broader than a `/22` needs confirmation: a dialog in the
  GUI, `--i-am-authorized` in the CLI. Refusal means nothing is scanned. The
  engine also refuses IPv6 and anything broader than a `/12` outright.
- **Bounded probes.** The HTTP, dial-budget and worker limits in
  [How it works](README.md#how-it-works) are fixed invariants, and TLS verification is
  never disabled.
- **Single-endpoint service checks.** Check Service connects only after a
  confirmation that names the address, port and traffic, to that one pinned
  address, with every stage bounded by time and size.
- **No application credential store.** Share enumeration uses the current
  Windows token or macOS SMB session; Finder and Explorer handle
  authentication when you open shares.
- **Bounded external actions.** Arguments are passed separately. Windows uses
  `cmd /c start` for Browser and Ping; macOS uses `open` for URLs and a
  Terminal AppleScript for Ping. Ping targets come only from parsed IP
  literals, macOS share names are URL-escaped, and Wake-on-LAN sends its UDP
  packet only after confirmation.
- **No phone-home.** No telemetry, analytics or runtime downloads; the OUI
  database is embedded and current as of generation. Traffic beyond the scan
  comes only from configured resolvers (which may be off-LAN) and explicit
  actions: Ping, Wake-on-LAN, Check Service, desktop launches, and the
  confirmed Apple or Cloudflare measurements. Opening or comparing sessions
  sends nothing.
- **Unelevated by design.** The Windows manifest is `asInvoker` (never
  `requireAdministrator`) and its Elevate button is optional. macOS needs no
  root for the inventory workflow.
