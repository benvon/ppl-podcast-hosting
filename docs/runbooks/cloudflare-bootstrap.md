# Cloudflare deployment setup

This runbook is safe to keep in the public repository. Store the actual account
identifier, bucket and project names, credential identifiers, and all secret
values in the Cloudflare dashboard, GitHub configuration, or the team's secret
manager.

## Provisioned resources

Create the following resources in Cloudflare:

| Resource | State |
| --- | --- |
| Private R2 staging bucket | Keep non-public; Standard storage |
| Public-media R2 bucket | Attach the public media custom domain |
| R2 lifecycle rule | Expire `staging/` objects after 14 days; abort incomplete multipart uploads after 7 days |
| Pages project | Attach the public site hostname |

## Public hostnames

Attach the public site and media hostnames to the appropriate Cloudflare zone:

1. **Pages site:** `pplstudyguide.com` is attached to the Pages project.
2. **R2 media:** `media.pplstudyguide.com` is attached to the public-media
   bucket.
3. The R2 bucket's `r2.dev` public-development URL remains disabled.
4. Verify:

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
| Staging credential | Object read/write for the private staging bucket only | Local `scripts/stage-episode` |
| Publishing credential | Object read for the private staging bucket; object read/write for the public-media bucket | GitHub Actions |

The staging script issues `HeadObject` before upload so it can refuse an
overwrite; it therefore needs read access in addition to write access. Neither
credential needs bucket-administration permission.

## GitHub Actions secrets

Set these repository secrets before pushing the publisher workflow to `main`.
This avoids a release workflow failure caused by unset credentials.

| Secret | Configuration value |
| --- | --- |
| `R2_ENDPOINT` | S3-compatible endpoint for the Cloudflare account |
| `R2_STAGING_BUCKET` | Private staging bucket name |
| `R2_PUBLIC_BUCKET` | Public-media bucket name |
| `R2_PUBLISH_ACCESS_KEY_ID` | Access-key ID for the publishing credential |
| `R2_PUBLISH_SECRET_ACCESS_KEY` | Secret-access key for the publishing credential |
| `CLOUDFLARE_ACCOUNT_ID` | Cloudflare account identifier |
| `CLOUDFLARE_PAGES_API_TOKEN` | Narrowly scoped Pages deployment token |

Store the staging credential only in the workstation secret manager or shell
environment. It must never be added as a GitHub secret or committed to this
repository.

## First directory-ready release

Before staging audio, verify that `config/show.yaml` has the real square
cover-art URL plus a public show identity and feedback address. The release
validator refuses to publish an episode without them. Then run the local
staging script from a clean release branch, review the generated manifest, and
merge the PR into `main`.
