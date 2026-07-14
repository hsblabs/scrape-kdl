# Validation

Validated on 2026-07-15 with Go 1.26.5 and Node.js 26.4.0 on macOS arm64.

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
- TypeScript package typechecking, source lint, tests, coverage thresholds, intended-file allowlisting, secret/local-path inspection, `npm pack`, and a clean-consumer install and import smoke test;
- manifest-complete Go conformance execution and exact comparison of shared Go/TypeScript observations in the dated language-neutral result format;
- `go vet ./...`;
- `go test ./...`;
- `go test -race ./...`;
- CLI build, validation, offline extraction, and version output;
- go-rod adapter compilation and tests against the local API contract stub.

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
- the Node.js 26 TypeScript contract-slice and conformance-coverage workflow paths;
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
