# Nightly improvement report

Date: 2026-07-13  
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
- Linux amd64 and macOS arm64 release archive builds and SHA-256 verification.

## Unresolved failures

None. Two useful transient failures were resolved during the run: the E2E fixture's invalid JavaScript and concurrent rod verification corrupting temporary module metadata state.

## Environment-limited verification

- The branch was not pushed, so GitHub-hosted Linux/macOS Actions were not run against these commits.
- No tag, GitHub Release, merge, or main-branch mutation was performed.
- Windows verification was not attempted because Windows is explicitly unsupported.

## Decisions recorded

- Secure CLI session input: decide whether and how to replace or deprecate direct `--cookie` and sensitive `--header` values with file or standard-input based secret handling. See `docs/decision-needed.md`.

## Next safe candidates

- Extend Chromium E2E coverage to current-scoped JavaScript and JavaScript failure propagation.
- Add focused charset sniffing tests for meta declarations and UTF-16 decoding.
- Add parent-context cancellation tests for HTTP fetch and browser workflows.
- Add direct tests for source span and KDL AST helper behavior where they improve diagnostic confidence.
