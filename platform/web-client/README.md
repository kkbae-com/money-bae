# platform/web-client

CDK app deploying `clients/app` (the SPA) as a static site: an S3 bucket
behind CloudFront, aliased to `https://money.kkbae.com` via a
`platform/dns`-issued Route 53 zone and its own ACM certificate. Deployed
once today, as `MoneyBaeWebClient-Dev` — see the "hardcoded to dev" note
at the top of `lib/web-client-stack.ts` before ever deploying a second
environment.

## Deploy

There's no CI/CD for this stack — every deploy is a manual, local step
against live infrastructure:

```bash
npm install
npm run deploy
```

`deploy` is `npm run build:client && cdk deploy`, where `build:client`
runs `clients/app`'s `build:dev` (production build, `--mode dev`, so it
picks up `clients/app/.env.dev`'s `VITE_MSAL_CLIENT_ID`/`VITE_API_SCOPE`/
`VITE_REDIRECT_URI`/`VITE_API_BASE_URL`). The stack's `BucketDeployment`
then syncs `clients/app/dist/` into the S3 bucket and invalidates
CloudFront for every path (`/*`), so a deploy always fully replaces the
previously-shipped build — there's no gradual rollout.

Requires AWS credentials for the account this stack lives in (`aws sts
get-caller-identity` to check; `aws login` if the session has expired).

**Before deploying, run `npx cdk diff` first.** This stack owns
DNS-adjacent resources (an ACM certificate, two Route 53 alias records)
that a stale local checkout can blow away: if the checked-out
`web-client-stack.ts` doesn't match what's actually deployed (e.g. an
in-flight PR added the custom domain and hasn't merged to `main` yet),
`cdk diff`/`cdk deploy` will show/perform a destructive replacement of
those resources rather than just updating the site's content. Only
proceed once the diff shows content-only changes (a
`Custom::CDKBucketDeployment` `SourceObjectKeys` change is normal and
expected on every deploy).

## Outputs

`DistributionDomainName` (the `*.cloudfront.net` fallback domain) and
`SiteUrl` (`https://money.kkbae.com`) — both serve the same content.
