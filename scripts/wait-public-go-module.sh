#!/usr/bin/env bash
set -euo pipefail

module="${1:?module path is required}"
version="${2:?module version is required}"
attempts="${GO_MODULE_WAIT_ATTEMPTS:-30}"
interval="${GO_MODULE_WAIT_INTERVAL_SECONDS:-10}"

if [[ ! "$attempts" =~ ^[1-9][0-9]*$ ]]; then
  echo "invalid GO_MODULE_WAIT_ATTEMPTS: $attempts" >&2
  exit 2
fi
if [[ ! "$interval" =~ ^[0-9]+$ ]]; then
  echo "invalid GO_MODULE_WAIT_INTERVAL_SECONDS: $interval" >&2
  exit 2
fi

last=""
for ((attempt = 1; attempt <= attempts; attempt++)); do
  if last="$(GOWORK=off GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org go list -m -json "$module@$version" 2>&1)"; then
    printf '%s\n' "$last"
    exit 0
  fi
  if (( attempt < attempts )); then
    sleep "$interval"
  fi
done

echo "timed out waiting for $module@$version through the public Go proxy" >&2
printf '%s\n' "$last" >&2
exit 1
