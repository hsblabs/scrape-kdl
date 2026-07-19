# Changelog

All notable implementation changes are recorded here. Formal releases will use Semantic Versioning.

## Unreleased — M5 release hardening

- Added `PublicInternetURLPolicy` and `NewPublicInternetHTTPClient` (dial-time re-check against DNS rebinding) to the Go API, and made the CLI reject loopback, private, link-local, multicast, and unspecified targets by default; pass `--allow-private-hosts` to restore local-network extraction. Library `Options` defaults are unchanged.
- Decoded WHATWG-labelled charsets (Shift_JIS, EUC-JP, ...) in the Go HTTP runtime by default via `golang.org/x/text/encoding/htmlindex`, aligning with the TypeScript runtime's `TextDecoder` fallback; `Options.CharsetDecoder` still overrides, and `E_HTML_CHARSET_UNSUPPORTED` now indicates labels outside the WHATWG index.
- Prepared immutable execution plans once per compiled program, centralized output recovery/cardinality semantics across HTTP and browser modes, bounded first/one selector queries and collection overflow work, and de-duplicated diamond import traversal.
- Replaced TypeScript native regular-expression execution with the pinned RE2-compatible `re2js` runtime and added a nested-repetition regression test.
- Added machine-checked built-in, diagnostic, TypeScript export, and exact IR declaration contract gates; narrowed the exported IR types to the approved public surface.
- Added optional `BrowserAdapterQueryLimit` support with backward-compatible fallback, plus Playwright and go-rod implementations.
- Aligned runtime cancellation on `E_EXECUTION_CANCELED`, added a shared cross-runtime fixture, and propagated CLI diagnostic/JSON writer failures.
- Renamed TypeScript "slice" implementation and automation identifiers to the completed compiler/runtime terminology; use `conformance:typescript` and `test:typescript-contract` in workspace automation.
- Replaced the untagged integer/`0.1` working-draft identifiers with independent `2026-07-15` document, language, and Validated IR date strings; `Program.Version` now returns a string and the Go API exposes exact supported-version registries.
- Added the dated Validated IR schema layout, canonical JSON normalization, schema/golden validation, declaration-shape drift checks, and acquisition-time file/capability metadata preflight.
- Defined equivalent Go and TypeScript v1 API candidates, made Go compilation context-first with injected source loading, removed internal executor aliases from public browser signatures, and added independent consumer contract checks. This is a pre-v0.9 Go API break: `CompileFile` and `ValidateFile` now require `context.Context`.
- Completed the `2026-07-15` contract audit with a drift-checked normative coverage inventory, corrected the dated grammar summary, and added a bounded independent Node.js 26 TypeScript parser/compiler slice that matches the Go IR and shared diagnostic fixtures.
- Added the dated cross-language conformance manifest and result schemas, focused suite selection, a Go runner, the manifest-driven TypeScript slice runner, and CI failure gates for missing fixtures/artifacts and unapproved differences.
- Added the ESM-only `@hsblabs/scrape-kdl` npm package scaffold and reserved Playwright adapter workspace, approved root and Node.js entry points, full IR declarations, Node.js 26 Linux/macOS gates, coverage and source lint checks, and packed clean-consumer verification without a core Playwright dependency.
- Completed the TypeScript KDL subset parser and injectable source graph, added explicit Node.js filesystem loading, and introduced shared Go/TypeScript parser, invalid-UTF-8, import, cycle, and bounded-mutation fixtures with exact diagnostic parity.
- Moved all module paths to `github.com/hsblabs/scrape-kdl`.
- Added KDL slashdash suppression for nodes, arguments, properties, and child blocks.
- Added hexadecimal, octal, and binary integer literals.
- Added the optional `BrowserAdapterLease` contract.
- Added extraction-wide serialization to the go-rod adapter.
- Added browser failure-path, lease, and JavaScript result tests.
- Added a release-clean go-rod module and repository workspace.
- Added CLI build-version metadata.
- Added CI, scheduled browser E2E, core release, and adapter release workflows.
- Added Apache-2.0 licensing and project security/contribution policies.
- Added release, compatibility, parser-conformance, and security-model documentation.
- Added raw-text/RCDATA handling, truncated-document recovery, and common optional-end-tag normalization to the internal HTML parser.
- Added `Options.URLPolicy` for initial URL and HTTP redirect enforcement.
- Aligned runtime diagnostics with the normative diagnostic registry, including timeout and partial-extraction codes.
- Added KDL, CSS selector, and HTML parser fuzz tests with scheduled CI.
- Added release-tag validation and automatic diagnostic-documentation drift checks.
- Added `--session-file` JSON and standard-input support for secrets and deprecated direct `--header` and `--cookie` values.
- Changed explicit subcommand `-h` and `--help` requests to exit successfully.
- Defined browser JavaScript result representations and added `NormalizeBrowserResult` for adapter authors.
- Added immediate external-transform result validation with `E_EXTERNAL_TRANSFORM_RESULT_TYPE`.
- Defined `session policy="none"` as suppressing only explicit session input while preserving documented host-owned ambient state.
- Bounded millisecond workflow durations to the portable `time.Duration` maximum.
- Added coarse-grained offline HTML cancellation with `E_EXECUTION_CANCELED`.

## M4 — go-rod adapter

- Added an independent go-rod `BrowserAdapter` module.
- Added browser session headers, cookies, User-Agent handling, workflow actions, document/current JavaScript evaluation, scoped queries, and element reads.
- Added a go-rod example CLI and a build-tagged Chromium E2E test.

## M3 — browser runtime

- Added the dependency-neutral browser adapter contract.
- Added browser navigation, workflow execution, live-DOM extraction, and JavaScript opt-in.

## M2 — HTTP interpreter

- Added HTTP and offline HTML extraction.
- Added the public Go API and `scrape-kdl extract`.
- Added runtime inputs, URL-template expansion, sessions, timeout, and response-size limits.
- Added charset decoding and a custom charset callback.
- Added the internal DOM and portable CSS selector engine.
- Implemented built-in, declared pipeline, match, and external transforms.
- Added missing-value semantics, field recovery, row skipping, warnings, partial state, and runtime type checks.

## M1 — parser, validator, and Validated IR

- Implemented the Go reference parser, semantic validator, and Validated IR emitter.
- Added module imports, type checking, diagnostics, fixtures, and golden IR tests.
