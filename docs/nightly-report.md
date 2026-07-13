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

## Verification results

Passed:

- `make verify`, including `GOTOOLCHAIN=local go vet ./...`, `go test ./...`, `go test -race ./...`, build, fixture validation/extraction, golden checks, diagnostic registry checks, formatting, module checks, and rod contract tests;
- `make test-rod` against `github.com/go-rod/rod v0.116.2`;
- `make test-rod-e2e` with Chromium;
- `make release-check`;
- concurrent execution of all four gates above;
- `go test -shuffle=on -count=10 ./...`;
- 10-second fuzz runs for KDL and selector parsing, plus a post-fix 20-second HTML parser run covering approximately 1.64 million executions;
- `staticcheck ./...` for the root and adapter modules;
- `govulncheck ./...` for the root and adapter modules, with no reachable vulnerabilities found;
- `go mod verify` for both modules and `go mod tidy -diff` for the root module;
- `actionlint` and `bash -n scripts/*.sh`;
- Linux amd64 and macOS arm64 release archive builds and SHA-256 verification;
- focused executor and CLI race tests after cancellation, JavaScript return, and command-workflow changes;
- root statement coverage at 89.1%, CLI coverage at 88.8%, compiler coverage at 72.0%, executor coverage at 76.6%, and source package coverage at 100%.

## Unresolved failures

None. Useful transient failures resolved during the run included the E2E fixture's invalid JavaScript, concurrent rod verification corrupting temporary module metadata state, a regression test demonstrating that `net/http` can invoke a custom transport for an already-canceled request unless the runtime checks cancellation first, an HTML fuzz input that triggered a raw-text slice-bounds panic with invalid UTF-8, and a malformed negative `regex-capture` group that reached a negative slice index.

## Environment-limited verification

- The branch was not pushed, so GitHub-hosted Linux/macOS Actions were not run against these commits.
- No tag, GitHub Release, merge, or main-branch mutation was performed.
- Windows verification was not attempted because Windows is explicitly unsupported.

## Decisions recorded

- Secure CLI session input: decide whether and how to replace or deprecate direct `--cookie` and sensitive `--header` values with file or standard-input based secret handling. See `docs/decision-needed.md`.
- Subcommand help exit status: decide when explicit `validate --help`, `compile --help`, and `extract --help` should change from status 2 to status 0. See `docs/decision-needed.md`.
- Go representation of browser JavaScript results: define the concrete adapter result types accepted for logical JSON arrays, objects, and numbers. See `docs/decision-needed.md`.
- External transform result-type diagnostics: choose the public diagnostic used when a host callback returns a value incompatible with its declared output type. See `docs/decision-needed.md`.

## Next safe candidates

- Extend malformed HTML regression coverage around raw-text closing tags and optional-end-tag recovery without broadening the documented parser contract.
- Extend browser lease cleanup tests across ordinary adapter errors and cancellation interleavings without changing the public adapter contract.
- Add deterministic tests for cookie-jar scope and a custom `CheckRedirect` that removes sensitive headers, preserving standard `net/http` ownership of redirect behavior.
- Add a bounded fuzz target for malformed built-in transform calls to find panic-only failures without changing valid KDL semantics.
