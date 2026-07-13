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
backup="$(mktemp -d)"
had_sum=false

cp "$module/go.mod" "$backup/go.mod"
if [[ -f "$module/go.sum" ]]; then
  cp "$module/go.sum" "$backup/go.sum"
  had_sum=true
fi

restore() {
  cp "$backup/go.mod" "$module/go.mod"
  if [[ "$had_sum" == true ]]; then
    cp "$backup/go.sum" "$module/go.sum"
  else
    rm -f "$module/go.sum"
  fi
  rm -rf "$backup"
}
trap restore EXIT

(
  cd "$module"
  GOWORK=off GOTOOLCHAIN=local go mod edit \
    -replace "github.com/hsblabs/scrape-kdl=$root"

  if [[ "$mode" == "--e2e" ]]; then
    GOWORK=off GOTOOLCHAIN=local go test -tags=e2e -timeout=15m ./...
  else
    GOWORK=off GOTOOLCHAIN=local go test ./...
    GOWORK=off GOTOOLCHAIN=local go vet ./...
  fi
)
