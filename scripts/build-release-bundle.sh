#!/usr/bin/env bash
set -euo pipefail

version="${1:?version tag is required}"
out="${2:-dist}"
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
"$root/scripts/validate-release-tag.sh" core "$version"

out_abs="$("$root/scripts/resolve-release-output.sh" "$root" "$out")"

bundle="$(mktemp -d)"
trap 'rm -rf "$bundle"' EXIT
artifacts="$bundle/artifacts"
"$root/scripts/build-release.sh" "$version" "$artifacts"
rm "$artifacts/checksums.txt"
node "$root/scripts/build-npm-release.mjs" --version "${version#v}" --output "$artifacts"
"$root/scripts/write-release-checksums.sh" "$artifacts"
"$root/scripts/verify-release-bundle.sh" "$version" "$artifacts"

rm -rf "$out_abs"
mv "$artifacts" "$out_abs"
printf 'release bundle: %s\n' "$out_abs"
