#!/usr/bin/env bash
# check.sh — deterministic LanCartographer invariant guard.
#
# Read-only. Checks the AGENTS.md invariants a reviewer would otherwise have
# to eyeball: the CFAA sentence, the asInvoker manifest, the checked-in
# generated files, the engine's safety limits, and workflow hardening.
# Exits 1 if any check fails. Needs bash, git, grep, tr and awk only, so it
# runs in Git Bash on Windows, on macOS and in a Linux cloud session.
#
# Usage: bash .claude/skills/lan-guard/check.sh
set -uo pipefail
cd "$(git rev-parse --show-toplevel)"

FAILED=0
pass() { echo "PASS  $1"; }
fail() { echo "FAIL  $1"; FAILED=1; }

# Normalizes whitespace so a sentence wrapped across lines still matches.
flat() { tr -s '[:space:]' ' ' < "$1"; }

CFAA='Scan only networks you own or have explicit authorization to inventory. Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws.'
for f in LICENSE README.md; do
    if flat "$f" | grep -qF "$CFAA"; then pass "CFAA sentence verbatim in $f"; else fail "CFAA sentence missing or altered in $f"; fi
done
# authz.Notice is a Go string concatenation; check each half.
if grep -qF '"Scan only networks you own or have explicit authorization to inventory. "' internal/authz/authz.go &&
   grep -qF '"Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws."' internal/authz/authz.go; then
    pass "authz.Notice carries the CFAA sentence"
else
    fail "authz.Notice no longer carries the CFAA sentence verbatim"
fi

MANIFEST=cmd/lancartographer/app.manifest
if grep -q 'level="asInvoker"' "$MANIFEST" && ! grep -rq 'requireAdministrator' "$MANIFEST" versioninfo.json; then
    pass "manifest is asInvoker"
else
    fail "manifest is not asInvoker or requests administrator"
fi

for f in cmd/lancartographer/resource.syso internal/oui/oui_data.go go.sum; do
    if ! git ls-files --error-unmatch "$f" >/dev/null 2>&1; then
        fail "$f is not tracked"
    elif git check-ignore -q --no-index "$f"; then
        fail "$f is matched by .gitignore"
    else
        pass "$f tracked and not ignored"
    fi
done

# Engine limits (AGENTS.md §4). A change here needs a measurement, not a guess.
want() { # file, fixed string, label
    if grep -qF "$2" "$1"; then pass "$3"; else fail "$3 ($1 no longer has: $2)"; fi
}
want internal/probe/http.go 'httpProbeTimeout = time.Second' "HTTP probe timeout is one second"
want internal/probe/http.go 'httpBodyCap = 32 << 10' "HTTP body cap is 32 KiB"
want internal/probe/limiter.go 'MaxConcurrency = 256' "dial budget hard cap is 256"
want internal/scan/engine.go 'const maxHostsInFlight = 64' "host worker limit is 64"
if grep -q 'CheckRedirect' internal/probe/http.go; then pass "HTTP probe sets a redirect policy"; else fail "HTTP probe lost its CheckRedirect policy"; fi
if grep -rn 'InsecureSkipVerify: *true' --include='*.go' internal cmd | grep -v '_test.go' | grep -q .; then
    fail "TLS verification disabled outside tests"
else
    pass "no InsecureSkipVerify outside tests"
fi
if grep -rnE 'Listen[A-Za-z]*\([^)]*"(0\.0\.0\.0)?:' --include='*.go' internal cmd | grep -v '_test.go' | grep -q .; then
    fail "a non-test listener binds all interfaces"
else
    pass "no all-interfaces listener outside tests"
fi

# Workflow hardening.
# Top-level permissions: either deny-all (`permissions: {}`, jobs opt in) or a
# block/flow map that grants contents: read and nothing writable. Absent,
# read-all, write-all and any top-level `: write` fail.
toplevel_perms() { # prints "ok", or the reason it is not
    local wf=$1 line value body
    line=$(grep -m1 -E '^permissions:' "$wf") || { echo "missing top-level permissions"; return; }
    value=$(printf '%s' "$line" | sed -E 's/^permissions:[[:space:]]*//; s/[[:space:]]+#.*$//; s/[[:space:]]+$//')
    case "$value" in
        '{}') echo ok; return ;;
        '') body=$(awk '/^permissions:/{p=1;next} p&&/^[^ #]/{exit} p' "$wf" | sed -E 's/#.*$//') ;;
        '{'*'}') body=$(printf '%s' "$value" | tr ',{}' '\n  ') ;;
        *) echo "top-level permissions: $value"; return ;;
    esac
    if printf '%s\n' "$body" | grep -qE ':[[:space:]]*write'; then
        echo "top-level permissions grant write"
    elif printf '%s\n' "$body" | grep -qE '^[[:space:]]*contents:[[:space:]]*read[[:space:]]*$'; then
        echo ok
    else
        echo "top-level permissions lack contents: read"
    fi
}
# contents: write is allowed only in these job-level blocks, each of which
# publishes a GitHub release. Everything else, including any new workflow,
# fails. Matches print as "<workflow>:<job>".
WRITE_ALLOWED=' release.yml:release release-main-macos.yml:publish '
write_grants() { # file -> "<base>:<job>" per contents: write / write-all line under jobs
    awk -v base="${1##*/}" '
        /^[^ #]/ { injobs = ($0 ~ /^jobs:/); job = ""; next }
        injobs && /^  [A-Za-z0-9_-]+:/ { job = $1; sub(/:$/, "", job); next }
        injobs && job != "" && /^[^#]*(contents:[[:space:]]*write|permissions:[[:space:]]*write-all)/ { print base ":" job }
    ' "$1"
}
WRITE_VIOLATIONS=""
for wf in .github/workflows/*.yml; do
    why=$(toplevel_perms "$wf")
    if [ "$why" = ok ]; then
        pass "$wf: top-level permissions are contents: read or deny-all"
    else
        fail "$wf: $why (need contents: read or {})"
    fi
    for grant in $(write_grants "$wf"); do
        case "$WRITE_ALLOWED" in *" $grant "*) ;; *) WRITE_VIOLATIONS="$WRITE_VIOLATIONS $grant" ;; esac
    done
    if grep -q 'pull_request_target' "$wf"; then fail "$wf: uses pull_request_target"; fi
    unpinned=$(grep -nE '^\s*(- )?uses:\s*[^./][^@]*@' "$wf" | grep -vE '@[0-9a-f]{40}( |$)' || true)
    if [ -n "$unpinned" ]; then
        fail "$wf: actions not pinned to a commit SHA:"; echo "$unpinned" | sed 's/^/        /'
    else
        pass "$wf: actions pinned to commit SHAs"
    fi
    secrets=$(grep -noE 'secrets\.[A-Za-z_]+' "$wf" | grep -v 'secrets.GITHUB_TOKEN' || true)
    if [ -n "$secrets" ]; then fail "$wf: uses a secret other than GITHUB_TOKEN: $secrets"; fi
    if grep -q '@latest' "$wf"; then fail "$wf: installs a tool at @latest"; fi
done
if [ -n "$WRITE_VIOLATIONS" ]; then
    fail "contents: write outside the release publish jobs:$WRITE_VIOLATIONS"
else
    pass "contents: write only in release.yml:release and release-main-macos.yml:publish"
fi

if command -v actionlint >/dev/null 2>&1; then
    if actionlint .github/workflows/*.yml; then pass "actionlint"; else fail "actionlint"; fi
else
    echo "SKIP  actionlint not installed (go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12)"
fi

echo
if [ "$FAILED" -eq 0 ]; then echo "LAN-GUARD: PASS"; else echo "LAN-GUARD: FAIL"; fi
exit "$FAILED"
