#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
module="$root/adapters/rod"
workspace="$(mktemp -d)"
modfile="$workspace/rod.mod"

restore() {
  rm -rf "$workspace"
}
trap restore EXIT

cp "$module/go.mod" "$modfile"
if [[ -f "$module/go.sum" ]]; then
  cp "$module/go.sum" "$workspace/rod.sum"
fi
GOWORK=off GOTOOLCHAIN=local go mod edit -modfile="$modfile" \
  -replace "github.com/go-rod/rod=$root/testdata/rodstub" \
  -replace "github.com/hsblabs/scrape-kdl=$root"

(
  cd "$module"
  GOWORK=off GOTOOLCHAIN=local go test -modfile="$modfile" ./...
  GOWORK=off GOTOOLCHAIN=local go vet -modfile="$modfile" ./...
)
