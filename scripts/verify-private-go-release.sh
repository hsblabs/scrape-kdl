#!/usr/bin/env bash
set -euo pipefail

core_version="${1:?core private version is required, for example v0.9.0-private.1}"
rod_version="${2:?go-rod private version is required, for example adapters/rod/v0.9.0-private.1}"
root="$(cd "$(dirname "$0")/.." && pwd)"

"$root/scripts/validate-private-release-tag.sh" core "$core_version"
"$root/scripts/validate-private-release-tag.sh" rod "$rod_version"

consumer="$(mktemp -d)"
trap 'rm -rf "$consumer"' EXIT
cp "$root/testdata/private-go-consumer/consumer_test.go.txt" "$consumer/consumer_test.go"

(
  cd "$consumer"
  export GOPRIVATE=github.com/hsblabs/scrape-kdl
  export GONOSUMDB=github.com/hsblabs/scrape-kdl
  go mod init example.com/scrape-kdl-private-consumer
  go get "github.com/hsblabs/scrape-kdl@$core_version"
  go get "github.com/hsblabs/scrape-kdl/adapters/rod@${rod_version#adapters/rod/}"
  go test ./...
)
