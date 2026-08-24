#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 4 ]]; then
  echo "usage: publish-release.sh RELEASE_TAG RECORD_FILE EPISODE_ID PUBLISHED_COMMIT" >&2
  exit 64
fi

tag=$1
record_file=$2
episode_id=$3
published_commit=$4
gh_bin=${GH_BIN:-gh}
repo=${GH_REPO:-${GITHUB_REPOSITORY:-}}
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
asset=$(basename -- "$record_file")

if [[ -z "$repo" ]]; then
  echo "GH_REPO or GITHUB_REPOSITORY is required" >&2
  exit 64
fi
if [[ ! "$published_commit" =~ ^[a-f0-9]{40}$ ]]; then
  echo "published commit must be a full Git commit SHA" >&2
  exit 64
fi
if ! jq -e --arg episode "$episode_id" --arg commit "$published_commit" \
  '.schema_version == 1 and .episode.id == $episode and .source_commit == $commit' \
  "$record_file" >/dev/null; then
  echo "publication record does not identify the requested episode and commit" >&2
  exit 64
fi

release_state() {
  GH_BIN="$gh_bin" GH_REPO="$repo" bash "$script_dir/release-state.sh" "$tag" "$asset" "$episode_id" "$record_file"
}

delete_tag_if_present() {
  local ref
  if ref=$("$gh_bin" api "repos/${repo}/git/ref/tags/${tag}" 2>&1); then
    echo "Removing incomplete release tag ${tag} before recreating it."
    "$gh_bin" api --method DELETE "repos/${repo}/git/refs/tags/${tag}" >/dev/null
  elif [[ "$ref" != *"HTTP 404"* ]]; then
    echo "Could not inspect Git tag ${tag}: ${ref}" >&2
    return 1
  fi
  if ref=$("$gh_bin" api "repos/${repo}/git/ref/tags/${tag}" 2>&1); then
    echo "Git tag ${tag} still exists after cleanup" >&2
    return 1
  elif [[ "$ref" != *"HTTP 404"* ]]; then
    echo "Could not verify cleanup of Git tag ${tag}: ${ref}" >&2
    return 1
  fi
}

state=$(release_state)
case "$state" in
  complete)
    echo "GitHub release record already exists for ${tag} and matches the attested record."
    exit 0
    ;;
  draft)
    echo "Removing incomplete draft ${tag} before retrying its release record."
    "$gh_bin" release delete "$tag" --yes
    delete_tag_if_present
    ;;
  orphan-tag)
    delete_tag_if_present
    ;;
  missing) ;;
  published-missing-asset|published-invalid|published-record-mismatch)
    echo "Published GitHub Release ${tag} is not a valid immutable release record (${state}); refusing to overwrite it." >&2
    exit 1
    ;;
  *)
    echo "Unknown release state for ${tag}: ${state}" >&2
    exit 1
    ;;
esac

"$gh_bin" release create "$tag" "$record_file" \
  --target "$published_commit" \
  --title "Published ${episode_id}" \
  --notes "Machine-readable publication record for ${episode_id}. Its SHA-256 is independently attested by GitHub Actions."

state=$(release_state)
if [[ "$state" != "complete" ]]; then
  echo "GitHub Release ${tag} did not close in a valid state after creation (${state})" >&2
  exit 1
fi
echo "Verified immutable GitHub release record ${tag}."
