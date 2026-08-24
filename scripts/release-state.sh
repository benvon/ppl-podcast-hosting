#!/usr/bin/env bash
set -euo pipefail

# A GitHub Release is complete only when it is published and includes the
# attested publication-record asset. A draft is intentionally retryable.
if [[ $# -ne 2 ]]; then
  echo "usage: release-state.sh RELEASE_TAG RECORD_ASSET" >&2
  exit 64
fi

tag=$1
asset=$2
gh_bin=${GH_BIN:-gh}
repo=${GH_REPO:-${GITHUB_REPOSITORY:-}}
if [[ -z "$repo" ]]; then
  echo "GH_REPO or GITHUB_REPOSITORY is required" >&2
  exit 64
fi

if ! release=$("$gh_bin" api "repos/${repo}/releases/tags/${tag}" --jq '{isDraft: .draft, assets: [.assets[].name]}' 2>&1); then
  if [[ "$release" == *"HTTP 404"* ]]; then
    echo "missing"
    exit 0
  fi
  echo "Could not inspect GitHub Release ${tag}: ${release}" >&2
  exit 1
fi

if ! jq -e . >/dev/null 2>&1 <<<"$release"; then
  echo "GitHub Release ${tag} returned invalid JSON" >&2
  exit 1
fi

if jq -e '.isDraft == false and (.assets | index($asset) != null)' --arg asset "$asset" >/dev/null <<<"$release"; then
  echo "complete"
elif jq -e '.isDraft == true' >/dev/null <<<"$release"; then
  echo "draft"
else
  echo "published-missing-asset"
fi
