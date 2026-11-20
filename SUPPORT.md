# Support

Use the repository's [bug report form](.github/ISSUE_TEMPLATE/bug_report.md)
for reproducible defects and the [feature request form](.github/ISSUE_TEMPLATE/feature_request.md)
for proposed improvements. Include the LanCartographer version, operating
system and architecture, entry point, reproduction steps, and relevant output.

Check the existing [open issues](https://github.com/cgfixit/LanCartographer/issues)
first. [Issue #22](https://github.com/cgfixit/LanCartographer/issues/22) owns the
roadmap and the remaining native platform acceptance. A pending check is not a confirmed defect;
add a concrete reproduction when behavior fails.

Redact IP and MAC addresses, host and share names, usernames, credentials,
and any other private inventory data before posting. Do not post information
from networks you do not own or have explicit authorization to inventory.

For session/comparison defects, include the completion states and the displayed
compatibility reason. Prefer a small synthetic reproduction over attaching full
session JSON: it contains unredacted inventory and local source addresses. A
selected evidence report defaults to redaction, but notes and screenshots need
manual review. See the [session troubleshooting table](docs/USER-GUIDE.md#when-comparison-or-export-is-unavailable).

Report suspected vulnerabilities privately through GitHub Security Advisories,
following [SECURITY.md](SECURITY.md). Do not disclose them in a public issue.
