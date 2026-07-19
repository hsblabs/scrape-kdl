#!/usr/bin/env bash
set -euo pipefail

root="${1:?repository root is required}"
output="${2:?release output directory is required}"
root_abs="$(cd "$root" && pwd -P)"

reject() {
  echo "refusing to replace unsafe release output directory: $output" >&2
  exit 1
}

case "$output" in
  /|*/|.|..|*/.|*/..) reject ;;
esac

case "$output" in
  /*) candidate="$output" ;;
  *) candidate="$root_abs/$output" ;;
esac

name="$(basename "$candidate")"
case "$name" in
  ""|.|..) reject ;;
esac

parent="$(dirname "$candidate")"
mkdir -p "$parent"
parent_abs="$(cd "$parent" && pwd -P)"
resolved="$parent_abs/$name"

if [[ "$resolved" == / || "$resolved" == "$root_abs" ]]; then
  reject
fi

printf '%s\n' "$resolved"
