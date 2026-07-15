.PHONY: test race vet build format-check module-check golden diagnostics ir-contract api-contract typescript-contract conformance-coverage conformance validate-example extract-example verify
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

ir-contract:
	GOTOOLCHAIN=local go test ./internal/canonicaljson ./internal/ir ./scripts -run 'Test(Canonical|IR)'

api-contract:
	GOTOOLCHAIN=local go test ./testdata/api-consumers/go
	GOTOOLCHAIN=local go test ./scripts -run TestGoPublicSignatures
	npm run typecheck:api

typescript-contract:
	npm run test:contract-slice

conformance-coverage:
	GOTOOLCHAIN=local go test ./scripts -run TestConformanceCoverage

conformance:
	./scripts/check-conformance.sh

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

verify: format-check module-check golden diagnostics ir-contract api-contract typescript-contract conformance-coverage conformance vet test race build validate-example extract-example test-rod-contract

release-check:
	./scripts/verify-release.sh

release-dist:
	./scripts/build-release.sh "$${VERSION:?set VERSION=vX.Y.Z}" dist
