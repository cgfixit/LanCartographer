<!--
Use the branch prefix required by the active agent/user instructions.
One concern per PR. Open as draft. Every section below is required.
-->

## Summary

<!-- What does this PR do? One short paragraph. -->

## Type of change

- [ ] feat — new capability
- [ ] fix — bug fix
- [ ] perf — performance improvement
- [ ] security — security fix or hardening
- [ ] docs — documentation only
- [ ] ci — build, CI, or tooling only

## Motivation / value

<!-- Why is this change worth making? What breaks or improves without it? -->

## Test evidence (REQUIRED)

<!--
Paste the exact commands and results for the affected behavior and relevant
formatting, vet, build, or workflow checks. Broad local suites are not required
when hosted CI provides that coverage. Examples:

    bash deploy/build.sh                        # native package build
    python3 tests/macos-smoke.py dist            # packaged macOS paths
    actionlint .github/workflows/*.yml           # changed workflows
-->

```
<paste commands and output summary here>
```

### Platform and CI limitations

<!-- State what was not run locally, why, and which hosted CI job covers it. -->

## Security impact

<!-- Answer each: "no" or "yes + explanation". -->

- Touches the authorization gate (`internal/authz`)? **no** — <!-- explain if yes; any change must remain fail-closed, justify it -->
- Touches engine routing / scan dispatch? **no** — <!-- explain if yes -->
- Touches process launches (subprocess invocation)? **no** — <!-- explain if yes; launches must stay argv-only, never shell strings -->
- Touches network probes or scan behavior? **no** — <!-- explain if yes; the CFAA authorization notice stays intact -->
- Changes TLS verification, redirect handling, HTTP one-second/32-KiB limits, dial budget, worker limit, cancellation, or monotonic progress? **no** — <!-- explain and verify if yes -->
- Adds telemetry or runtime downloads? **no** — <!-- explain if yes; both are prohibited -->

## UI changes

- [ ] No UI changes
- [ ] UI changes — screenshots attached below

<!-- Before/after screenshots are REQUIRED for any visible change. -->

## Checklist

- [ ] Changed behavior exercised and exact results recorded above
- [ ] Relevant formatting, vet, build, and workflow checks pass
- [ ] Platform gaps and checks left to hosted CI are recorded above
- [ ] Tests added/updated for behavior changes
- [ ] Docs updated (README / USER-GUIDE / SECURITY as applicable)
- [ ] CFAA authorization notice untouched — or change is intentional and called out in the Summary
- [ ] `cmd/lancartographer/resource.syso` and `internal/oui/oui_data.go` not accidentally modified
- [ ] No logs, caches, build outputs, or secrets committed
