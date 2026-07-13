#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
module="$root/adapters/rod"
backup="$(mktemp)"
cp "$module/go.mod" "$backup"
restore() {
  cp "$backup" "$module/go.mod"
  rm -f "$backup"
}
trap restore EXIT

cat >> "$module/go.mod" <<MOD

replace github.com/go-rod/rod => ../../testdata/rodstub
replace github.com/hsblabs/scrape-kdl => ../..
MOD

(
  cd "$module"
  GOWORK=off GOTOOLCHAIN=local go test ./...
  GOWORK=off GOTOOLCHAIN=local go vet ./...
)
