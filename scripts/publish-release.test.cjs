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

test("release publisher removes an orphan tag and verifies the recreated release", () => {
  const temporary = fs.mkdtempSync(path.join(os.tmpdir(), "ppl-publish-release-test-"));
  const fakeGh = path.join(temporary, "gh");
  const stateFile = path.join(temporary, "state");
  const logFile = path.join(temporary, "calls");
  const record = path.join(temporary, asset);
  fs.writeFileSync(stateFile, "orphan\n");
  fs.writeFileSync(record, `${JSON.stringify({ schema_version: 1, source_commit: commit, release_tag: tag, episode: { id: "core-07", release_key: releaseKey, content_version: contentVersion } })}\n`);
  fs.writeFileSync(fakeGh, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\\n' "$*" >>"$CALL_LOG"
state=$(tr -d '\\n' <"$STATE_FILE")
if [[ "$1 $2" == "attestation verify" ]]; then
  exit 0
elif [[ "$1" == "api" ]]; then
  endpoint="\${!#}"
  method="GET"
  [[ "$*" == *"--method DELETE"* ]] && method="DELETE"
  case "$endpoint" in
    repos/*/releases/tags/*)
      if [[ "$state" != "complete" ]]; then echo "gh: Not Found (HTTP 404)" >&2; exit 1; fi
      echo '{"draft":false,"prerelease":false,"tag_name":"episode-07/v0.1.4","assets":[{"name":"episode-07-v0.1.4.json","id":1}]}'
      ;;
    repos/*/git/ref/tags/*|repos/*/git/refs/tags/*)
      if [[ "$method" == "DELETE" ]]; then printf 'missing\\n' >"$STATE_FILE"; exit 0; fi
      if [[ "$state" == "missing" ]]; then echo "gh: Not Found (HTTP 404)" >&2; exit 1; fi
      tag_commit="${commit}"
      if [[ "$state" == "orphan" ]]; then tag_commit="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"; fi
      printf '{"object":{"type":"commit","sha":"%s"}}\\n' "$tag_commit"
      ;;
    repos/*/releases/assets/*) sed -n '1,$p' "$RECORD_FILE" ;;
    *) exit 91 ;;
  esac
elif [[ "$1 $2" == "release create" ]]; then
  printf 'complete\\n' >"$STATE_FILE"
else
  exit 92
fi
`, { mode: 0o755 });
  try {
    const result = childProcess.spawnSync("bash", [
      path.join(__dirname, "publish-release.sh"),
      releaseKey,
      contentVersion,
      record,
      "core-07",
      commit,
    ], {
      encoding: "utf8",
      env: {
        ...process.env,
        GH_BIN: fakeGh,
        GH_REPO: "benvon/ppl-podcast-hosting",
        STATE_FILE: stateFile,
        CALL_LOG: logFile,
        RECORD_FILE: record,
      },
    });
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Removing incomplete release tag/);
    assert.match(result.stdout, /Verified immutable GitHub release record/);
    const calls = fs.readFileSync(logFile, "utf8");
    assert.match(calls, /--method DELETE repos\/benvon\/ppl-podcast-hosting\/git\/refs\/tags\/episode-07\/v0.1.4/);
    assert.match(calls, /release create episode-07\/v0.1.4/);
    assert.ok(calls.indexOf("--method DELETE") < calls.indexOf("release create"), calls);
  } finally {
    fs.rmSync(temporary, { recursive: true, force: true });
  }
});
