#!/usr/bin/env bash
# bootstrap.sh — lan-optimize harness.
#
# Deterministic setup + scan seed for the lan-optimize skill. It does the
# scriptable parts so the agent spends its budget reading code and writing
# PRs:
#   1. Reports the git identity commits will carry.
#   2. Fetches origin/main and, when given a branch name, cuts that branch
#      from origin/main (an existing branch is checked out, never reset).
#   3. Prints a repo inventory that seeds the time-boxed scan.
#
# It does not call the GitHub API (PR dedup is an MCP step), edit code, or
# commit.
#
# Pipelines that end in head can die of SIGPIPE under pipefail; those end
# in `|| true` so set -e does not silently cut the inventory short.
#
# Usage: bash .claude/skills/lan-optimize/bootstrap.sh [work-branch-name]
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

WORK_BRANCH="${1:-}"
hr() { printf '%s\n' "------------------------------------------------------------"; }

hr
# git var honours GIT_AUTHOR_*/GIT_COMMITTER_* overrides, unlike git config.
author=$(git var GIT_AUTHOR_IDENT 2>/dev/null | sed -E 's/ [0-9]+ [-+][0-9]{4}$//') || true
committer=$(git var GIT_COMMITTER_IDENT 2>/dev/null | sed -E 's/ [0-9]+ [-+][0-9]{4}$//') || true
if [ -z "$author" ] || [ -z "$committer" ]; then
    echo "git identity: incomplete (author: ${author:-none}, committer: ${committer:-none})"
elif [ "$author" = "$committer" ]; then
    echo "git identity: $committer"
else
    echo "git identity: author $author, committer $committer"
fi

echo "fetching origin/main ..."
git fetch origin main --quiet || echo "WARN: fetch failed (offline?), using local refs"

if [ -n "$WORK_BRANCH" ]; then
    if git show-ref --verify --quiet "refs/heads/${WORK_BRANCH}"; then
        echo "branch '${WORK_BRANCH}' exists, checking it out (no reset)"
        git checkout "$WORK_BRANCH"
    else
        echo "creating '${WORK_BRANCH}' from origin/main"
        git checkout -b "$WORK_BRANCH" origin/main
    fi
fi
hr
echo "current branch: $(git rev-parse --abbrev-ref HEAD)"
echo "commits ahead of origin/main: $(git rev-list --count origin/main..HEAD 2>/dev/null || echo '?')"
echo "commits behind origin/main:   $(git rev-list --count HEAD..origin/main 2>/dev/null || echo '?')"
echo "working tree: $(git status --porcelain | wc -l | tr -d ' ') changed paths"

hr
echo "REPO INVENTORY (scan seed)"
hr
echo "## Go toolchain"
echo "  go.mod: $(grep -m1 '^go ' go.mod)   host: $(go version 2>/dev/null || echo 'go not on PATH')"

echo
echo "## Packages (non-test LOC / test LOC)"
for d in cmd/lancartographer internal/* tools/icongen; do
    [ -d "$d" ] || continue
    src=$(find "$d" -maxdepth 1 -name '*.go' ! -name '*_test.go' ! -name 'oui_data.go' -exec cat {} + 2>/dev/null | wc -l | tr -d ' ')
    tst=$(find "$d" -maxdepth 1 -name '*_test.go' -exec cat {} + 2>/dev/null | wc -l | tr -d ' ')
    printf '  %-26s %5s / %5s\n' "$d" "$src" "$tst"
done

echo
echo "## Platform files (Windows / macOS / other) — parity check seed"
for d in internal/winutil internal/gui; do
    w=$(ls "$d" | grep -c '_windows' || true)
    m=$(ls "$d" | grep -c '_darwin' || true)
    o=$(ls "$d" | grep -c '_other' || true)
    printf '  %-20s windows %s  darwin %s  other %s\n' "$d" "$w" "$m" "$o"
done

echo
echo "## Workflows (uses: lines not SHA-pinned are findings)"
for wf in .github/workflows/*.yml; do
    total=$(grep -cE '^\s*(- )?uses:' "$wf" || true)
    unpinned=$(grep -E '^\s*(- )?uses:' "$wf" | grep -cvE '@[0-9a-f]{40}' || true)
    printf '  %-34s %s uses, %s unpinned, triggers: %s\n' "$wf" "$total" "$unpinned" \
        "$(awk '/^on:/{p=1;next} p&&/^[^ ]/{exit} p&&/^  [a-z_]+:/{gsub(/[ :]/,"");printf "%s ",$0}' "$wf")"
done

echo
echo "## todo.txt (owner's deletion list; never re-add these as features)"
sed -n '1,20p' todo.txt 2>/dev/null | sed 's/^/  /' || true

echo
echo "## Recent commits on main (avoid redoing landed work)"
git log origin/main --oneline -n 15 2>/dev/null | sed 's/^/  /' || true

echo
echo "## Hot files (most-changed in the last 30 commits on main)"
git log origin/main -n 30 --name-only --format= 2>/dev/null | grep -v '^$' | sort | uniq -c | sort -rn | sed 's/^/  /' | head -10 || true

hr
echo "Next: dispatch the read-only scan subagent (SKILL.md Step 1), then dedup"
echo "against open PRs with mcp__github__list_pull_requests (Step 2)."
