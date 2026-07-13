@AGENTS.md

# Claude Code instructions

Use `AGENTS.md` as the shared repository contract. Keep this file limited to Claude Code-specific workflow guidance.

## Workflow

- Read the relevant normative spec, implementation, and fixtures before proposing changes.
- For multi-file or language-level changes, form a brief implementation plan before editing.
- Make the smallest coherent change, then run focused tests before broader verification.
- Use `make verify` as the default completion gate.
- Run `make test-rod` or `make test-rod-e2e` only when the environment can resolve the adapter dependency and provide Chromium.
- Do not rewrite golden files until the semantic change has been reviewed.
- Summarize changed behavior, tests run, and any unverified external dependency or browser check.

## Context management

Keep permanent instructions concise. Put durable project rules in `AGENTS.md`, normative behavior in `docs/spec/`, and task-specific detail in code, tests, issues, or focused documentation rather than expanding this file.
