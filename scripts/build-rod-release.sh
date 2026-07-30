#!/usr/bin/env bash
set -euo pipefail

version="${1:?go-rod version tag is required}"
out="${2:-dist}"
root="$(cd "$(dirname "$0")/.." && pwd)"
"$root/scripts/validate-release-tag.sh" rod "$version"

out_abs="$("$root/scripts/resolve-release-output.sh" "$root" "$out")"
build_dir="$(mktemp -d "${out_abs}.tmp.XXXXXX")"
trap 'rm -rf "$build_dir"' EXIT

commit="${GITHUB_SHA:-$(git -C "$root" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)}"
date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
ldflags="-s -w -X main.version=$version -X main.commit=$commit -X main.date=$date"
artifact_version="${version#adapters/rod/v}"
targets="${SCRAPE_KDL_RELEASE_TARGETS:-linux/amd64 linux/arm64 darwin/amd64 darwin/arm64}"

for target in $targets; do
  case "$target" in
    linux/amd64|linux/arm64|darwin/amd64|darwin/arm64) ;;
    windows/*) echo "Windows is not a supported release target: $target" >&2; exit 1 ;;
    *) echo "unsupported release target: $target" >&2; exit 1 ;;
  esac
  goos="${target%/*}"
  goarch="${target#*/}"
  name="scrape-kdl-rod_${artifact_version}_${goos}_${goarch}"
  echo "building ${goos}/${goarch}"
  (
    stage="$(mktemp -d)"
    trap 'rm -rf "$stage"' EXIT
    cd "$root/adapters/rod"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build \
      -trimpath \
      -ldflags "$ldflags" \
      -o "$stage/scrape-kdl-rod" \
      ./cmd/scrape-kdl-rod
    cp "$root/LICENSE" "$root/NOTICE" "$stage/"
    cp "$root/adapters/rod/README.md" "$stage/README.md"
    tar -C "$stage" -czf "$build_dir/$name.tar.gz" scrape-kdl-rod LICENSE NOTICE README.md
  )
done

"$root/scripts/write-release-checksums.sh" "$build_dir"
rm -rf "$out_abs"
mv "$build_dir" "$out_abs"
