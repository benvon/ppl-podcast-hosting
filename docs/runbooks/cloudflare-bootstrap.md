# Cloudflare bootstrap and handoff

## Provisioned resources

The following resources were created in Cloudflare on 2026-08-11:

| Resource | Name | State |
| --- | --- | --- |
| R2 staging bucket | `pplstudyguide-staging` | Private; Standard storage |
| R2 media bucket | `pplstudyguide-media` | Standard storage; no public domain yet |
| R2 lifecycle | `expire-unreleased-audio` | Expires `staging/` objects after 14 days; aborts incomplete multipart uploads after 7 days |
| Pages project | `pplstudyguide` | Initial empty-site deployment is live at `https://pplstudyguide.pages.dev` |

The initial site exposes a cache-revalidated empty RSS document at
`https://pplstudyguide.pages.dev/feed.xml`. It contains no podcast episodes.

## One-time hostname attachment

Attach these in the Cloudflare dashboard. Both names are in the
`pplstudyguide.com` zone.

1. **Pages apex:** Workers & Pages → `pplstudyguide` → Custom domains → Set up
   a domain → `pplstudyguide.com`.
2. **R2 media:** R2 → `pplstudyguide-media` → Settings → Custom Domains → Add →
   `media.pplstudyguide.com`.
3. Confirm the R2 bucket's `r2.dev` public-development URL remains disabled.
4. In a private browser window, verify:

   ```text
   https://pplstudyguide.com/
   https://pplstudyguide.com/feed.xml
   https://media.pplstudyguide.com/does-not-exist
   ```

The expected final request/response behavior is a `200` homepage and feed, and
a `404` for the nonexistent media object. No R2 object should be publicly
available before the GitHub release workflow promotes it.

## R2 API credentials

Create distinct S3-compatible R2 API credentials in the Cloudflare dashboard.
Do not reuse the OAuth login or a dashboard-wide API token.

| Credential | Bucket access | Consumer |
| --- | --- | --- |
| `pplstudyguide-stager` | Object read/write for `pplstudyguide-staging` only | Local `scripts/stage-episode` |
| `pplstudyguide-publisher` | Object read for `pplstudyguide-staging`; object read/write for `pplstudyguide-media` | GitHub Actions |

The staging script issues `HeadObject` before upload so it can refuse an
overwrite; it therefore needs read access in addition to write access. Neither
credential needs bucket-administration permission.

## GitHub Actions secrets

Set these repository secrets before pushing the publisher workflow to `main`.
This avoids a release workflow failure caused by unset credentials.

| Secret | Value |
| --- | --- |
| `R2_ENDPOINT` | `https://58e07a6311e7106e485f9271f7ae1e14.r2.cloudflarestorage.com` |
| `R2_STAGING_BUCKET` | `pplstudyguide-staging` |
| `R2_PUBLIC_BUCKET` | `pplstudyguide-media` |
| `R2_PUBLISH_ACCESS_KEY_ID` | Access-key ID for `pplstudyguide-publisher` |
| `R2_PUBLISH_SECRET_ACCESS_KEY` | Secret-access key for `pplstudyguide-publisher` |
| `CLOUDFLARE_ACCOUNT_ID` | `58e07a6311e7106e485f9271f7ae1e14` |
| `CLOUDFLARE_PAGES_API_TOKEN` | A narrowly scoped Cloudflare token that can deploy the `pplstudyguide` Pages project |

Store the staging credential only in the workstation secret manager or shell
environment. It must never be added as a GitHub secret or committed to this
repository.

## First directory-ready release

Before staging audio, update `config/show.yaml` with the real square cover-art
URL, owner name, and owner email. The release validator refuses to publish an
episode without them. Then run the local staging script from a clean release
branch, review the generated manifest, and merge the PR into `main`.
