#!/usr/bin/env bash
set -euo pipefail

version="${1:?private npm version is required, for example 0.9.0-private.1}"
root="$(cd "$(dirname "$0")/.." && pwd)"
"$root/scripts/validate-private-release-tag.sh" core "v$version"

npm whoami >/dev/null
test "$(npm view "@hsblabs/scrape-kdl@$version" version)" = "$version"
test "$(npm view "@hsblabs/scrape-kdl-playwright@$version" version)" = "$version"

consumer="$(mktemp -d)"
trap 'rm -rf "$consumer"' EXIT
npm install \
  --prefix "$consumer" \
  --ignore-scripts \
  --no-audit \
  --no-fund \
  "@hsblabs/scrape-kdl@$version" \
  "@hsblabs/scrape-kdl-playwright@$version"
(
  cd "$consumer"
  node --input-type=module -e '
    const core = await import("@hsblabs/scrape-kdl");
    const adapter = await import("@hsblabs/scrape-kdl-playwright");
    if (typeof core.compile !== "function" || typeof adapter.PlaywrightAdapter !== "function") {
      throw new Error("private npm package exports are incomplete");
    }
  '
)
