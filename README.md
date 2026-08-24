# PPL Study Guide publisher

This repository is the GitHub-controlled publishing plane for the public PPL
Study Guide podcast:

- `https://pplstudyguide.com/` is the homepage, episode show notes, and
  canonical RSS feed.
- `https://media.pplstudyguide.com/` serves immutable MP3 enclosure files from
  Cloudflare R2.
- A private R2 staging bucket receives local release audio before an episode
  pull request is merged.

The source material and local master audio stay outside this repository. The
release contract here records the exact staged audio object and its SHA-256;
audio is never committed to Git.

## Release lifecycle

1. Use the source repository's `release:prepare-handoff` command to assemble a
   sealed local release directory containing `episode.yaml`, `show-notes.md`,
   `audio.mp3`, and `source-release-seal.yaml`.
2. From a new branch in this repository, run `scripts/stage-episode` against
   that directory. It checks the files, writes an immutable release manifest,
   and uploads the MP3 to the non-public staging bucket.
3. Review, commit, push, and merge the release PR into `main`.
4. GitHub Actions verifies the staged object, publishes its immutable public
   copy, verifies the public audio endpoint, rebuilds the feed/site, and
   deploys the site to Cloudflare Pages.

The workflow never publishes an episode whose staged bytes do not match the
recorded SHA-256. It uploads audio before it deploys the updated feed, so a
podcast client cannot receive an enclosure URL for an unavailable file.

After every successful publication job, follow-on jobs find newly sealed
episode packages that do not yet have a GitHub Release. They create an
immutable release containing a small machine-readable publication record and
use GitHub artifact attestation to bind that record to the publishing workflow
and commit. The record includes the audio, metadata, show-notes, and
source-handoff-seal hashes. This state-based discovery recovers an episode if a
previous pending run was superseded. It is forward-only: older episodes without
the sealed-package marker are not retroactively released or attested.

Each sealed package also carries a public release key and semantic content
version. GitHub tags use the immutable, namespaced form
`<release-key>/v<content-version>`: for example, `episode-07/v0.1.4`,
`supplement-01/v0.1.0`, or `rough-spot-001/v0.1.0`. Internal production IDs
such as `core-07` never determine public tag names. The release-record asset
uses the corresponding stable name, such as `episode-07-v0.1.4.json`.

A release is considered complete only when it is published and contains its
episode publication-record JSON asset, its tag resolves to the commit named by
that record, the record identifies the expected episode, and GitHub verifies an
attestation from this repository's `publish.yml` workflow on `main` at that
exact commit. If GitHub leaves a draft or orphaned tag after a transient
asset-upload, cleanup, or publish failure, the next successful publish removes
that incomplete state and retries it. Creation ends with the same full
verification, including an exact byte comparison with the locally attested
record. An invalid published release fails closed instead of silently claiming
the episode is attested.

The workflow derives the tag and asset name from the sealed public key and
version, then verifies that those same values appear in the generated record
before it is attested or released. A mismatched key, version, tag, asset, or
record is rejected rather than being published under an ambiguous identity.

Manual recovery runs are allowed only from `main`. Publication jobs share one
non-cancelling concurrency group, so retry cleanup, deployment, and release
creation cannot overlap another publication run.

## Local release directory

```text
my-episode/
├── audio.mp3
├── episode.yaml
├── show-notes.md
└── source-release-seal.yaml
```

`source-release-seal.yaml` records the SHA-256 identity of the exact three
handoff inputs and the reviewed source package. `scripts/stage-episode` checks
the seal before it creates local metadata or sends bytes to private staging.
If any input changes, return to the source repository, rerun its release gates,
and create a new handoff directory.

The generated hosted release directory retains the seal, the original
source-facing `episode.yaml`, and the seal SHA-256 in the published contract.
Validation compares those retained inputs, the hosted show notes, and the
immutable audio identity before publication and before an attested record can
be produced. This forms the provenance link from the hosted release contract to
the later GitHub release record and its attestation.

`episode.yaml` is deliberately small and describes listener-facing facts. The
staging location, final audio key, byte count, and checksum are written by the
staging script.

```yaml
id: "001-airworthiness"
guid: "pplstudyguide.com:001-airworthiness"
title: "Airworthiness: The Documents and the Decision"
description: "How a private-pilot learner can distinguish required documents from safe decisions."
published_at: "2026-08-15T14:00:00Z"
duration: "00:34:12"
season: 1
number: 1
explicit: false
audio: {}
```

For a release with validated embedded MP3 chapters, add an optional `chapters`
list to `episode.yaml`. Each entry needs a `title` and `start_ms`; the site
renders it as a collapsible, click-to-seek chapter list. Omit `chapters` for an
episode with no markers. The chapter-generation step must supply
`chapters_audio_sha256` from the exact MP3 it used. Staging compares that value
with the supplied MP3 and rejects a chapter list for a different audio file.

Keep show notes in the listener-facing format defined by the production plan.
The generator renders Markdown without allowing raw HTML. It publishes the
episode synopsis plus every unique HTTPS link from those rendered notes in the
RSS episode description and `itunes:summary`, under **Study materials and
visual aids**. It also includes the complete rendered notes in
`content:encoded` for podcast players that support rich notes. Give each link a
short, descriptive label: it becomes the player-facing study-material label.

The hosted episode page displays the single AI-production disclosure from
`config/show.yaml` immediately below the audio download link. A legacy
`## Production notice` section in imported show notes is deliberately omitted
from the public page and RSS rich notes so listeners do not see it twice.

## Local prerequisites

- Go (managed through `mise`)
- AWS CLI v2, configured only with the limited R2 staging credentials
- `R2_ENDPOINT`, for example `https://<account-id>.r2.cloudflarestorage.com`
- `PPL_R2_STAGING_BUCKET`
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_DEFAULT_REGION=auto`

Run a local validation without making network calls:

```bash
mise run validate
mise run build
```

Stage an episode from a new branch:

```bash
scripts/stage-episode /absolute/path/to/my-episode
```

The script intentionally does not commit, push, or merge. Inspect the generated
`episodes/<id>/` directory before creating the release PR.

## Cloudflare and GitHub setup

Use separate R2 credentials:

| Credential | Scope | Where it lives |
| --- | --- | --- |
| Workstation staging credential | Object read/write only to the staging bucket | Local environment / secret manager |
| GitHub publishing credential | Read staging; read/write public-media bucket | GitHub Actions secrets |
| Pages deployment token | Cloudflare Pages edit for this project only | GitHub Actions secret |

The Cloudflare bootstrap state and the exact secret names are recorded in
[`docs/runbooks/cloudflare-bootstrap.md`](docs/runbooks/cloudflare-bootstrap.md).

The generic show cover is committed as `pplsg-cover.png`, deployed to
`https://pplstudyguide.com/pplsg-cover.png`, and referenced in the homepage and
RSS feed. The build verifies that a configured local cover is a square PNG or
JPEG between 1400 and 3000 pixels and rejects PNGs with an alpha channel.
Before the first release, set a real
`owner_name` and `owner_email` in `config/show.yaml`.

Required GitHub Actions secrets are documented in
`.github/workflows/publish.yml`. Secrets are never committed to this repository.

## Statistics

Cloudflare Pages and the R2 custom domain provide operational request and
transfer metrics. These are useful for trend monitoring but are not
IAB-certified podcast-download analytics; podcast clients may issue multiple
range requests for one listener action.
