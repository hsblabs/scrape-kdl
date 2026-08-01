---
status: accepted
date: 2026-08-01T03:23:37+09:00
decision-makers:
  - project owner
agent: OpenAI Codex (GPT-5)
---

# ADR 0006: Derive offline snapshot eligibility from the whole program

## Context

Authoring tools need to validate saved HTML against the exact browser-mode
program being edited. Rewriting `fetch mode="browser"` to `mode="http"` changes
the program under test. Broadening the existing HTTP-only `ExtractHTML` method
would be misleading because static HTML cannot reproduce browser workflows or
JavaScript-derived output.

Snapshot execution must also stay outside acquisition. A fixture run must not
navigate, fetch, apply a URL policy, use ambient session state, or acquire a
mutable browser page merely because the compiled source declares browser mode.

## Decision

Go exposes `Program.ExtractSnapshot`; TypeScript exposes
`program.extractSnapshot`. Both accept already-decoded HTML and run the existing
portable DOM output engine without resolving inputs or invoking acquisition
dependencies.

Eligibility belongs to the complete immutable program. The prepared runtime
plan derives and caches the first deterministic incompatibility. Any browser
workflow rejects at `source.workflow`, and any JavaScript field value source,
including a nested one, rejects at that output member ID. Both use the stable
runtime code `E_SNAPSHOT_UNSUPPORTED`.

The snapshot runtime never removes or interprets unsupported nodes. Eligible
programs use the same selector, value, transform, external-transform, recovery,
warning, partial-result, and cancellation machinery as ordinary portable DOM
execution. HTTP- and browser-mode programs share this path. Existing Go
`ExtractHTML` remains HTTP-only.

## Consequences

- Authoring tools execute the exact compiled source mode against saved HTML.
- Static fixtures cannot accidentally conceal workflow or JavaScript
  dependencies.
- Snapshot calls have no network, navigation, browser, URL-policy, or session
  side effects.
- Eligibility is intentionally conservative: even a workflow that appears not
  to affect the final DOM makes the whole program ineligible.
- Snapshot eligibility is runtime-prepared metadata, not a new language
  capability or Validated IR field, so the dated wire contract does not change.
