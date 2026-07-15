#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
temporary="$(mktemp -d)"
trap 'rm -rf "$temporary"' EXIT
version="v1.0.0-rc.1"

cd "$root"
./scripts/build-release.sh "$version" "$temporary/dist"
node ./scripts/prepare-npm-release.mjs "${version#v}" "$temporary/dist"
rm "$temporary/dist/checksums.txt"
./scripts/write-release-checksums.sh "$temporary/dist"

expected=(
  "hsblabs-scrape-kdl-1.0.0-rc.1.tgz"
  "hsblabs-scrape-kdl-playwright-1.0.0-rc.1.tgz"
  "scrape-kdl_1.0.0-rc.1_darwin_amd64.tar.gz"
  "scrape-kdl_1.0.0-rc.1_darwin_arm64.tar.gz"
  "scrape-kdl_1.0.0-rc.1_linux_amd64.tar.gz"
  "scrape-kdl_1.0.0-rc.1_linux_arm64.tar.gz"
  "checksums.txt"
)
for artifact in "${expected[@]}"; do test -s "$temporary/dist/$artifact"; done
test "$(find "$temporary/dist" -type f | wc -l | tr -d ' ')" = "${#expected[@]}"

for archive in "$temporary"/dist/scrape-kdl_*.tar.gz; do
  test "$(tar -tzf "$archive" | sort | tr '\n' ' ')" = "LICENSE README.md scrape-kdl "
done
(
  cd "$temporary/dist"
  if command -v sha256sum >/dev/null 2>&1; then sha256sum -c checksums.txt
  else shasum -a 256 -c checksums.txt
  fi
)

native_os="$(go env GOOS)"
native_arch="$(go env GOARCH)"
tar -xzf "$temporary/dist/scrape-kdl_1.0.0-rc.1_${native_os}_${native_arch}.tar.gz" -C "$temporary"
"$temporary/scrape-kdl" version | grep -F "$version" >/dev/null
make package-go
echo "RC packaging: four CLI archives, checksums, two npm tarballs, native smoke, and packed Go consumer passed"
