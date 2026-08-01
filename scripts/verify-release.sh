#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

./scripts/check-format.sh
./scripts/check-module-paths.sh
make release-plan
make release-publish-contract
./scripts/check-golden.sh
./scripts/check-diagnostics.sh
GOTOOLCHAIN=local go run ./cmd/check-examples
make examples-typescript
make api-contract
make package-go
make typescript-package
make conformance-coverage
make conformance
make html-differential
make release-matrix
make performance
make support-matrix
make hardening
GOTOOLCHAIN=local go vet ./...
GOTOOLCHAIN=local go test ./...
GOTOOLCHAIN=local go test -race ./...
GOTOOLCHAIN=local go build ./cmd/scrape-kdl
GOTOOLCHAIN=local go run ./cmd/scrape-kdl validate ./fixtures/valid/race-detail.kdl
GOTOOLCHAIN=local go run ./cmd/scrape-kdl extract ./fixtures/valid/basic-http.kdl --html ./fixtures/html/basic-http.html >/dev/null
GOTOOLCHAIN=local go run ./cmd/scrape-kdl version
./scripts/verify-rod-contract.sh

release_bundle_root="$(mktemp -d)"
trap 'rm -rf "$release_bundle_root"' EXIT
make release-dist VERSION=v1.0.0-rc.1 OUT="$release_bundle_root/dist"
