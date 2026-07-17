#!/usr/bin/env bash
set -euo pipefail

version="${1:?version tag is required}"
out="${2:-dist}"
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
"$root/scripts/validate-release-tag.sh" core "$version"

case "$out" in
  /*) out_abs="$out" ;;
  *) out_abs="$root/$out" ;;
esac
if [[ "$out_abs" == "/" || "$out_abs" == "$root" ]]; then
  echo "refusing to replace unsafe release output directory: $out_abs" >&2
  exit 1
fi

bundle="$(mktemp -d)"
trap 'rm -rf "$bundle"' EXIT
artifacts="$bundle/artifacts"
"$root/scripts/build-release.sh" "$version" "$artifacts"
rm "$artifacts/checksums.txt"
node "$root/scripts/build-npm-release.mjs" --version "${version#v}" --output "$artifacts"
"$root/scripts/write-release-checksums.sh" "$artifacts"
"$root/scripts/verify-release-bundle.sh" "$version" "$artifacts"

mkdir -p "$(dirname "$out_abs")"
rm -rf "$out_abs"
mv "$artifacts" "$out_abs"
printf 'release bundle: %s\n' "$out_abs"
