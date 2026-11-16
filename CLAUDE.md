@AGENTS.md

# CLAUDE.md

`AGENTS.md`, imported above and loaded every session, is the full contract (along with INVARIANTS.md).
Follow it literally. The notes below are only Claude Code-specific deltas.

- Skills live in `.agents/skills/`, not `.claude/skills/`. Read the SKILL.md
  directly when AGENTS.md §6 routes you there. `/lan-optimizer` means
  `.agents/skills/lan-optimizer/SKILL.md`.
- Use the `claude/<slug>` branch prefix unless the session requires another.
- Prefer running the changed behavior plus gofmt/vet over broad test runs
  (AGENTS.md §4).
- Never post PR comments that request Codex or other bot reviews without the
  owner's approval.
- Before treating red CI as a code failure, read the check-run annotations
  (AGENTS.md §7, "Live status").
