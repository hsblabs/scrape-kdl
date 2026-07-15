.PHONY: test race vet build format-check module-check golden diagnostics examples validate-example extract-example verify
.PHONY: test-rod-contract test-rod test-rod-e2e release-check release-dist

test:
	GOTOOLCHAIN=local go test ./...

race:
	GOTOOLCHAIN=local go test -race ./...

vet:
	GOTOOLCHAIN=local go vet ./...

build:
	GOTOOLCHAIN=local go build ./cmd/scrape-kdl

format-check:
	./scripts/check-format.sh

module-check:
	./scripts/check-module-paths.sh

golden:
	./scripts/check-golden.sh

diagnostics:
	./scripts/check-diagnostics.sh

examples:
	GOTOOLCHAIN=local go run ./cmd/check-examples

validate-example:
	GOTOOLCHAIN=local go run ./cmd/scrape-kdl validate ./fixtures/valid/race-detail.kdl

extract-example:
	GOTOOLCHAIN=local go run ./cmd/scrape-kdl extract ./fixtures/valid/basic-http.kdl --html ./fixtures/html/basic-http.html

test-rod-contract:
	./scripts/verify-rod-contract.sh

test-rod:
	./scripts/verify-rod.sh

test-rod-e2e:
	./scripts/verify-rod.sh --e2e

verify: format-check module-check golden diagnostics examples vet test race build validate-example extract-example test-rod-contract

release-check:
	./scripts/verify-release.sh

release-dist:
	./scripts/build-release.sh "$${VERSION:?set VERSION=vX.Y.Z}" dist
