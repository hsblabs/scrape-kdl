#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
GOTOOLCHAIN=local go run ./cmd/scrape-kdl compile ./fixtures/valid/race-detail.kdl --out "$tmp" >/dev/null
cmp -s "$tmp" ./fixtures/expected-ir/race-detail.ir.json || {
  diff -u ./fixtures/expected-ir/race-detail.ir.json "$tmp" || true
  echo 'golden IR is stale' >&2
  exit 1
}
