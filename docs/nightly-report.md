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

## Verification results

Passed:

- `make verify`, including `GOTOOLCHAIN=local go vet ./...`, `go test ./...`, `go test -race ./...`, build, fixture validation/extraction, golden checks, diagnostic registry checks, formatting, module checks, and rod contract tests;
- `make test-rod` against `github.com/go-rod/rod v0.116.2`;
- `make test-rod-e2e` with Chromium;
- `make release-check`;
- concurrent execution of all four gates above;
- `go test -shuffle=on -count=10 ./...`;
- 10-second fuzz runs for KDL parsing, selector parsing, and HTML parsing;
- `staticcheck ./...` for the root and adapter modules;
- `govulncheck ./...` for the root and adapter modules, with no reachable vulnerabilities found;
- `go mod verify` for both modules and `go mod tidy -diff` for the root module;
- `actionlint` and `bash -n scripts/*.sh`;
- Linux amd64 and macOS arm64 release archive builds and SHA-256 verification;
- focused executor and CLI race tests after cancellation, JavaScript return, and command-workflow changes;
- root statement coverage at 89.1%, CLI coverage at 88.8%, compiler coverage at 70.0%, and source package coverage at 100%.

## Unresolved failures

None. Useful transient failures resolved during the run included the E2E fixture's invalid JavaScript, concurrent rod verification corrupting temporary module metadata state, and a regression test demonstrating that `net/http` can invoke a custom transport for an already-canceled request unless the runtime checks cancellation first.

## Environment-limited verification

- The branch was not pushed, so GitHub-hosted Linux/macOS Actions were not run against these commits.
- No tag, GitHub Release, merge, or main-branch mutation was performed.
- Windows verification was not attempted because Windows is explicitly unsupported.

## Decisions recorded

- Secure CLI session input: decide whether and how to replace or deprecate direct `--cookie` and sensitive `--header` values with file or standard-input based secret handling. See `docs/decision-needed.md`.
- Subcommand help exit status: decide when explicit `validate --help`, `compile --help`, and `extract --help` should change from status 2 to status 0. See `docs/decision-needed.md`.
- Go representation of browser JavaScript results: define the concrete adapter result types accepted for logical JSON arrays, objects, and numbers. See `docs/decision-needed.md`.

## Next safe candidates

- Cover browser adapter read/query failures and their structured error/cancellation propagation.
- Add focused built-in edge tests for array joining, numeric `to-string`, and invalid argument combinations already defined by the built-in specification.
- Add deterministic compiler tests for URL-template escaping and unsupported selector diagnostics.
- Re-run static analysis, vulnerability checks, shuffle repetitions, and fuzz smoke after the next substantive runtime change.
