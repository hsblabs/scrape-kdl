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

restore() {
  rm -rf "$workspace"
}
trap restore EXIT

(
  cd "$workspace"
  GOWORK=off GOTOOLCHAIN=local go work init "$module"
  GOWORK="$workspace/go.work" GOTOOLCHAIN=local go work edit \
    -replace "github.com/hsblabs/scrape-kdl=$root"
)

(
  cd "$module"

  if [[ "$mode" == "--e2e" ]]; then
    GOWORK="$workspace/go.work" GOTOOLCHAIN=local go test -tags=e2e -timeout=15m ./...
  else
    GOWORK="$workspace/go.work" GOTOOLCHAIN=local go test ./...
    GOWORK="$workspace/go.work" GOTOOLCHAIN=local go vet ./...
  fi
)
