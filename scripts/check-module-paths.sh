#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

[[ "$(head -n1 go.mod)" == 'module github.com/hsblabs/scrape-kdl' ]]
[[ "$(head -n1 adapters/rod/go.mod)" == 'module github.com/hsblabs/scrape-kdl/adapters/rod' ]]
[[ "$(head -n1 scripts/releaseplan/go.mod)" == 'module github.com/hsblabs/scrape-kdl/scripts/releaseplan' ]]

for module in go.mod adapters/rod/go.mod docs/ir/go/go.mod scripts/releaseplan/go.mod testdata/rodstub/go.mod; do
  if ! grep -qx 'go 1.26' "$module"; then
    echo "$module must declare Go 1.26" >&2
    exit 1
  fi
done

if grep -RInE 'github.com/(hsb-forge|mktbsh)/scrape-kdl'   --exclude-dir=.git --exclude='*.zip' .; then
  echo 'legacy module path found' >&2
  exit 1
fi

if grep -n '^replace ' adapters/rod/go.mod; then
  echo 'published adapter go.mod must not contain replace directives' >&2
  exit 1
fi
