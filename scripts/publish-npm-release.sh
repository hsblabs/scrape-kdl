#!/usr/bin/env bash
set -euo pipefail

version="${1:?npm version without a leading v is required}"
artifact_dir="${2:?npm artifact directory is required}"
selection="${3:-all}"
root="$(cd "$(dirname "$0")/.." && pwd)"
attempts="${NPM_WAIT_ATTEMPTS:-12}"
interval="${NPM_WAIT_INTERVAL_SECONDS:-5}"

case "$selection" in
  all|core|playwright) ;;
  *)
    echo "invalid npm release selection: $selection (want all, core, or playwright)" >&2
    exit 2
    ;;
esac

"$root/scripts/validate-public-release-tag.sh" core "v$version"
if [[ ! "$attempts" =~ ^[1-9][0-9]*$ ]]; then
  echo "invalid NPM_WAIT_ATTEMPTS: $attempts" >&2
  exit 2
fi
if [[ ! "$interval" =~ ^[0-9]+$ ]]; then
  echo "invalid NPM_WAIT_INTERVAL_SECONDS: $interval" >&2
  exit 2
fi

npm_version="$(npm --version)"
IFS=. read -r npm_major npm_minor npm_patch <<<"$npm_version"
if [[ ! "$npm_major" =~ ^[0-9]+$ || ! "$npm_minor" =~ ^[0-9]+$ || ! "$npm_patch" =~ ^[0-9]+$ ]]; then
  echo "invalid npm version: $npm_version" >&2
  exit 1
fi
if (( npm_major < 11 || (npm_major == 11 && npm_minor < 5) || (npm_major == 11 && npm_minor == 5 && npm_patch < 1) )); then
  echo "npm $npm_version does not support trusted publishing; require >=11.5.1" >&2
  exit 1
fi

dist_tag=latest
if [[ "$version" == *-* ]]; then
  dist_tag=next
fi

wait_for_npm_value() {
  local package="$1"
  local field="$2"
  local expected="$3"
  local actual=""
  for ((attempt = 1; attempt <= attempts; attempt++)); do
    actual="$(npm view "$package" "$field" 2>/dev/null || true)"
    if [[ "$actual" == "$expected" ]]; then
      return 0
    fi
    if (( attempt < attempts )); then
      sleep "$interval"
    fi
  done
  echo "timed out waiting for npm $package $field=$expected (last=$actual)" >&2
  return 1
}

publish_package() {
  local package="$1"
  local archive="$2"
  local expected_integrity
  local published_integrity
  local current_tag

  if [[ ! -f "$archive" ]]; then
    echo "missing inspected npm archive: $archive" >&2
    return 1
  fi
  expected_integrity="$(node "$root/scripts/npm-archive-integrity.mjs" "$archive")"

  if npm view "$package@$version" version >/dev/null 2>&1; then
    echo "$package@$version is already published"
  else
    npm publish "$archive" --tag "$dist_tag"
  fi

  wait_for_npm_value "$package@$version" version "$version"
  published_integrity="$(npm view "$package@$version" dist.integrity)"
  if [[ "$published_integrity" != "$expected_integrity" ]]; then
    echo "npm integrity mismatch for $package@$version" >&2
    return 1
  fi

  current_tag="$(npm view "$package" "dist-tags.$dist_tag" 2>/dev/null || true)"
  if [[ -z "$current_tag" ]]; then
    echo "npm $package@$version exists without the expected $dist_tag dist-tag; trusted publishing cannot repair registry metadata" >&2
    return 1
  elif [[ "$current_tag" != "$version" ]]; then
    echo "refusing to move npm $package $dist_tag from $current_tag to $version" >&2
    return 1
  fi
  wait_for_npm_value "$package" "dist-tags.$dist_tag" "$version"
}

if [[ "$selection" == "all" || "$selection" == "core" ]]; then
  publish_package \
    @hsblabs/scrape-kdl \
    "$artifact_dir/hsblabs-scrape-kdl-$version.tgz"
fi
if [[ "$selection" == "all" || "$selection" == "playwright" ]]; then
  publish_package \
    @hsblabs/scrape-kdl-playwright \
    "$artifact_dir/hsblabs-scrape-kdl-playwright-$version.tgz"
fi
