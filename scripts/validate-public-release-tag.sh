#!/usr/bin/env bash
set -euo pipefail

kind="${1:?release kind is required: core or rod}"
tag="${2:?release tag is required}"
root="$(cd "$(dirname "$0")/.." && pwd)"

"$root/scripts/validate-release-tag.sh" "$kind" "$tag"

if [[ "$tag" == *-private.* ]]; then
  echo "public $kind release tag must not use the private release channel: $tag" >&2
  exit 1
fi
