# Changelog

All notable implementation changes are recorded here. Formal releases will use Semantic Versioning.

## Unreleased — M5 release hardening

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
