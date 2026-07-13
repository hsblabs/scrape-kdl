#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf 'Usage: %s [--e2e]\n' "${0##*/}"
}

mode="${1:-}"
if [[ "$mode" == "-h" || "$mode" == "--help" ]]; then
  usage
  exit 0
fi
if (( $# > 1 )) || [[ -n "$mode" && "$mode" != "--e2e" ]]; then
  usage >&2
  exit 2
fi

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
  -replace "github.com/hsblabs/scrape-kdl=$root"

(
  cd "$module"

  if [[ "$mode" == "--e2e" ]]; then
    GOWORK=off GOTOOLCHAIN=local go test -modfile="$modfile" -tags=e2e -timeout=15m ./...
  else
    GOWORK=off GOTOOLCHAIN=local go test -modfile="$modfile" ./...
    GOWORK=off GOTOOLCHAIN=local go vet -modfile="$modfile" ./...
  fi
)
