#!/usr/bin/env bash
set -euo pipefail

# A GitHub Release is complete only when it is published, its tag resolves to
# the commit named by the publication record, and that record identifies the
# expected episode. A draft and an orphaned tag are intentionally retryable.
if [[ $# -lt 3 || $# -gt 4 ]]; then
  echo "usage: release-state.sh RELEASE_KEY CONTENT_VERSION EPISODE_ID [EXPECTED_RECORD]" >&2
  exit 64
fi

release_key=$1
content_version=$2
episode_id=$3
expected_record=${4:-}
tag="${release_key}/v${content_version}"
asset="${release_key}-v${content_version}.json"
gh_bin=${GH_BIN:-gh}
repo=${GH_REPO:-${GITHUB_REPOSITORY:-}}
if [[ -z "$repo" ]]; then
  echo "GH_REPO or GITHUB_REPOSITORY is required" >&2
  exit 64
fi

if [[ ! "$release_key" =~ ^(episode|supplement|rough-spot)-[0-9]{2,3}$ || ! "$content_version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ || ! "$episode_id" =~ ^[a-z0-9][a-z0-9-]{1,62}$ ]]; then
  echo "release key, content version, or episode id has an invalid format" >&2
  exit 64
fi
if [[ -n "$expected_record" && ! -f "$expected_record" ]]; then
  echo "expected publication record does not exist: ${expected_record}" >&2
  exit 64
fi

tag_commit() {
  local ref object_type object_sha tag_object depth
  if ! ref=$("$gh_bin" api "repos/${repo}/git/ref/tags/${tag}" 2>&1); then
    if [[ "$ref" == *"HTTP 404"* ]]; then
      return 2
    fi
    echo "Could not inspect Git tag ${tag}: ${ref}" >&2
    return 1
  fi
  if ! jq -e '.object.type and .object.sha' >/dev/null 2>&1 <<<"$ref"; then
    echo "Git tag ${tag} returned invalid JSON" >&2
    return 1
  fi
  object_type=$(jq -r '.object.type' <<<"$ref")
  object_sha=$(jq -r '.object.sha' <<<"$ref")
  for depth in 1 2 3 4 5; do
    case "$object_type" in
      commit)
        [[ "$object_sha" =~ ^[a-f0-9]{40}$ ]] || { echo "Git tag ${tag} resolves to an invalid commit" >&2; return 1; }
        printf '%s\n' "$object_sha"
        return 0
        ;;
      tag)
        if ! tag_object=$("$gh_bin" api "repos/${repo}/git/tags/${object_sha}" 2>&1); then
          echo "Could not resolve annotated Git tag ${tag}: ${tag_object}" >&2
          return 1
        fi
        if ! jq -e '.object.type and .object.sha' >/dev/null 2>&1 <<<"$tag_object"; then
          echo "Annotated Git tag ${tag} returned invalid JSON" >&2
          return 1
        fi
        object_type=$(jq -r '.object.type' <<<"$tag_object")
        object_sha=$(jq -r '.object.sha' <<<"$tag_object")
        ;;
      *)
        echo "Git tag ${tag} resolves to unsupported object type ${object_type}" >&2
        return 1
        ;;
    esac
  done
  echo "Git tag ${tag} exceeds the supported annotation depth" >&2
  return 1
}

if ! release=$("$gh_bin" api "repos/${repo}/releases/tags/${tag}" 2>&1); then
  if [[ "$release" == *"HTTP 404"* ]]; then
    if resolved_commit=$(tag_commit); then
      echo "orphan-tag"
    else
      tag_status=$?
      if [[ $tag_status -eq 2 ]]; then
        echo "missing"
      else
        exit "$tag_status"
      fi
    fi
    exit 0
  fi
  echo "Could not inspect GitHub Release ${tag}: ${release}" >&2
  exit 1
fi

if ! jq -e . >/dev/null 2>&1 <<<"$release"; then
  echo "GitHub Release ${tag} returned invalid JSON" >&2
  exit 1
fi

if jq -e '.draft == true' >/dev/null <<<"$release"; then
  echo "draft"
  exit 0
fi
if ! jq -e '.draft == false and .prerelease == false and .tag_name == $tag' --arg tag "$tag" >/dev/null <<<"$release"; then
  echo "published-invalid"
  exit 0
fi
asset_id=$(jq -er '.assets[] | select(.name == $asset) | .id' --arg asset "$asset" <<<"$release" | head -n 1 || true)
if [[ -z "$asset_id" ]]; then
  echo "published-missing-asset"
  exit 0
fi

if ! resolved_commit=$(tag_commit); then
  echo "published-invalid"
  exit 0
fi
record_file=$(mktemp)
trap 'rm -f "$record_file"' EXIT
if ! "$gh_bin" api -H 'Accept: application/octet-stream' "repos/${repo}/releases/assets/${asset_id}" >"$record_file"; then
  echo "Could not download publication record ${asset} from ${tag}" >&2
  exit 1
fi
if ! jq -e --arg episode "$episode_id" --arg key "$release_key" --arg version "$content_version" --arg tag "$tag" --arg commit "$resolved_commit" \
  '.schema_version == 1 and .episode.id == $episode and .episode.release_key == $key and .episode.content_version == $version and .release_tag == $tag and .source_commit == $commit' \
  "$record_file" >/dev/null; then
  echo "published-invalid"
  exit 0
fi
if ! "$gh_bin" attestation verify "$record_file" \
  --repo "$repo" \
  --signer-workflow "${repo}/.github/workflows/publish.yml" \
  --source-ref "refs/heads/main" \
  --source-digest "$resolved_commit" >/dev/null 2>&1; then
  echo "published-invalid"
  exit 0
fi
if [[ -n "$expected_record" ]] && ! cmp -s "$expected_record" "$record_file"; then
  echo "published-record-mismatch"
  exit 0
fi
echo "complete"
