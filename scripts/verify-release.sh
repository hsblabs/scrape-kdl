#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

./scripts/check-format.sh
./scripts/check-module-paths.sh
./scripts/check-golden.sh
./scripts/check-diagnostics.sh
GOTOOLCHAIN=local go run ./cmd/check-examples
GOTOOLCHAIN=local go vet ./...
GOTOOLCHAIN=local go test ./...
GOTOOLCHAIN=local go test -race ./...
GOTOOLCHAIN=local go build ./cmd/scrape-kdl
GOTOOLCHAIN=local go run ./cmd/scrape-kdl validate ./fixtures/valid/race-detail.kdl
GOTOOLCHAIN=local go run ./cmd/scrape-kdl extract ./fixtures/valid/basic-http.kdl --html ./fixtures/html/basic-http.html >/dev/null
GOTOOLCHAIN=local go run ./cmd/scrape-kdl version
./scripts/verify-rod-contract.sh
