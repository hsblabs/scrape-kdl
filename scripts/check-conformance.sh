#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

npm run build:contract-slice >/dev/null
node scripts/compare-conformance.test.mjs
GOTOOLCHAIN=local go run ./cmd/conformance-runner \
  --manifest conformance/manifest.json \
  --suite pr \
  --job core \
  --output "$tmp_dir/go.json"
node packages/scrape-kdl/test/manifest-runner.mjs \
  --manifest conformance/manifest.json \
  --suite typescript-slice \
  --job core \
  --output "$tmp_dir/typescript.json"
node scripts/compare-conformance.mjs \
  --manifest conformance/manifest.json \
  --go "$tmp_dir/go.json" \
  --typescript "$tmp_dir/typescript.json"
