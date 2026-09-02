"use strict";

const assert = require("node:assert/strict");
const childProcess = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const commit = "0123456789abcdef0123456789abcdef01234567";
const releaseKey = "episode-07";
const contentVersion = "0.1.4";
const tag = `${releaseKey}/v${contentVersion}`;
const asset = `${releaseKey}-v${contentVersion}.json`;

test("release state closes only when the tag, episode, commit, and record agree", () => {
  const temporary = fs.mkdtempSync(path.join(os.tmpdir(), "ppl-release-state-test-"));
  const fakeGh = path.join(temporary, "gh");
  const expectedRecord = path.join(temporary, asset);
  fs.writeFileSync(expectedRecord, `${JSON.stringify({ schema_version: 1, source_commit: commit, release_tag: tag, episode: { id: "core-07", release_key: releaseKey, content_version: contentVersion } })}\n`);
  fs.writeFileSync(fakeGh, `#!/usr/bin/env bash
set -euo pipefail
endpoint="\${!#}"
if [[ "$1 $2" == "attestation verify" ]]; then
  if [[ "\${RELEASE_STATE}" == "legacy-attestation" ]]; then
    [[ " $* " == *" --repo benvon/ppl-postcast-hosting "* ]] && exit 0
    exit 1
  fi
  if [[ "\${RELEASE_STATE}" == "unattested" ]]; then exit 1; fi
  exit 0
fi
if [[ "$1" != "api" ]]; then exit 90; fi
case "$endpoint" in
  repos/*/releases/tags/*)
    case "\${RELEASE_STATE}" in
      missing|orphan-tag) echo "gh: Not Found (HTTP 404)" >&2; exit 1 ;;
      unavailable) echo "gh: service unavailable (HTTP 502)" >&2; exit 1 ;;
      draft) echo '{"draft":true,"prerelease":false,"tag_name":"episode-07/v0.1.4","assets":[]}' ;;
      published-missing-asset) echo '{"draft":false,"prerelease":false,"tag_name":"episode-07/v0.1.4","assets":[]}' ;;
      prerelease) echo '{"draft":false,"prerelease":true,"tag_name":"episode-07/v0.1.4","assets":[{"name":"episode-07-v0.1.4.json","id":1}]}' ;;
      *) echo '{"draft":false,"prerelease":false,"tag_name":"episode-07/v0.1.4","assets":[{"name":"episode-07-v0.1.4.json","id":1}]}' ;;
    esac
    ;;
  repos/*/git/ref/tags/*)
    if [[ "\${RELEASE_STATE}" == "missing" ]]; then echo "gh: Not Found (HTTP 404)" >&2; exit 1; fi
    printf '{"object":{"type":"commit","sha":"%s"}}\\n' "\${TAG_COMMIT}"
    ;;
  repos/*/releases/assets/*)
    record_commit="\${TAG_COMMIT}"
    record_episode="core-07"
    [[ "\${RELEASE_STATE}" == "invalid-record" ]] && record_commit="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    [[ "\${RELEASE_STATE}" == "wrong-episode" ]] && record_episode="core-08"
    printf '{"schema_version":1,"source_commit":"%s","release_tag":"episode-07/v0.1.4","episode":{"id":"%s","release_key":"episode-07","content_version":"0.1.4"}}\\n' "$record_commit" "$record_episode"
    ;;
  *) exit 91 ;;
esac
`, { mode: 0o755 });
  try {
    const state = (fixture, includeExpectedRecord = false, legacyRepo = "") => childProcess.spawnSync("bash", [
      path.join(__dirname, "release-state.sh"),
      releaseKey,
      contentVersion,
      "core-07",
      ...(includeExpectedRecord ? [expectedRecord] : []),
    ], {
      encoding: "utf8",
      env: {
        ...process.env,
        GH_BIN: fakeGh,
        GH_REPO: "benvon/ppl-podcast-hosting",
        ...(legacyRepo ? { GH_LEGACY_REPO: legacyRepo } : {}),
        RELEASE_STATE: fixture,
        TAG_COMMIT: commit,
      },
    });
    const expectState = (fixture, expected, includeExpectedRecord = false) => {
      const result = state(fixture, includeExpectedRecord);
      assert.equal(result.status, 0, result.stderr);
      assert.equal(result.stdout.trim(), expected, result.stderr);
    };
    expectState("missing", "missing");
    expectState("orphan-tag", "orphan-tag");
    expectState("draft", "draft");
    expectState("complete", "complete");
    expectState("complete", "complete", true);
    expectState("published-missing-asset", "published-missing-asset");
    expectState("prerelease", "published-invalid");
    expectState("invalid-record", "published-invalid");
    expectState("wrong-episode", "published-invalid");
    expectState("unattested", "published-invalid");
    expectState("legacy-attestation", "published-invalid");
    const legacyAttestation = state("legacy-attestation", false, "benvon/ppl-postcast-hosting");
    assert.equal(legacyAttestation.status, 0, legacyAttestation.stderr);
    assert.equal(legacyAttestation.stdout.trim(), "complete", legacyAttestation.stderr);

    const malformedVersion = childProcess.spawnSync("bash", [
      path.join(__dirname, "release-state.sh"),
      releaseKey,
      "1.0.0-01",
      "core-07",
    ], { encoding: "utf8", env: { ...process.env, GH_BIN: fakeGh, GH_REPO: "benvon/ppl-podcast-hosting" } });
    assert.equal(malformedVersion.status, 64, malformedVersion.stderr);

    const invalidLegacyRepo = state("complete", false, "other-owner/ppl-postcast-hosting");
    assert.equal(invalidLegacyRepo.status, 64, invalidLegacyRepo.stderr);
    assert.match(invalidLegacyRepo.stderr, /GH_LEGACY_REPO must name a different repository owned by benvon/);

    fs.writeFileSync(expectedRecord, `${JSON.stringify({ schema_version: 1, source_commit: commit, release_tag: tag, episode: { id: "core-07", release_key: releaseKey, content_version: contentVersion, changed: true } })}\n`);
    expectState("complete", "published-record-mismatch", true);

    const unavailable = state("unavailable");
    assert.notEqual(unavailable.status, 0);
    assert.match(unavailable.stderr, /Could not inspect/);
  } finally {
    fs.rmSync(temporary, { recursive: true, force: true });
  }
});
