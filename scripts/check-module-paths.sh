#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

[[ "$(head -n1 go.mod)" == 'module github.com/hsblabs/scrape-kdl' ]]
[[ "$(head -n1 adapters/rod/go.mod)" == 'module github.com/hsblabs/scrape-kdl/adapters/rod' ]]

if grep -RInE 'github.com/(hsb-forge|mktbsh)/scrape-kdl'   --exclude-dir=.git --exclude='*.zip' .; then
  echo 'legacy module path found' >&2
  exit 1
fi

if grep -n '^replace ' adapters/rod/go.mod; then
  echo 'published adapter go.mod must not contain replace directives' >&2
  exit 1
fi
