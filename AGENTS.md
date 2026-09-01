# PPL Study Guide hosting-agent guidance

## Credentials and staging

- `.envrc` is a credential-bearing local file. Never read, print, search, source, diff, or otherwise inspect it. Never use `env`, `printenv`, or `direnv export` in this repository.
- The only allowed way to use the workstation R2 staging credentials is the purpose-built command `direnv exec . ./scripts/stage-episode <absolute-handoff-path>`. It supplies credentials only to the staging child process; do not copy, print, or pass them to another process.
- Use a sealed handoff directory made by the source repository's `release:prepare-handoff` command. `scripts/stage-episode` verifies the seal, creates the release manifest, and uploads the exact MP3 to the private staging bucket. It intentionally does not commit, push, or merge.
- For all review, use the resulting `episodes/<id>/` manifest, the source seal, and repository validation. Do not use environment configuration as evidence.

## Release workflow

- Start from a clean, current `main` and use a fresh `feature/` branch.
- Before opening a release PR, run the repository tests, sealed-package validation, and static-site build using the repository's documented `mise` tasks. PR validation must remain credential-free.
- Commit only the generated episode contract and related reviewed changes; audio stays outside Git. A merge to `main` performs public publication, deployment, and release attestation.
