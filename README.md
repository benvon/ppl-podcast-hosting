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

1. Assemble a local release directory containing `episode.yaml`,
   `show-notes.md`, and `audio.mp3`.
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

## Local release directory

```text
my-episode/
├── audio.mp3
├── episode.yaml
└── show-notes.md
```

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

Keep show notes in the listener-facing format defined by the production plan.
The generator renders Markdown without allowing raw HTML.

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
