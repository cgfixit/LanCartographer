# Security Policy

## Supported Versions

Only the **latest release tag** receives security fixes. Older tags and
arbitrary commits on `main` between releases are not maintained — if a fix
ships, upgrade to the newest release.

| Version | Supported |
| ------- | --------- |
| Latest release tag | Yes |
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
