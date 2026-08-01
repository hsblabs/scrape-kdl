#!/usr/bin/env bash
set -euo pipefail

kind="${1:?release kind is required: core or rod}"
tag="${2:?release tag is required}"
source_revision="${3:?source revision is required}"
artifact_dir="${4:?artifact directory is required}"
root="$(cd "$(dirname "$0")/.." && pwd)"

"$root/scripts/validate-public-release-tag.sh" "$kind" "$tag"
source_commit="$(git -C "$root" rev-parse "$source_revision^{commit}")"
if [[ ! -d "$artifact_dir" ]]; then
  echo "missing release artifact directory: $artifact_dir" >&2
  exit 1
fi
shopt -s nullglob
artifacts=("$artifact_dir"/*)
if (( ${#artifacts[@]} == 0 )); then
  echo "release artifact directory is empty: $artifact_dir" >&2
  exit 1
fi
for artifact in "${artifacts[@]}"; do
  if [[ ! -f "$artifact" ]]; then
    echo "release artifacts must be regular files: $artifact" >&2
    exit 1
  fi
done

verify_local_tag() {
  local object_type
  local peeled
  object_type="$(git -C "$root" cat-file -t "refs/tags/$tag")"
  if [[ "$object_type" != tag ]]; then
    echo "release tag is not annotated: $tag" >&2
    return 1
  fi
  peeled="$(git -C "$root" rev-parse "$tag^{}")"
  if [[ "$peeled" != "$source_commit" ]]; then
    echo "release tag $tag points to $peeled; want $source_commit" >&2
    return 1
  fi
}

remote_refs="$(git -C "$root" ls-remote --tags origin "refs/tags/$tag" "refs/tags/$tag^{}")"
if [[ -n "$remote_refs" ]]; then
  remote_peeled="$(printf '%s\n' "$remote_refs" | awk -v ref="refs/tags/$tag^{}" '$2 == ref { print $1 }')"
  if [[ -z "$remote_peeled" ]]; then
    echo "remote release tag is not annotated: $tag" >&2
    exit 1
  fi
  if [[ "$remote_peeled" != "$source_commit" ]]; then
    echo "remote release tag $tag points to $remote_peeled; want $source_commit" >&2
    exit 1
  fi
  if ! git -C "$root" show-ref --verify --quiet "refs/tags/$tag"; then
    git -C "$root" fetch origin "refs/tags/$tag:refs/tags/$tag"
  fi
  verify_local_tag
else
  if git -C "$root" show-ref --verify --quiet "refs/tags/$tag"; then
    verify_local_tag
  else
    git -C "$root" tag -a "$tag" "$source_commit" -m "Release $tag"
  fi
  git -C "$root" push origin "refs/tags/$tag"
fi

expected_prerelease=false
if [[ "$tag" == *-* ]]; then
  expected_prerelease=true
fi

if ! gh release view "$tag" >/dev/null 2>&1; then
  if [[ "$expected_prerelease" == true ]]; then
    gh release create "$tag" "${artifacts[@]}" \
      --verify-tag \
      --generate-notes \
      --title "$tag" \
      --prerelease \
      --latest=false
  else
    gh release create "$tag" "${artifacts[@]}" \
      --verify-tag \
      --generate-notes \
      --title "$tag"
  fi
fi

is_draft="$(gh release view "$tag" --json isDraft --jq .isDraft)"
is_prerelease="$(gh release view "$tag" --json isPrerelease --jq .isPrerelease)"
if [[ "$is_draft" != false || "$is_prerelease" != "$expected_prerelease" ]]; then
  echo "GitHub Release metadata does not match $tag" >&2
  exit 1
fi

remote_asset_file="$(mktemp)"
trap 'rm -f "$remote_asset_file"' EXIT
gh release view "$tag" --json assets --jq '.assets[].name' >"$remote_asset_file"

while IFS= read -r remote_asset; do
  if [[ -n "$remote_asset" && ! -f "$artifact_dir/$remote_asset" ]]; then
    echo "GitHub Release $tag has unexpected asset: $remote_asset" >&2
    exit 1
  fi
done <"$remote_asset_file"

for artifact in "${artifacts[@]}"; do
  name="${artifact##*/}"
  if ! grep -Fxq "$name" "$remote_asset_file"; then
    gh release upload "$tag" "$artifact"
    printf '%s\n' "$name" >>"$remote_asset_file"
  fi
  download_dir="$(mktemp -d)"
  if ! gh release download "$tag" --pattern "$name" --dir "$download_dir"; then
    rm -rf "$download_dir"
    exit 1
  fi
  if ! cmp -s "$artifact" "$download_dir/$name"; then
    rm -rf "$download_dir"
    echo "GitHub Release $tag asset differs from inspected artifact: $name" >&2
    exit 1
  fi
  rm -rf "$download_dir"
done
