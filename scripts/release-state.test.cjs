"use strict";

const assert = require("node:assert/strict");
const childProcess = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

test("release state requires a published release with its publication record", () => {
  const temporary = fs.mkdtempSync(path.join(os.tmpdir(), "ppl-release-state-test-"));
  const fakeGh = path.join(temporary, "gh");
  fs.writeFileSync(fakeGh, `#!/usr/bin/env bash
case "\${RELEASE_STATE}" in
  missing) echo "gh: Not Found (HTTP 404)" >&2; exit 1 ;;
  draft) echo '{"isDraft":true,"assets":["core-07.json"]}' ;;
  complete) echo '{"isDraft":false,"assets":["core-07.json"]}' ;;
  published-missing-asset) echo '{"isDraft":false,"assets":[]}' ;;
  unavailable) echo "gh: service unavailable (HTTP 502)" >&2; exit 1 ;;
esac
`, { mode: 0o755 });
  try {
    const state = (fixture) => childProcess.spawnSync("bash", [path.join(__dirname, "release-state.sh"), "episode-core-07", "core-07.json"], {
      encoding: "utf8",
      env: { ...process.env, GH_BIN: fakeGh, GH_REPO: "benvon/ppl-postcast-hosting", RELEASE_STATE: fixture },
    });
    assert.equal(state("missing").stdout.trim(), "missing");
    assert.equal(state("draft").stdout.trim(), "draft");
    assert.equal(state("complete").stdout.trim(), "complete");
    assert.equal(state("published-missing-asset").stdout.trim(), "published-missing-asset");
    const unavailable = state("unavailable");
    assert.notEqual(unavailable.status, 0);
    assert.match(unavailable.stderr, /Could not inspect/);
  } finally {
    fs.rmSync(temporary, { recursive: true, force: true });
  }
});
