# Validation

Validated on 2026-07-13 with Go 1.23.2 in the artifact build environment.

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
GOMAXPROCS=2 go test ./internal/kdl -run=^$ \
  -fuzz=FuzzParseNeverPanics -fuzztime=3s
GOMAXPROCS=2 go test ./internal/dom -run=^$ \
  -fuzz=FuzzParseSelectorNeverPanics -fuzztime=3s
GOMAXPROCS=2 go test ./internal/dom -run=^$ \
  -fuzz=FuzzParseHTMLNeverPanics -fuzztime=3s
```

No crash or hang was found. Scheduled CI runs each target for two minutes.

## Release archive smoke

The release builder produced and verified both archive formats:

```bash
SCRAPE_KDL_RELEASE_TARGETS='linux/amd64 darwin/amd64' \
  ./scripts/build-release.sh v0.1.0 /tmp/scrape-kdl-dist-smoke
sha256sum -c /tmp/scrape-kdl-dist-smoke/checksums.txt
```

Linux and macOS tar archives passed archive-integrity checks. The default release matrix contains four supported targets. Windows targets are rejected explicitly.

## Workflow and metadata checks

Passed:

- all `.github/**/*.yml` files parsed as YAML;
- core and adapter semantic-version tag validation;
- `manifest.json` JSON parsing;
- source ZIP integrity and post-extraction root-module tests.

## External dependency boundary

The environment cannot resolve an external Go module proxy. Therefore the actual `github.com/go-rod/rod` dependency and Chromium E2E were not executed locally.

The repository contains:

- real-dependency compilation and vet jobs in `.github/workflows/ci.yml`;
- scheduled Chromium E2E in `.github/workflows/browser-e2e.yml`;
- a local API contract stub used only for offline build verification.

Before the first public tag, the real-dependency and browser workflows must pass in GitHub Actions.

## Platform support

- Default release targets contain no `windows/*`.
- `SCRAPE_KDL_RELEASE_TARGETS=windows/amd64` fails explicitly.
- Core CI covers Linux and macOS.
- Windows-1252 decoding remains because it is an HTML character encoding, not OS support.
