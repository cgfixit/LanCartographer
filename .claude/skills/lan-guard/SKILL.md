---
name: lan-guard
description: Deterministic invariant check for LanCartographer — CFAA sentence verbatim, asInvoker manifest, checked-in generated files, HTTP/dial/worker limits, no all-interfaces listener or disabled TLS verification, and workflow hardening (SHA-pinned actions, least-privilege permissions, no extra secrets, no @latest installs, actionlint). Use before opening or pushing any PR that touches .github/workflows, internal/authz, internal/probe, internal/scan, the manifest, LICENSE/README, or generated files, or when asked "is this safe to merge", "check invariants", or "lint the workflows".
---

# lan-guard

Runs the checks from `AGENTS.md` §1–§4 that a script can decide, so a
reviewer does not have to eyeball them:

```bash
bash .claude/skills/lan-guard/check.sh
```

It is read-only, needs only bash, git, grep, tr and awk, and exits 1 on any
`FAIL`. `actionlint` runs when it is on PATH; otherwise the line says `SKIP`
and you install it with
`go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` (binary in
`$(go env GOPATH)/bin`).

## Reading a failure

| FAIL line | What it means | Fix |
|---|---|---|
| CFAA sentence missing or altered | LICENSE, README, or `authz.Notice` no longer carries the exact sentence | Restore it word for word. Line wrapping is fine; wording, punctuation and `§` are not. |
| manifest is not asInvoker | `app.manifest` changed level or `requireAdministrator` appeared | Revert. Elevation stays optional and per-action. |
| not tracked / matched by .gitignore | `resource.syso`, `oui_data.go`, or `go.sum` was deleted or ignored | Restore from `origin/main`; drop the ignore rule. |
| HTTP / dial / worker limit | A constant in `probe` or `scan` moved | Revert unless the PR carries a measurement and says why. |
| redirect policy / InsecureSkipVerify | The HTTP probe can now follow redirects or skip TLS checks | Revert. |
| all-interfaces listener | Non-test code listens on `:port` or `0.0.0.0` | Bind loopback, or drop the listener. |
| not pinned to a commit SHA | A `uses:` line names a tag or branch | Pin `owner/action@<40-hex> # vX.Y.Z`. Resolve with `git ls-remote https://github.com/<owner>/<action> 'refs/tags/vX.Y.Z^{}'` (the peeled `^{}` line is the commit for annotated tags). Keep the `# vX.Y.Z` comment so Dependabot can bump it. |
| secret other than GITHUB_TOKEN | A workflow now needs a credential | Not allowed without the owner's sign-off (`AGENTS.md` §4). |
| installs a tool at @latest | `go install …@latest` in a workflow | Pin the version; `curl -s https://proxy.golang.org/<module>/@latest` shows the current one. |
| top-level permissions (missing, `read-all`, `write-all`, grant write, lack `contents: read`) | A workflow's top-level `permissions:` is absent, a blanket scope, or writable | Use `permissions: {}` (deny-all; each job opts in) or a map with `contents: read` and no `write`. Writes go on the job that needs them. |
| contents: write outside the release publish jobs | A job other than `release.yml:release` or `release-main-macos.yml:publish` gained `contents: write` or `write-all` | Drop it. Adding a job to the allowlist (`WRITE_ALLOWED` in `check.sh`) is its own PR with the reason. |

The write allowlist is job-scoped, not file-scoped. Both entries publish a
GitHub release, which needs `contents: write`: `release.yml:release` for `v*`
tags, and `release-main-macos.yml:publish` for the macOS prerelease after a
merge to `main`. The latter is only reachable through its `gate` job, which
keeps `contents: read` and checks the triggering run is a same-repo push. A
write grant in any other job, in `gate`, at the top level, or in a new
workflow fails.

## Limits

A PASS means these specific strings and patterns are intact. It is not a
test run, a Windows or macOS build, or evidence that a change works. Report
it as "lan-guard: PASS" alongside the commands that did exercise the change,
never instead of them.

Do not relax a check to make a PR pass. A check that is wrong changes in its
own PR, with the reason in the body.
