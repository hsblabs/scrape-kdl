#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
mapfile -t files < <(find . -name '*.go' -type f -not -path './.git/*' | sort)
if ((${#files[@]} == 0)); then
  exit 0
fi
unformatted="$(gofmt -l "${files[@]}")"
if [[ -n "$unformatted" ]]; then
  printf 'gofmt required:
%s
' "$unformatted" >&2
  exit 1
fi
