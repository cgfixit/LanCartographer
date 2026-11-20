# Security Policy

## Supported Versions

Until the first version tag is published, no release exists yet: during that
period only `main` is supported, on a best-effort basis. The policy below
applies from v1.0.0 onward.

From v1.0.0 onward, only the **latest release tag** receives security fixes.
Older tags and arbitrary commits on `main` between releases are not
maintained — if a fix ships, upgrade to the newest release.

| Version | Supported |
| ------- | --------- |
| Latest release tag (from v1.0.0) | Yes |
| `main` before the first tag | Best-effort only |
| Anything older | No |

## Reporting a Vulnerability

**Do not open a public GitHub issue for a vulnerability.** Public issues are
visible to everyone and would disclose the flaw before a fix exists.

Report privately through GitHub **private vulnerability reporting / Security
Advisories** for this repository:

1. Open the repository's **Security** tab.
2. Choose **Report a vulnerability** (creates a private draft advisory).
3. Include: affected version/commit, reproduction steps, impact, and (if you
   have one) a proposed fix or mitigation.

### Response timeline

- **Acknowledgement:** within **72 hours** of the report.
- **Triage** (severity assessment and a fix plan, or a reasoned rejection):
  within **7 days** of acknowledgement.

There is **no bug bounty program**. Reports are credited in the advisory and
release notes at your request.

## Scope

In scope:

- The Windows GUI/CLI executables and macOS app/CLI, including scans,
  imported session files and evidence exports.
- The source in this repository, including the CGO build and the
  `internal/winutil` memory-unsafe surface.

Out of scope: issues in third-party dependencies already tracked upstream,
and anything requiring physical access to the machine running the tool.

## Product Security Posture

LanCartographer is an **authorization-gated LAN inventory tool**:

> Scan only networks you own or have explicit authorization to inventory. Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws.

Security-relevant behavior is documented in the README, including:

- the **authorization gate** (`internal/authz`) that must confirm authorized
  use before any scan runs,
- **bounded desktop actions** with parsed IP/port targets and platform-specific
  argument builders (Windows Browser/Ping invoke `cmd`; macOS Ping uses Terminal
  AppleScript), and
- **zero telemetry** — no analytics, crash reports or runtime OUI downloads.
  This does not mean all traffic stays on the LAN: authorized targets, DNS/name
  resolution and explicit desktop/network actions determine destinations.

Changes that weaken the authorization gate or fail-closed behavior are
security regressions and will be treated as such.

CSV exports prefix formula-like cells with an apostrophe, including leading
whitespace and full-width formula prefixes; CSV delimiters and quotes are
escaped by Go's CSV writer. JSON exports preserve the original values.
Spreadsheet import behavior varies, and saving and reopening a CSV in Excel
can remove text escapes. Keep untrusted inventories as text when editing or
re-exporting them; this hardening does not guarantee safety across spreadsheet
save/reopen cycles. See [OWASP's CSV injection guidance](https://owasp.org/www-community/attacks/CSV_Injection).

## Saved sessions and evidence

Session JSON contains full, unredacted inventory and local interface addresses.
Saving is explicit; the app does not upload or automatically share these files.
Reads reject unsupported schemas, inconsistent records and files over 8 MiB or
65,536 hosts. Writes use a temporary file and rename, with modes `0600` for
files and `0700` for new directories on macOS. These permissions are not
application-level encryption; access also depends on the destination filesystem
and its permissions.

Comparison requires compatible completed sessions. Missing observations are
not proof of an offline device, and session files are editable records rather
than authenticated attestations. Plain-text evidence exports contain selected
differences and coverage context, with no active HTML or remote assets.

Report redaction is on by default. It withholds identifiers and discovered
text but retains timing, counts and numeric probe settings; aliases are not
anonymization. It does not redact the GUI comparison list, full session JSON,
operator notes or screenshots. Inspect what you share, especially notes and
reports exported with redaction disabled. See the [User Guide](docs/USER-GUIDE.md#what-is-saved-and-what-is-shared).

## Check Service

**Tools → Check Service…** contacts a service only after the operator
confirms the exact address, port and protocol; the confirmation repeats the
authorization notice and declining sends nothing. Resolving a name uses the
system resolver (hosts file, configured DNS or, on macOS, multicast DNS for
`.local` names). The check then dials only the confirmed address: it never
re-resolves the name, uses no proxy, never disables TLS verification, never
follows redirects and sends at most one `GET /` without credentials. Connect
and TLS stages have three-second deadlines; HTTP keeps the one-second and
32 KiB body limits with a 32 KiB header cap. Server-provided text is reduced
to printable single-line text before display. Results stay in memory and are
excluded from saved sessions, exports and logs; **Copy results** writes them
to the clipboard only on request.

## Local macOS diagnostics

Connections reads visible process and socket metadata through the system
`lsof`. Route & DNS reads the routing table for a numeric destination and
configured DNS resolver scopes. These reads send no probes. The snapshots
stay in memory. Review process, endpoint and resolver details before
copying or sharing them.

## Internet Quality

On macOS, the optional Internet Quality test sends active measurement
traffic to Apple only after the operator confirms the provider and data
use. The app invokes the system utility with fixed arguments and a
deadline. Results stay in memory and are excluded from saved sessions,
inventory exports and application logs. The separate Cloudflare action
opens the provider's HTTPS page in the default browser after confirmation.
Cloudflare applies its own [measurement privacy policy](https://speed.cloudflare.com/).
