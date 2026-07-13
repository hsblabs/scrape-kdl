# Nightly improvement report

Date: 2026-07-13 through 2026-07-14
Branch: `codex/nightly-quality`  
Baseline: `e5363b7`

## Completed work

- Isolated real and contract go-rod verification from tracked `adapters/rod/go.mod` and `go.sum` by using temporary `-modfile` copies. Added success and failure regression tests and verified that real, contract, E2E, release, and core checks can run concurrently without corrupting module metadata.
- Repaired the Chromium E2E fixture's malformed JavaScript raw string and extended assertions to cover document-scoped JavaScript and collection extraction.
- Added direct tests for deterministic diagnostic sorting, non-mutating sort behavior, text/JSON rendering, writer failures, type relationships, malformed type syntax, CLI argument parsing, runtime input parsing, and session parsing.
- Removed four unused private helpers/types and simplified two equivalent boolean expressions reported by `staticcheck`.
- Made release checksum generation portable across Linux `sha256sum` and macOS `shasum`, with tests for both tools and the no-tool failure path.
- Ensured temporary release staging directories are removed after successful and failed cross-builds, with regression tests for both paths.
- Updated `VALIDATION.md` from the previous offline-only record to the current Go 1.26.5, real go-rod, Chromium E2E, fuzz, static analysis, vulnerability, and release-smoke results.
- Recorded the security and compatibility decision required for cookie and sensitive-header CLI input in `docs/decision-needed.md` without changing the existing CLI contract.
- Extended real Chromium coverage to current-scoped JavaScript, JavaScript failure propagation, and integer return normalization.
- Added charset boundary tests for HTML meta declarations, the 4096-byte sniff limit, UTF-16 decoding, BOM precedence, malformed input, unsupported encodings, and fallback failures; corrected the runtime documentation to state that recognized BOMs override declared charsets.
- Prevented already-canceled HTTP requests from reaching a custom transport, preserving the existing timeout/cancellation diagnostic mapping and cancellation cause. Added matching browser acquire, workflow, and lease-release coverage.
- Added direct source-span, KDL AST helper, public `Program`, execution error, and cancellation contract tests.
- Rejected malformed, non-finite, and overflowing `json.Number` JavaScript results.
- Enforced the specified `evaluate-js returns=...` contract before transforms, including nullable and array values plus range-checked integer and float normalization for browser-provided JSON numbers.
- Exercised successful and failing `validate`, `compile`, and offline `extract` command workflows, including file output. CLI statement coverage increased from 35.6% to 88.8%.
- Recorded the compatibility decision required to change explicit subcommand help from exit status 2 to 0 without changing current behavior.
- Aligned browser-mode missing/default/warn/fail recovery coverage with the HTTP runtime and wrapped malformed recovery defaults in the same structured `E_IR_INVALID` error.
- Added browser collection row-skip, warning order, row index, partial-result, and minimum/maximum cardinality coverage.
- Covered all browser workflow lowering forms and invalid selector, state, numeric, timeout, and unknown-step diagnostics. Added success and failure coverage for all four input default types; compiler statement coverage increased from 63.1% to 70.0%.
- Recorded the public adapter decision required to define concrete Go representations for JSON-compatible JavaScript results, without broadening the runtime contract implicitly.
- Covered browser text, HTML, attribute, field-query, and collection-query success and failure paths, including deadline mapping, cancellation causes, and warning recovery.
- Normalized numeric `match` results and `coalesce` literals to their resolved output types, and rejected non-finite `float32` values in `to-string`. Added malformed-IR failure coverage.
- Added join, integer/float conversion, radix, overflow, finite-number, and configurable boolean parsing boundaries; executor statement coverage increased to 73.9%.
- Corrected attribute selector case-sensitivity flags from malformed-selector classification to the specification-required unsupported-selector diagnostic.
- Added deterministic URL-template tokenization, escaped-brace, scalar percent-encoding, invalid-target, optional/undeclared-input, and portable selector classification tests; compiler statement coverage increased to 72.0%.
- Fixed a fuzz-discovered raw-text parser panic where Unicode lowercasing expanded invalid UTF-8 bytes and invalidated source offsets. Preserved the minimized corpus input and verified the fix with race tests and an additional 20-second HTML fuzz run.
- Covered every browser workflow operation across successful execution, ordinary adapter errors, deadline expiry, and cancellation while preserving stable paths and causes.
- Audited external-transform output validation and recorded the required diagnostic-contract decision instead of changing observable error codes implicitly.
- Verified HTTP response-body closure on success, status failure, read failure, size rejection, body-read timeout, successful redirects, and policy-rejected redirects. Fixed the ordering contract in tests so `URLPolicy` is proven to run before a custom `CheckRedirect`; executor statement coverage increased to 74.6%.
- Moved browser external-transform preflight ahead of input/session validation, adapter acquisition, and navigation, so an unavailable host callback cannot cause browser activity. Added ordering coverage for transform, input, and session failures.
- Made external-transform preflight diagnostics deterministic by preserving declaration order instead of iterating a map. Added repeated-order, partial-registry, nested-recursion, call-stack cleanup, callback-cause, and malformed-IR tests.
- Documented and tested session header and cookie propagation across HTTP redirects. Sensitive headers remain on the same domain and its subdomains under Go's default redirect policy, are not forwarded to a different domain, and custom non-sensitive headers continue to follow redirects.
- Re-ran core, race, real go-rod, Chromium E2E, and release gates concurrently after the runtime safety changes; executor statement coverage increased to 75.8%.
- Added an HTTP preflight boundary matrix proving that missing external transforms, invalid selectors, browser-only value sources, missing or mistyped inputs, required sessions, and invalid expanded URLs all fail before the transport is called, with a successful request as the control.
- Hardened malformed built-in arguments: negative `regex-capture` groups now return an error instead of indexing a negative slice offset, and non-string `parse-bool` `true`/`false` values no longer collapse to empty strings. Removed the now-unnecessary `unicode/utf8` sentinel.
- Re-ran core, race, real go-rod, Chromium E2E, and release gates concurrently after the HTTP preflight and built-in hardening changes; executor statement coverage increased to 76.6%.
- Added a bounded malformed-argument fuzz target across regex capture/replacement, substring, split, boolean parsing, URL queries and paths, numeric assertions, and coalescing. A 20-second run completed approximately 1.60 million executions without finding another panic.
- Verified that the wrapped HTTP client preserves cookie-jar scope and custom redirect mutation: host-only cookies do not reach subdomains, domain cookies do, and a supplied `CheckRedirect` can remove session headers and cookies without runtime reinjection. Executor statement coverage increased to 76.8%.
- Extended browser lease coverage to reject a nil release callback before navigation and to prove exactly-once release after ordinary workflow and output-query failures. Executor statement coverage increased to 77.0%.
- Recorded the required compatibility and security decision for ambient `http.Client` jar or browser-context state under `session policy="none"`; current behavior was intentionally left unchanged.
- Verified response-cookie lifecycle through a supplied jar: redirect responses persist host-only and domain cookies with their proper scope, only the domain cookie reaches a subdomain, and final-response cookies are retained. Tests record cookie names only, never values.
- Added direct HTTP client wrapper tests proving nil-policy identity, non-nil-policy clone isolation, preservation of transport/jar/timeout, caller callback ordering, and policy rejection before the caller-owned redirect callback.
- Added browser output-IR preflight before input resolution and adapter acquisition. Nested malformed selectors, unknown output members, and unknown value sources now fail with existing structured codes before browser activity; valid extraction remains unchanged. Updated browser runtime execution-order documentation. Executor statement coverage increased to 77.1%.
- Added browser workflow-IR preflight for known step kinds and selector-bearing steps. Malformed workflow selectors and unknown steps now fail at a stable workflow path before adapter acquisition, while valid workflow order is preserved. Updated browser runtime documentation; executor statement coverage increased to 77.4%.
- Added direct URL-policy tests for nil and allowing policies, invalid URL rejection before callback invocation, structured policy rejection, `url.Error` wrapping from `http.Client`, cause preservation, and non-conversion of unrelated transport errors. Executor statement coverage increased to 77.5%.
- Rejected trailing data after IR JSON literals instead of silently accepting the first valid prefix. Added direct single-value decoding tests and an `ExecuteHTML` malformed-default regression covering stable `E_IR_INVALID` mapping.
- Fixed runtime integer input normalization at the rounded `float64` upper boundary: logical `2^63` is now rejected instead of saturating to `MaxInt64`, while `MinInt64` and the largest representable float below `2^63` remain valid.
- Made multiple unknown runtime inputs deterministic by reporting the lexicographically first name in one pass. Added coverage for every accepted input representation, invalid and non-finite values, defaults, provided overrides, trailing default data, type mismatches, and missing required inputs. Executor statement coverage increased to 80.3%.
- Normalized numeric field defaults to each field's resolved successful type in both HTTP and browser execution. Missing-value and error-recovery defaults now return consistent concrete values, while malformed out-of-range defaults in hand-built IR fail with a stable `E_IR_INVALID` path. Executor statement coverage increased to 81.0%.
- Added HTTP field-recovery parity coverage for null, warn, fail, and required-missing behavior, including warning order, structured error propagation, and the rule that `on-error` does not recover missing required values.
- Rejected unknown HTTP value sources during output-IR preflight before transport activity, matching browser-mode validation and preventing malformed hand-built IR from being hidden by field recovery. Executor statement coverage increased to 81.2%.
- Added transform-call preflight across declared pipelines and nested output fields. Nil targets, unknown built-ins, and missing declared transforms now retain their existing structured diagnostics while failing before HTTP transport or browser acquisition. Executor statement coverage increased to 81.6%.
- Stabilized session request construction by sorting header names before applying values in both HTTP and go-rod runtimes, while preserving per-header and duplicate-cookie input order. HTTP now ignores nil cookie entries instead of panicking, matching the rod adapter. Added 100-run ordering regressions and real-adapter helper coverage without logging secrets.
- Extended browser workflow-IR preflight to reject invalid wait states, non-positive timeouts and network-idle windows, and non-finite scroll coordinates before adapter acquisition. The explicit schema/compiler constraints remain unchanged; executor statement coverage increased to 82.0%.
- Recorded the compatibility decision required to define a portable upper bound for `timeout-ms`; no overflow-limit behavior was changed implicitly.
- Corrected HTML normalization for mixed-case start/end tag names and cascaded omitted table cell, row, and section closures across `thead`, `tbody`, and `tfoot`. Added raw-text closing-boundary and table-tree regressions plus fuzz seeds; a 20-second HTML fuzz run completed 28,788 executions without a panic. DOM statement coverage increased from 77.4% to 78.2%.
- Exercised the normative portable selector profile across every attribute operator, empty/child/type-position pseudo-classes, positive and negative `An+B` expressions, non-matches, and selector-list de-duplication in document order. DOM statement coverage increased to 85.8%.
- Added direct DOM node boundary coverage for nil receivers, non-element attributes, recursive text, element-only child/sibling traversal, and the distinction between empty, zero-length, whitespace, and element children. DOM statement coverage increased to 87.1%.
- Covered missing and malformed JSON for required `coalesce` and numeric assertion arguments, including preservation of decoding causes without changing transform diagnostics. Executor statement coverage increased to 82.3%.
- Recorded the diagnostic-contract decision required for canceling offline `ExecuteHTML` parsing and extraction; no existing operation-specific code was broadened implicitly.
- Added structure regressions and fuzz seeds for omitted description-list, ruby-annotation, select-option, and paragraph end tags within the parser's documented normalization scope. DOM statement coverage increased to 87.8%.
- Enforced normative URL built-in boundaries in malformed IR: `path-segment` now requires its declared `index`, `url-query` rejects negative indexes, and malformed percent-encoded queries propagate parsing errors instead of becoming null. Added decoded, empty, absent, out-of-range, negative, and malformed URL/path coverage; executor statement coverage increased to 82.8%.
- Enforced the portable regex profile in malformed IR by rejecting duplicate/unsupported flags, named captures, negative literal/regex replacement counts, and capture indexes beyond the compiled group count. Distinguished an unmatched optional capture from an invalid group, and covered zero/all/limited/empty-match replacement plus `$` expansion. A 20-second malformed built-in fuzz run completed approximately 1.42 million executions without a panic; executor statement coverage increased to 83.7%.
- Corrected declared US-ASCII decoding to reject every byte above `0x7f` instead of accepting arbitrary valid UTF-8. Added valid ASCII, invalid HTTP charset metadata with meta fallback, custom decoder byte/label propagation, deterministic alias normalization, and structured fallback-cause coverage; executor statement coverage increased to 84.2%.
- Enforced required `substring start` and non-negative `split limit` in malformed IR, matching the compiler and built-in specification. Added Unicode scalar, negative/upper clamp, reversed range, zero/unlimited split, empty input, missing arguments, fractional values, and malformed JSON coverage; executor statement coverage increased to 84.9%.
- Rejected duplicate named transform arguments and malformed positional or named argument JSON during runtime preflight. Hand-built IR now retains structured `E_TRANSFORM` paths and decoding causes while failing before HTTP transport or browser lease acquisition; executor statement coverage increased to 85.0%.
- Rejected NaN and positive or negative infinity in numeric assertion comparisons. `math/big` accepts infinity without a parse error, so malformed or host-originated non-finite values previously bypassed `assert-min`/`assert-max`; added integer target/radix, malformed argument, exact `MaxUint64`, non-numeric, and non-finite regression coverage. Executor statement coverage increased to 85.9%.
- Preflighted declared match-transform case inputs, case results, and defaults for valid JSON and compatibility with their declared types. Malformed hand-built IR now fails with its transform path and preserved cause before HTTP transport or browser acquisition; executor statement coverage increased to 86.1%.
- Centralized runtime built-in signatures and preflighted call names and arity. Unknown or missing named arguments, forbidden positional arguments, empty `assert-enum` calls, and arguments on declared transforms now fail before source acquisition instead of being ignored; executor statement coverage increased to 86.3%.

## Commits

- `7be651b` test: isolate rod verification workspace
- `2290f56` test: repair rod JavaScript E2E fixture
- `760e043` test: cover deterministic diagnostic output
- `63040f5` test: exercise type relationships and classifiers
- `37a85a9` test: cover CLI parsing boundaries
- `4f4656e` docs: record secure CLI session input decision
- `033b762` chore: remove staticcheck dead code
- `3f53d1f` test: reuse registered diagnostic codes
- `39bcf0f` fix: support macOS release checksums
- `b8209a7` fix: clean release stages on build failure
- `21320fa` docs: refresh validation record
- `7fb7a60` test: isolate all rod module metadata
- `764f7ae` test: cover scoped rod JavaScript execution
- `51a251b` test: cover HTML charset decoding boundaries
- `6b8cfd5` fix: stop canceled HTTP fetches before transport
- `99e951f` test: cover source spans and KDL AST helpers
- `a05a6e4` test: cover public program API contracts
- `dd0103c` docs: record subcommand help exit decision
- `ea0eb0c` docs: clarify HTML BOM precedence
- `465e9fa` fix: reject invalid JSON number results
- `d3db672` fix: enforce JavaScript return declarations
- `686c9b1` test: exercise CLI command workflows
- `626a019` test: align browser field recovery coverage
- `66a947f` test: cover browser collection recovery
- `6ce917e` docs: record browser result representation decision
- `88c5bb5` test: cover browser workflow compilation
- `e8f4cdf` test: cover compiler default values
- `91a5bb3` test: cover browser adapter read failures
- `ea4da6c` fix: normalize scalar transform literals
- `1608ce9` test: cover numeric conversion boundaries
- `27a76b8` fix: classify unsupported selector flags
- `16b8936` test: cover templates and selector diagnostics
- `3f39a34` fix: preserve raw-text offsets for invalid UTF-8
- `d020067` test: cover browser workflow failures
- `e6942ef` docs: record external transform type decision
- `dd88d5c` test: verify HTTP response cleanup
- `385f3a1` fix: preflight browser transforms before navigation
- `ec3293c` test: cover browser preflight ordering
- `3410810` fix: stabilize transform preflight order
- `ae0c0d8` test: cover session redirect boundaries
- `6a511aa` test: enforce HTTP preflight boundary
- `1eab03f` fix: harden malformed builtin arguments
- `0c5081d` test: fuzz malformed builtin arguments
- `9f682d1` test: verify redirect client ownership
- `e1f2798` test: extend browser lease cleanup
- `a7b0a44` docs: record ambient session state decision
- `422f5e8` test: cover response cookie lifecycle
- `c0d6a22` test: verify HTTP client clone isolation
- `270b01a` fix: preflight browser output IR
- `e66744f` docs: clarify browser preflight order
- `20465ba` fix: preflight browser workflow IR
- `4786118` docs: include workflow preflight
- `f4b4c9f` test: cover URL policy error mapping
- `d982b5e` fix: reject trailing IR JSON data
- `13d23f7` fix: reject rounded integer overflow
- `44383db` fix: stabilize unknown input errors
- `cf172fb` test: cover runtime input defaults
- `afb431b` fix: normalize numeric field defaults
- `3603059` test: cover HTTP field recovery policies
- `e939fb9` fix: preflight unknown HTTP value sources
- `fd06c4d` fix: preflight malformed transform calls
- `fa39d8b` fix: stabilize session request construction
- `769dace` fix: preflight malformed browser workflow values
- `3734988` docs: record workflow timeout limit decision
- `e523572` fix: recover mixed-case and table HTML
- `51591fc` test: cover portable selector profile
- `55d9a41` test: cover DOM node boundaries
- `43d513f` test: cover required builtin arguments
- `e85deee` docs: record offline cancellation decision
- `203d05a` test: cover optional HTML end tags
- `0dd79c1` fix: enforce URL builtin boundaries
- `e5bff83` fix: enforce portable regex boundaries
- `dcfccb6` fix: validate declared ASCII responses
- `a3c136a` fix: enforce substring and split arguments
- `5fe3f20` docs: record charset and argument hardening
- `13a2170` fix: preflight transform argument encoding
- `5ba4613` docs: record transform argument preflight
- `d0ba18c` fix: reject non-finite numeric assertions
- `455ef0f` docs: record numeric assertion hardening
- `fca3cfb` fix: preflight match transform literals
- `772e22b` docs: record match literal preflight
- `2df2402` fix: preflight transform call signatures

## Verification results

Passed:

- `make verify`, including `GOTOOLCHAIN=local go vet ./...`, `go test ./...`, `go test -race ./...`, build, fixture validation/extraction, golden checks, diagnostic registry checks, formatting, module checks, and rod contract tests;
- `make test-rod` against `github.com/go-rod/rod v0.116.2`;
- `make test-rod-e2e` with Chromium;
- `make release-check`;
- concurrent execution of all four gates above;
- `go test -shuffle=on -count=10 ./...`;
- 10-second fuzz runs for KDL and selector parsing, a post-fix 20-second HTML parser run covering approximately 1.64 million executions, a second 20-second HTML boundary run completing 28,788 executions, and malformed built-in argument runs covering approximately 1.60 million and 1.42 million executions;
- `staticcheck ./...` for the root and adapter modules;
- `govulncheck ./...` for the root and adapter modules, with no reachable vulnerabilities found;
- `go mod verify` for both modules and `go mod tidy -diff` for the root module;
- `actionlint` and `bash -n scripts/*.sh`;
- Linux amd64 and macOS arm64 release archive builds and SHA-256 verification;
- focused executor and CLI race tests after cancellation, JavaScript return, and command-workflow changes;
- root statement coverage at 89.1%, CLI coverage at 88.8%, compiler coverage at 72.0%, DOM coverage at 87.8%, executor coverage at 86.3%, and source package coverage at 100%.

## Unresolved failures

None. Useful transient failures resolved during the run included the E2E fixture's invalid JavaScript, concurrent rod verification corrupting temporary module metadata state, a regression test demonstrating that `net/http` can invoke a custom transport for an already-canceled request unless the runtime checks cancellation first, an HTML fuzz input that triggered a raw-text slice-bounds panic with invalid UTF-8, a malformed negative `regex-capture` group that reached a negative slice index, trailing data accepted after an IR JSON value, rounded `float64` input at logical `2^63` saturating into the signed integer range, numeric field defaults leaking raw `json.Number` values instead of their resolved runtime types, unknown HTTP value sources reaching transport activity before malformed-IR rejection, malformed transform calls reaching transport or browser activity before failure, nondeterministic duplicate session-header ordering, an HTTP nil-cookie panic path, malformed workflow values reaching browser operations instead of failing preflight, mixed-case raw-text closing tags rejected by the XML tokenizer, omitted table sections nesting under cells, malformed query escapes silently converted to absent query values, malformed regex IR bypassing portable flag, capture, and count constraints, non-ASCII UTF-8 accepted under US-ASCII, missing/negative substring and split arguments interpreted as defaults, duplicate or malformed transform arguments deferred until transform application, infinity accepted by numeric assertions through `math/big` parsing, malformed match literals deferred until field extraction, and invalid transform call names or arity ignored until application.

## Environment-limited verification

- The branch was not pushed, so GitHub-hosted Linux/macOS Actions were not run against these commits.
- No tag, GitHub Release, merge, or main-branch mutation was performed.
- Windows verification was not attempted because Windows is explicitly unsupported.

## Decisions recorded

- Secure CLI session input: decide whether and how to replace or deprecate direct `--cookie` and sensitive `--header` values with file or standard-input based secret handling. See `docs/decision-needed.md`.
- Subcommand help exit status: decide when explicit `validate --help`, `compile --help`, and `extract --help` should change from status 2 to status 0. See `docs/decision-needed.md`.
- Go representation of browser JavaScript results: define the concrete adapter result types accepted for logical JSON arrays, objects, and numbers. See `docs/decision-needed.md`.
- External transform result-type diagnostics: choose the public diagnostic used when a host callback returns a value incompatible with its declared output type. See `docs/decision-needed.md`.
- Ambient state under `session policy="none"`: define whether only explicit `Session` input is ignored or whether host-owned cookie jars and browser contexts must also be stateless. See `docs/decision-needed.md`.
- Workflow timeout upper bound: define the portable maximum before Go `time.Duration` conversion and the behavior for larger positive values. See `docs/decision-needed.md`.
- Offline `ExecuteHTML` cancellation: define the cancellation boundary and a structured diagnostic that is not tied to HTTP, browser, or transform operations. See `docs/decision-needed.md`.

## Next safe candidates

- Audit duplicate declared transform symbol IDs so malformed IR cannot silently overwrite an earlier declaration in the runtime lookup map.
- Audit duplicate output member IDs in hand-built IR before extraction to prevent ambiguous result-map overwrites.
