# Validation

Validated on 2026-07-31 with Go 1.26.5, Node.js 26.4.0, and npm 11.17.0
on macOS arm64.

Local Playwright verification may use an installed Chromium-family executable
without downloading another browser:

```bash
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  make test-playwright-e2e
```

CI continues to install the matrix-selected Playwright browser and does not set
this override.

## Integrated release check

Passed:

```bash
npm ci
./scripts/verify-release.sh
```

This includes:

- `gofmt` verification;
- module-path and release-clean nested `go.mod` checks;
- golden Validated IR comparison;
- section-complete normative conformance coverage inventory checks;
- implementation-to-diagnostics documentation consistency;
- independent Go consumer compilation, public-signature internal-package checks, and strict TypeScript API consumer typechecking;
- independent TypeScript parser/compiler compilation of the representative valid fixture, exact canonical IR comparison, and exact shared invalid-fixture diagnostics;
- shared Go/TypeScript parser and import corpora covering exact acceptance, diagnostic messages, UTF-8 byte spans, ordering, invalid UTF-8, loader errors, aliases, document kinds, cycles, and bounded mutation smoke tests;
- TypeScript package typechecking, source lint, tests, coverage thresholds, intended-file allowlisting, secret/local-path inspection, `npm pack`, and a clean-consumer install and import smoke test;
- manifest-complete Go conformance execution and exact comparison of shared Go/TypeScript observations in the dated language-neutral result format;
- `go vet ./...`;
- `go test ./...`;
- `go test -race ./...`;
- CLI build, validation, offline extraction, and version output;
- go-rod adapter compilation and tests against the local API contract stub.

## Private distribution verification

The private-package changes passed:

- `make verify`;
- `make release-gate`, including the real go-rod dependency, go-rod Chromium
  E2E, Playwright Chromium E2E, race tests, security/resource hardening, and the
  Chromium WPT differential;
- a complete `v0.9.0-private.1` release bundle with four CLI archives, two
  restricted npm archives, clean npm consumers, license/notice payloads, and
  verified SHA-256 checksums;
- a separate public npm staging build proving that guarded public releases
  override the package metadata to `publishConfig.access=public`;
- private release workflow contract tests, private tag validation, and a
  go-rod archive-layout test;
- Actionlint, `pinact run --check --verify`, shell syntax checks, formatting,
  `go vet`, and `npm audit --omit=dev` with zero reported vulnerabilities.

No tag, GitHub Release, npm package, Pages deployment, or repository visibility
change was created. The npm account is not authenticated in this checkout, and
the package records do not yet exist. Protected GitHub Environments and
repository rulesets returned `403` for the current private-repository plan.
Actual private registry/module clean-consumer checks remain post-publication
gates because they require the owner-created tags, npm records, and read
credentials. The go-rod archive builder was verified with a fake cross-compiler;
the same source compiled and passed its real-dependency and Chromium tests in
`make release-gate`, but the final private-version archive cannot be built
without first tagging the core and updating the adapter dependency.

## Public release-candidate preparation

The `v1.0.0-rc.1` preparation passed:

- `make verify`;
- `make release-gate` with the installed Google Chrome executable selected
  through `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH`;
- Playwright Chromium E2E with five passing tests and forced test-runner exit
  after completion so a residual macOS `fsevents` handle cannot stall the gate;
- public-access staging of both npm packages, clean consumer smoke tests, four
  core CLI cross-builds, archive payload checks, and SHA-256 verification;
- public/private workflow separation tests, including rejection of
  `-private.N` versions by public workflows and GitHub prerelease flags for
  release-candidate tags;
- `npm audit --omit=dev`, with zero reported vulnerabilities;
- `actionlint` and `pinact run --check --verify`.

The go-rod browser contract and E2E tests passed. Its final release archive is
intentionally deferred until the public core candidate exists and
`adapters/rod/go.mod` can be updated from the published `v0.1.0` dependency to
`v1.0.0-rc.1` without a local `replace`.

## Architecture and performance hardening

The pre-Issue #18 architecture review changes passed `make verify`, `make release-check`, `make test-rod`, `make test-rod-e2e`, and `make test-playwright-e2e`. The checks cover the shared HTTP/browser output walker, immutable prepared execution plans, RE2-compatible TypeScript regular expressions, cross-runtime cancellation, exact public API/IR/diagnostic/built-in contract gates, and CLI writer failures.

The 10,000-element selector probe measured the following on Apple M2:

| Runtime | Full query | First-result query |
| --- | ---: | ---: |
| Go | 1.045 ms | 0.00017 ms |
| TypeScript | 2.322 ms | 0.001 ms |

The Go benchmark reported 1,061,877 B and 10,097 allocations per full query versus 24 B and 2 allocations for the first-result query. These observations are environment-specific evidence, not an absolute v1 performance SLA.

## Parser fuzz smoke

Passed three independent short fuzz runs:

```bash
go test ./internal/kdl -run=^$ \
  -fuzz=FuzzParseNeverPanics -fuzztime=10s
go test ./internal/dom -run=^$ \
  -fuzz=FuzzParseSelectorNeverPanics -fuzztime=10s
go test ./internal/dom -run=^$ \
  -fuzz=FuzzParseHTMLNeverPanics -fuzztime=20s
```

The HTML run first found an invalid-UTF-8 raw-text offset panic; the minimized input is now a committed fuzz corpus entry. After the fix, the 20-second rerun completed approximately 1.64 million executions without a crash or hang. Scheduled CI runs each target for two minutes.

## Release archive smoke

The release builder produced and verified both archive formats:

```bash
SCRAPE_KDL_RELEASE_TARGETS='linux/amd64 darwin/arm64' \
  ./scripts/build-release.sh v0.1.0 /tmp/scrape-kdl-dist-smoke
cd /tmp/scrape-kdl-dist-smoke
shasum -a 256 -c checksums.txt
```

Linux and macOS tar archives passed archive-integrity checks. Checksum generation is covered for both `sha256sum` on Linux and `shasum` on macOS, including the no-utility failure path. Temporary release stages are removed after both successful and failed builds. The default release matrix contains four supported targets. Windows targets are rejected explicitly.

## Workflow and metadata checks

Passed:

- all `.github/**/*.yml` files parsed as YAML;
- `actionlint` validation of all GitHub Actions workflows;
- the pinned Node.js 26 / `npm ci` API-contract workflow path;
- the complete Node.js 26 TypeScript contract and conformance-coverage workflow paths;
- the Node.js 26 package verification gate on Linux and macOS, including public root and `./node` entry-point imports;
- fixture registration, missing-artifact, suite-selection, result-schema, and unapproved-divergence failure tests;
- core and adapter semantic-version tag validation;
- `manifest.json` JSON parsing;
- source ZIP integrity and post-extraction root-module tests.

## Static, module, and vulnerability checks

Passed:

- `go mod verify` and `go mod tidy -diff` for the root module;
- `go mod verify` for the go-rod adapter through an isolated local workspace;
- `staticcheck` from `honnef.co/go/tools v0.7.0` for the root module and go-rod adapter;
- `govulncheck` from `golang.org/x/vuln v1.6.0` for the root module and go-rod adapter, with no reachable vulnerabilities found;
- `npm audit --omit=dev`, with no known runtime dependency vulnerabilities found;
- ten shuffled repetitions of the root test suite.

The scan tools were run through temporary `go install ...@latest` binaries. Adapter scans used a temporary source copy with a local root-module replacement so the committed release-clean `adapters/rod/go.mod` and `go.sum` remained unchanged.

## External adapter and browser checks

The environment resolved `github.com/go-rod/rod v0.116.2` and provided a downloadable Chromium build. Passed:

- `make test-rod-contract`;
- `make test-rod` against the real dependency;
- `make test-rod-e2e` with Chromium, including workflow interaction, document- and current-scoped JavaScript, JavaScript failure propagation, integer result normalization, live-DOM reads, and collection extraction.

The real-dependency and browser workflows remain the CI gates for Linux before the first public tag.

## Platform support

- Default release targets contain no `windows/*`.
- `SCRAPE_KDL_RELEASE_TARGETS=windows/amd64` fails explicitly.
- Core CI covers Linux and macOS.
- Windows-1252 decoding remains because it is an HTML character encoding, not OS support.
