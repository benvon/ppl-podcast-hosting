"use strict";

const assert = require("node:assert/strict");
const childProcess = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const stageScript = path.join(__dirname, "stage-episode");

function writeHandoff(directory) {
  fs.mkdirSync(directory, { recursive: true });
  fs.writeFileSync(path.join(directory, "episode.yaml"), "id: core-99\n");
  for (const name of ["show-notes.md", "audio.mp3", "source-release-seal.yaml"]) fs.writeFileSync(path.join(directory, name), "test\n");
}

function writeGitStub(directory) {
  const script = `#!/usr/bin/env bash
set -euo pipefail
case "$1" in
  fetch)
    [[ "$STAGE_GIT_MODE" == "fetch-fails" ]] && exit 1
    exit 0
    ;;
  rev-parse)
    if [[ "$3" == "origin/main^{commit}" ]]; then
      [[ "$STAGE_GIT_MODE" == "missing-publication-ref" ]] && exit 1
      printf '%s\\n' remote-main
      exit 0
    fi
    if [[ "$3" == "main^{commit}" ]]; then
      [[ "$STAGE_GIT_MODE" == "stale-local-main" ]] && printf '%s\\n' stale-main || printf '%s\\n' remote-main
      exit 0
    fi
    exit 1
    ;;
  merge-base)
    [[ "$STAGE_GIT_MODE" == "stale-feature-branch" ]] && exit 1
    exit 0
    ;;
  cat-file)
    [[ "$STAGE_GIT_MODE" == "already-published" ]] && exit 0
    exit 1
    ;;
esac
exit 1
`;
  const file = path.join(directory, "git");
  fs.writeFileSync(file, script, { mode: 0o755 });
}

function stageWithGitMode(mode) {
  const temporary = fs.mkdtempSync(path.join(os.tmpdir(), "ppl-stage-episode-test-"));
  try {
    const handoff = path.join(temporary, "handoff");
    const bin = path.join(temporary, "bin");
    writeHandoff(handoff); fs.mkdirSync(bin); writeGitStub(bin);
    return childProcess.spawnSync("bash", [stageScript, handoff], {
      cwd: temporary,
      encoding: "utf8",
      env: { ...process.env, PATH: `${bin}:${process.env.PATH}`, STAGE_GIT_MODE: mode },
    });
  } finally {
    fs.rmSync(temporary, { recursive: true, force: true });
  }
}

test("stage-episode fails closed when it cannot prove the current publication reference", () => {
  for (const [mode, expected] of [
    ["fetch-fails", /unable to refresh origin\/main/],
    ["missing-publication-ref", /origin\/main does not resolve/],
    ["stale-local-main", /local main is not current/],
    ["stale-feature-branch", /current branch does not include/],
  ]) {
    const result = stageWithGitMode(mode);
    assert.equal(result.status, 65, `${mode}: ${result.stderr}`);
    assert.match(result.stderr, expected);
  }
});

test("stage-episode refuses a handoff already present on the current publication branch", () => {
  const result = stageWithGitMode("already-published");
  assert.equal(result.status, 65, result.stderr);
  assert.match(result.stderr, /already present on origin\/main/);
});
