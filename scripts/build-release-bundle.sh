#!/usr/bin/env bash
set -euo pipefail

version="${1:?version tag is required}"
out="${2:-dist}"
npm_access="${NPM_ACCESS:-restricted}"
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
"$root/scripts/validate-release-tag.sh" core "$version"
case "$npm_access" in
  public|restricted) ;;
  *) echo "invalid npm publish access: $npm_access" >&2; exit 1 ;;
esac

out_abs="$("$root/scripts/resolve-release-output.sh" "$root" "$out")"

bundle="$(mktemp -d)"
trap 'rm -rf "$bundle"' EXIT
artifacts="$bundle/artifacts"
"$root/scripts/build-release.sh" "$version" "$artifacts"
rm "$artifacts/checksums.txt"
node "$root/scripts/build-npm-release.mjs" --version "${version#v}" --access "$npm_access" --output "$artifacts"
"$root/scripts/write-release-checksums.sh" "$artifacts"
"$root/scripts/verify-release-bundle.sh" "$version" "$artifacts" "$npm_access"

rm -rf "$out_abs"
mv "$artifacts" "$out_abs"
printf 'release bundle: %s\n' "$out_abs"
