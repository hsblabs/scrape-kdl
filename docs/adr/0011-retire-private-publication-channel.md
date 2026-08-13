---
status: accepted
date: 2026-08-13T12:52:00+09:00
decision-makers:
  - project owner
agent: OpenAI Codex (GPT-5)
---

# ADR 0011: Retire the private publication channel

## Context

The repository and both npm packages are public, and stable public release
publication is now protected by the unified `release.yml` workflow. The private
npm publication workflow and the private core/go-rod GitHub Release workflows
serve a distribution mode that is no longer part of the product contract.

Keeping those workflows creates unused manual-dispatch surfaces, stale npm
trusted-publisher assumptions, and a second release path that can diverge from
the public release process.

## Decision

Delete the private npm, core, and go-rod publication workflows and the private
npm post-publication verifier. Keep the read-only release rehearsal workflow,
private-tag validation helpers, and historical documentation/tests only where
they support immutable records or non-publishing verification.

Do not delete or rewrite historical `-private.N` tags, Releases, changelog
entries, or the versioning rationale. They remain immutable historical records,
but new private package or private GitHub Release publication is unsupported.

## Consequences

- There is one supported external release channel: the protected public
  workflow.
- npm trusted publishers no longer need a private workflow record.
- The release rehearsal remains available for no-write validation and artifact
  inspection.
- A future private distribution channel would require a new approved decision
  and workflow design rather than reviving deleted jobs implicitly.
