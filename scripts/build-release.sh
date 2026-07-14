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
rm -rf "$out_abs"
mkdir -p "$out_abs"

commit="${GITHUB_SHA:-$(git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)}"
date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
ldflags="-s -w -X main.version=$version -X main.commit=$commit -X main.date=$date"
plain="${version#v}"
targets="${SCRAPE_KDL_RELEASE_TARGETS:-linux/amd64 linux/arm64 darwin/amd64 darwin/arm64}"

for target in $targets; do
  case "$target" in
    linux/amd64|linux/arm64|darwin/amd64|darwin/arm64) ;;
    windows/*) echo "Windows is not a supported release target: $target" >&2; exit 1 ;;
    *) echo "unsupported release target: $target" >&2; exit 1 ;;
  esac
  goos="${target%/*}"
  goarch="${target#*/}"
  name="scrape-kdl_${plain}_${goos}_${goarch}"
  echo "building ${goos}/${goarch}"
  (
    stage="$(mktemp -d)"
    trap 'rm -rf "$stage"' EXIT
    binary="scrape-kdl"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "$ldflags" -o "$stage/$binary" ./cmd/scrape-kdl
    cp LICENSE README.md "$stage/"
    tar -C "$stage" -czf "$out_abs/$name.tar.gz" "$binary" LICENSE README.md
  )
done
"$root/scripts/write-release-checksums.sh" "$out_abs"
