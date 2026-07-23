#!/usr/bin/env bash
set -euo pipefail

version="${1:?version tag is required}"
directory="${2:?release directory is required}"
npm_access="${3:-restricted}"
root="$(cd "$(dirname "$0")/.." && pwd)"
"$root/scripts/validate-release-tag.sh" core "$version"
case "$npm_access" in
  public|restricted) ;;
  *) echo "invalid npm publish access: $npm_access" >&2; exit 1 ;;
esac
plain="${version#v}"

for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  archive="$directory/scrape-kdl_${plain}_${target}.tar.gz"
  [[ -f "$archive" ]] || { echo "missing CLI release archive: $archive" >&2; exit 1; }
  contents="$(tar -tzf "$archive")"
  for required in scrape-kdl LICENSE NOTICE README.md; do
    grep -qx "$required" <<<"$contents" || { echo "$archive is missing $required" >&2; exit 1; }
  done
done

for archive in \
  "$directory/hsblabs-scrape-kdl-${plain}.tgz" \
  "$directory/hsblabs-scrape-kdl-playwright-${plain}.tgz"; do
  [[ -f "$archive" ]] || { echo "missing npm release archive: $archive" >&2; exit 1; }
  tar -xOzf "$archive" package/package.json | node -e '
    let input = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("data", (chunk) => { input += chunk; });
    process.stdin.on("end", () => {
      const manifest = JSON.parse(input);
      if (manifest.publishConfig?.access !== process.argv[1]) {
        throw new Error(`${manifest.name} publishConfig.access is ${manifest.publishConfig?.access}, expected ${process.argv[1]}`);
      }
    });
  ' "$npm_access"
done
[[ -f "$directory/checksums.txt" ]] || { echo "missing release checksums: $directory/checksums.txt" >&2; exit 1; }

(
  cd "$directory"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -c checksums.txt
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 -c checksums.txt
  else
    echo "no SHA-256 checksum utility found (expected sha256sum or shasum)" >&2
    exit 1
  fi
)
printf 'release bundle verified: %s\n' "$directory"
