#!/usr/bin/env bash
set -euo pipefail

kind="${1:?release kind is required: core or rod}"
tag="${2:?private release tag is required}"
root="$(cd "$(dirname "$0")/.." && pwd)"

"$root/scripts/validate-release-tag.sh" "$kind" "$tag"

case "$kind:$tag" in
  core:v*-private.[1-9]|core:v*-private.[1-9][0-9]*) ;;
  rod:adapters/rod/v*-private.[1-9]|rod:adapters/rod/v*-private.[1-9][0-9]*) ;;
  *)
    echo "private $kind release tag must end in -private.N with N >= 1: $tag" >&2
    exit 1
    ;;
esac
