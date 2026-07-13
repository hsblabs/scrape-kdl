#!/usr/bin/env bash
set -euo pipefail

kind="${1:?release kind is required: core or rod}"
tag="${2:?release tag is required}"
semver='(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?'

case "$kind" in
  core)
    pattern="^v${semver}$"
    ;;
  rod)
    pattern="^adapters/rod/v${semver}$"
    ;;
  *)
    echo "unknown release kind: $kind" >&2
    exit 2
    ;;
esac

if [[ ! "$tag" =~ $pattern ]]; then
  echo "invalid $kind release tag: $tag" >&2
  exit 1
fi
