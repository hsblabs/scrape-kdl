---
status: accepted
date: 2026-08-09T18:50:00+09:00
decision-makers:
  - project owner
agent: OpenAI Codex (gpt-5.6-luna, max; orchestrated by GPT-5)
---

# ADR 0009: Stable v1.0.0 owner gate override

## Context

The public `v1.0.0-rc.3` candidate was independently qualified at
`2026-08-01T08:01:28Z`. The normal release process therefore measures a
14-day blocker-free candidate period before stable publication. On 2026-08-09,
the project owner explicitly directed the stable `v1.0.0` publication run and
overrode that time gate.

## Decision

For this stable release only, the 14-day candidate gate is approved as an
owner-direction exception. The reason is the owner's explicit direction only;
this decision does not claim that the system is technically safer or that the
elapsed qualification period is unnecessary. At the time this decision is
recorded, the release is prepared but not published, and the existing RC
versions remain immutable.

## Required verification

- Pre-publication: run the release plan and complete gate from the exact
  reviewed commit, verify version and tag availability plus release controls,
  inspect stable archives and checksums, and execute the protected workflow.
- Post-publication: verify Go modules, npm packages, GitHub Releases, Pages,
  clean consumers, provenance and signatures, checksums, native archives, and
  the first-24-hour monitoring record on issue #18.

## Outcome

The reviewed preparation landed in PR #72 at
`4204e3660eafe9a6129312d98b4c041e70124f9f`. Protected workflow
[run 31308137799](https://github.com/hsblabs/scrape-kdl/actions/runs/31308137799)
published stable `v1.0.0` on 2026-08-09. Immediate independent checks passed
for the immutable tags, Releases, both Go modules, both npm packages, clean
consumers, signatures and provenance, checksums, supported CLI archives, and
the dated specification endpoints. Issue #18 records the continuing
first-24-hour observation.

## Rollback and recovery

Do not move, replace, or reuse the existing `rc.1`, `rc.2`, or `rc.3` tags,
Releases, or package versions. If a stable artifact has a problem, publish
`v1.0.1` from a reviewed fix or revert.
