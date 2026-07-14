#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

for name in basic-http race-detail; do
  tmp="$tmp_dir/$name.ir.json"
  expected="./fixtures/expected-ir/$name.ir.json"
  GOTOOLCHAIN=local go run ./cmd/scrape-kdl compile "./fixtures/valid/$name.kdl" --out "$tmp" >/dev/null
  cmp -s "$tmp" "$expected" || {
    diff -u "$expected" "$tmp" || true
    echo "golden IR is stale: $expected" >&2
    exit 1
  }
done
