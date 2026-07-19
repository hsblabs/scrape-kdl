---
status: accepted
date: 2026-07-19
authors: Takumi Hasebe (decision), Claude Fable 5 (analysis and drafting)
---

# ADR 0001: Implementation language scope

## Context

Scraping KDL is a spec-first language: the normative documents and
conformance fixtures are the product, and implementations prove them. The
repository maintains two full implementations — Go (reference: parser,
compiler, runtimes, CLI) and TypeScript (parser/compiler slice, HTTP and
browser runtimes, Playwright adapter) — kept aligned by conformance
manifests, builtin/diagnostic contract gates, html-differential tests, and
RE2-compatible regex pinning.

The dual implementation has real, recurring cost: every semantic change is
either written twice or accrues parity debt (observed concretely in the
2026-07-19 compatibility work, where charset decoding diverged between the
runtimes and a Go-side URL-policy feature immediately created a TypeScript
follow-up). The question was whether this scope is justified and where it
is allowed to grow.

## Decision

1. **Two implementation languages: Go and TypeScript. This set is closed.**
   Python, PHP, and Ruby will never be supported; this is a deliberate
   authorial decision, not a resourcing gap. Contributions adding them will
   be declined regardless of quality.
2. **Go remains the reference implementation** and the source of truth when
   implementations disagree with each other but not with the specification.
3. **TypeScript exists for ecosystem reach, not symmetry**: the npm runtime
   and the Playwright adapter serve the JS-native browser-automation
   audience, and `evaluate-js` semantics are anchored there. Full parity of
   the TypeScript *compiler* is a means, not an end.

## Consequences

- With the language set closed at N=2, "compiler ×1 + runtime ×N" is an
  optimization, not a survival requirement. The main lever is eliminating
  the TypeScript parser/compiler/diagnostics parity surface, which is the
  most expensive and least valuable half of the duplication (runtimes must
  stay dual; they are the point).
- The considered mechanism is compiling the Go compiler to WebAssembly and
  shipping it as a separate, lazily-loaded npm package so runtime-only
  consumers download none of it. Measured on this codebase (Go 1.26,
  `-ldflags "-s -w"`): compiler-only wasm is 4.8 MB raw / 1.3 MB gzipped
  (the compile path is stdlib-only; the 7.0 MB figure includes the
  executor and is not needed). This is ordinary build-tool weight by npm
  standards; size is not a blocker. TinyGo (~1–2 MB expected) and
  `wasm-opt -Oz` are available reductions, gated on differential checks of
  canonical-JSON and diagnostic-ordering determinism.
- Timing: if the compiler is consolidated, it must land **before the v1
  API freeze** (`docs/roadmap-v1.md`); swapping compiler implementations
  after freeze is externally disruptive. Until then the TypeScript
  compiler may lag the Go compiler; the binding contract is the Validated
  IR and the shared fixtures. The deferred work item is tracked in
  `docs/design-backlog.md`.
