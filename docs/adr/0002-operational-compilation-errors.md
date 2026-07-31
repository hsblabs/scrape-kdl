---
status: accepted
date: 2026-08-01T02:58:26+09:00
decision-makers:
  - project owner
agent: OpenAI Codex (GPT-5)
---

# ADR 0002: Separate operational compilation errors from diagnostics

## Context

The release-candidate interface returned only a program and diagnostics from Go
compilation. Context cancellation, filesystem failures, and injected source
loader failures were converted to `E_KDL_SYNTAX`. Hosts could not distinguish
an invalid KDL document from a failed read, and wrapping destroyed the error
identity required by `errors.Is` and `errors.As`. TypeScript preserved aborts but
still converted non-abort loader failures to the same syntax diagnostic.

## Decision

Go compilation and validation entry points return an operational `error` in
addition to deterministic document diagnostics. Compiler syntax, semantic, and
type findings remain in `Diagnostics`. Cancellation, deadlines, filesystem
failures, invalid entry-point arguments, and injected-loader failures return an
`error` whose wrapping preserves the original cause.

TypeScript compilation keeps its promise-based interface. Aborts reject with
the abort reason. Other injected-loader failures reject with `SourceLoadError`,
which records the imported path and importing path and retains the original
failure in `cause`. Filesystem entry points retain the native filesystem error
for a failure to read the root document.

No program is returned for either class of failure. Operational failures do not
create diagnostic codes or source spans because they are not properties of the
KDL document.

## Consequences

- Existing Go callers must accept and handle the added `error` result before
  inspecting diagnostics or using the program.
- Hosts can reliably use `errors.Is` and `errors.As` for cancellation and I/O.
- Go and TypeScript use idiomatic interface shapes while preserving the same
  observable distinction between document findings and operational failure.
- CLI exit statuses and stream placement remain unchanged; operational errors
  are rendered separately from structured document diagnostics.
- The change is a release-blocking correction to the frozen v1 candidate and is
  included in the migration guide and independent consumer checks.
