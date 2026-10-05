# .agents/ — shared agent resources

This folder is a **shared, agent-agnostic resource** for any AI coding agent
working in this repository — Kimi Code, Codex, Claude Code, Grok Build,
Copilot, or any future tool. It is not tied to one product's config format.

## What lives here

- `.agents/skills/` — reusable skill definitions (`SKILL.md` files) that an
  agent can load on demand to follow a repeatable workflow. Today this holds
  the [`optimize`](skills/optimize/SKILL.md) skill, a read-only audit and
  improvement-finding pass.

## Rules

- **Skills here are read-only contracts.** Do not edit them in the middle of
  a task to make a result fit. Change a skill only through a normal pull
  request so the change is reviewed like any other code.
- **The root `AGENTS.md` is the entry point.** Read it first for build/test
  commands, repo facts, and operating rules; treat this folder as the
  library it points into.
- Nothing in this folder is imported or executed by the application build.
  It is documentation and procedure only.
