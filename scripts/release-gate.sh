#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

make release-check
make test-rod
make test-rod-e2e
make test-playwright-e2e
