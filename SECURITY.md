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

- The two distributed executables (`LanCartographer.exe`,
  `lancartographer-cli.exe`) and their scan behavior.
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
- **argv-only subprocess launches** (argument arrays, never `shell=True`-style
  command strings), and
- **zero telemetry** — the tool sends nothing off the local network being
  inventoried.

Changes that weaken the authorization gate or fail-closed behavior are
security regressions and will be treated as such.

CSV exports prefix formula-like cells with an apostrophe, including leading
whitespace and full-width formula prefixes; CSV delimiters and quotes are
escaped by Go's CSV writer. JSON exports preserve the original values.
Spreadsheet import behavior varies, and saving and reopening a CSV in Excel
can remove text escapes. Keep untrusted inventories as text when editing or
re-exporting them; this hardening does not guarantee safety across spreadsheet
save/reopen cycles. See [OWASP's CSV injection guidance](https://owasp.org/www-community/attacks/CSV_Injection).
