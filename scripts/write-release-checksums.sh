#!/usr/bin/env bash
set -euo pipefail

directory="${1:?release directory is required}"
cd "$directory"

if command -v sha256sum >/dev/null 2>&1; then
  checksum=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
  checksum=(shasum -a 256)
else
  echo "no SHA-256 checksum utility found (expected sha256sum or shasum)" >&2
  exit 1
fi

"${checksum[@]}" ./* > checksums.txt
