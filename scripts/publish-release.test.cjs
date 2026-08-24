"use strict";

const assert = require("node:assert/strict");
const childProcess = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const commit = "0123456789abcdef0123456789abcdef01234567";

test("release publisher removes an orphan tag and verifies the recreated release", () => {
  const temporary = fs.mkdtempSync(path.join(os.tmpdir(), "ppl-publish-release-test-"));
  const fakeGh = path.join(temporary, "gh");
  const stateFile = path.join(temporary, "state");
  const logFile = path.join(temporary, "calls");
  const record = path.join(temporary, "core-07.json");
  fs.writeFileSync(stateFile, "orphan\n");
  fs.writeFileSync(record, `${JSON.stringify({ schema_version: 1, source_commit: commit, episode: { id: "core-07" } })}\n`);
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
      echo '{"draft":false,"prerelease":false,"tag_name":"episode-core-07","assets":[{"name":"core-07.json","id":1}]}'
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
      "episode-core-07",
      record,
      "core-07",
      commit,
    ], {
      encoding: "utf8",
      env: {
        ...process.env,
        GH_BIN: fakeGh,
        GH_REPO: "benvon/ppl-postcast-hosting",
        STATE_FILE: stateFile,
        CALL_LOG: logFile,
        RECORD_FILE: record,
      },
    });
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Removing incomplete release tag/);
    assert.match(result.stdout, /Verified immutable GitHub release record/);
    const calls = fs.readFileSync(logFile, "utf8");
    assert.match(calls, /--method DELETE repos\/benvon\/ppl-postcast-hosting\/git\/refs\/tags\/episode-core-07/);
    assert.match(calls, /release create episode-core-07/);
    assert.ok(calls.indexOf("--method DELETE") < calls.indexOf("release create"), calls);
  } finally {
    fs.rmSync(temporary, { recursive: true, force: true });
  }
});
