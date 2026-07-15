.PHONY: test race vet build format-check module-check golden diagnostics examples examples-go examples-typescript html-differential ir-contract api-contract typescript-contract typescript-package conformance-coverage conformance release-matrix hardening validate-example extract-example verify
.PHONY: package-go performance support-matrix support-matrix-target test-rod-contract test-rod test-rod-e2e test-playwright-e2e release-check release-gate release-dist

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
	$(MAKE) examples-go
	$(MAKE) examples-typescript

examples-go:
	GOTOOLCHAIN=local go run ./cmd/check-examples

examples-typescript:
	npm run examples:typescript

ir-contract:
	GOTOOLCHAIN=local go test ./internal/canonicaljson ./internal/ir ./scripts -run 'Test(Canonical|IR)'

api-contract:
	GOTOOLCHAIN=local go test ./testdata/api-consumers/go
	GOTOOLCHAIN=local go test ./scripts -run TestGoPublicSignatures
	npm run typecheck:api

package-go:
	./scripts/check-go-package.sh

performance:
	npm run build:typescript
	node ./scripts/check-performance.mjs

support-matrix:
	node ./scripts/check-support-matrix.mjs

support-matrix-target:
	node ./scripts/check-support-matrix.mjs --target "$${TARGET:?set TARGET=linux/amd64}"

typescript-contract:
	npm run test:contract-slice

typescript-package:
	npm run verify:typescript

conformance-coverage:
	GOTOOLCHAIN=local go test ./scripts -run TestConformanceCoverage

conformance:
	./scripts/check-conformance.sh

html-differential:
	GOTOOLCHAIN=local go test ./internal/dom -run TestPinnedHTMLCompatibilityManifest
	npm run build:typescript
	node --test packages/scrape-kdl/test/html-compatibility.test.mjs

release-matrix:
	node ./scripts/check-release-matrix.mjs

hardening:
	GOTOOLCHAIN=local go test ./internal/executor ./internal/compiler ./internal/ir
	GOTOOLCHAIN=local go test -race ./internal/executor ./internal/compiler
	npm run build:typescript
	node --test packages/scrape-kdl/test/runtime.test.mjs packages/scrape-kdl/test/browser-runtime.test.mjs
	GOTOOLCHAIN=local go test ./internal/kdl -run=^$$ -fuzz=FuzzParseNeverPanics -fuzztime=2s
	GOTOOLCHAIN=local go test ./internal/dom -run=^$$ -fuzz=FuzzParseSelectorNeverPanics -fuzztime=2s
	GOTOOLCHAIN=local go test ./internal/dom -run=^$$ -fuzz=FuzzParseHTMLNeverPanics -fuzztime=2s

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

test-playwright-e2e:
	npm run build:typescript
	npm run test:e2e --workspace @hsblabs/scrape-kdl-playwright

verify: format-check module-check golden diagnostics examples ir-contract api-contract typescript-package conformance-coverage conformance vet test race build validate-example extract-example test-rod-contract

release-check:
	./scripts/verify-release.sh

release-gate:
	./scripts/release-gate.sh

release-dist:
	./scripts/build-release.sh "$${VERSION:?set VERSION=vX.Y.Z}" dist
