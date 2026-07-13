# Validation

Validated on 2026-07-14 with Go 1.26.5 on macOS arm64.

## Integrated release check

Passed:

```bash
./scripts/verify-release.sh
```

This includes:

- `gofmt` verification;
- module-path and release-clean nested `go.mod` checks;
- golden Validated IR comparison;
- implementation-to-diagnostics documentation consistency;
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
  -fuzz=FuzzParseHTMLNeverPanics -fuzztime=10s
```

No crash or hang was found. Scheduled CI runs each target for two minutes.

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
- core and adapter semantic-version tag validation;
- `manifest.json` JSON parsing;
- source ZIP integrity and post-extraction root-module tests.

## Static, module, and vulnerability checks

Passed:

- `go mod verify` and `go mod tidy -diff` for the root module;
- `go mod verify` for the go-rod adapter through an isolated local workspace;
- `staticcheck ./...` for the root module and go-rod adapter;
- `govulncheck ./...` for the root module and go-rod adapter, with no reachable vulnerabilities found;
- ten shuffled repetitions of the root test suite.

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
