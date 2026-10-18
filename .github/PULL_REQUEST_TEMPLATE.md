<!--
Keep the branch prefix kimi/<slug> (or grok/ / claude/ / codex/ per your tool).
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
Paste the exact commands you ran and a summary of their output.
"Not run" is NOT acceptable — the local gate must be green before review.

Expected minimum:

    bash tests/ci-gate.sh                       # or the equivalent steps:
    gofmt -l .                                  # (empty)
    go vet ./...                                # pass
    go generate ./... && git status --porcelain # no drift
    CGO_ENABLED=1 CC=gcc bash deploy/build.sh          # builds
    CGO_ENABLED=1 CC=gcc go test ./... -race    # pass
-->

```
<paste commands and output summary here>
```

## Security impact

<!-- Answer each: "no" or "yes + explanation". -->

- Touches the authorization gate (`internal/authz`)? **no** — <!-- explain if yes; any change must remain fail-closed, justify it -->
- Touches engine routing / scan dispatch? **no** — <!-- explain if yes -->
- Touches process launches (subprocess invocation)? **no** — <!-- explain if yes; launches must stay argv-only, never shell strings -->
- Touches network probes or scan behavior? **no** — <!-- explain if yes; the CFAA authorization notice stays intact -->

## UI changes

- [ ] No UI changes
- [ ] UI changes — screenshots attached below

<!-- Before/after screenshots are REQUIRED for any visible change. -->

## Checklist

- [ ] Local gate green: `gofmt -l .` empty, `go vet ./...` clean, `go generate` drift check clean
- [ ] CGO build passes (`CGO_ENABLED=1 CC=gcc bash deploy/build.sh`)
- [ ] Race tests pass (`CGO_ENABLED=1 CC=gcc go test ./... -race`)
- [ ] Tests added/updated for behavior changes
- [ ] Docs updated (README / USER-GUIDE / SECURITY as applicable)
- [ ] CFAA authorization notice untouched — or change is intentional and called out in the Summary
- [ ] `cmd/lancartographer/resource.syso` and `internal/oui/oui_data.go` not accidentally modified
- [ ] No logs, caches, build outputs, or secrets committed
