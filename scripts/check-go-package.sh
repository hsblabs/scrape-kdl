#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
temporary="$(mktemp -d)"
cleanup() {
  chmod -R u+w "$temporary" 2>/dev/null || true
  rm -rf "$temporary"
}
trap cleanup EXIT

module="github.com/hsblabs/scrape-kdl"
version="v0.0.0"
proxy="$temporary/proxy/$module/@v"
consumer="$temporary/consumer"
mkdir -p "$proxy" "$consumer"

git -C "$root" archive --format=zip --prefix="$module@$version/" HEAD -- . \
  ':(exclude)adapters/rod' ':(exclude)docs/ir/go' ':(exclude)packages' \
  ':(exclude)scripts/releaseplan' ':(exclude)testdata/rodstub' \
  >"$proxy/$version.zip"
cp "$root/go.mod" "$proxy/$version.mod"
printf '{"Version":"%s","Time":"2026-07-15T00:00:00Z"}\n' "$version" >"$proxy/$version.info"
printf '%s\n' "$version" >"$proxy/list"

cp "$root/testdata/api-consumers/go/consumer_test.go" "$consumer/consumer_test.go"
cat >"$consumer/go.mod" <<EOF
module example.com/scrape-kdl-consumer

go 1.26

require $module $version
EOF

(
  cd "$consumer"
  export GOPROXY="file://$temporary/proxy,https://proxy.golang.org,direct"
  export GOMODCACHE="$temporary/modcache"
  export GONOSUMDB="$module"
  export GOSUMDB="sum.golang.org"
  export GOTOOLCHAIN=local
  go mod tidy
  go test ./...
)

echo "Go package check: clean consumer installed $module@$version from a module-proxy zip"
