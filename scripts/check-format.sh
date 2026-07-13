#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
files=()
while IFS= read -r file; do
  files+=("$file")
done < <(find . -name '*.go' -type f -not -path './.git/*' | LC_ALL=C sort)
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
