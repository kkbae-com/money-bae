# Platform DNS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `kkbae.com` real Route 53 DNS, put the web app at `https://money.kkbae.com`, and keep sign-in working there.

**Architecture:** A new minimal `platform/dns` CDK app owns just the `kkbae.com` Route 53 hosted zone. `platform/web-client` imports that zone by ID, creates its own `us-east-1` ACM certificate for `money.kkbae.com`, attaches it to the existing CloudFront distribution, and adds the Route 53 alias records. `platform/entra-external-id`'s SPA app registration gets `https://money.kkbae.com/` added to its redirect URIs alongside the existing CloudFront one.

**Tech Stack:** AWS CDK (TypeScript) for `platform/dns` and `platform/web-client`; Terraform (`azuread` provider) for `platform/entra-external-id`.

**Spec:** `docs/superpowers/specs/2026-09-07-platform-dns-design.md`

## Global Constraints

- ACM certificates used by CloudFront must be created in `us-east-1` — non-negotiable AWS requirement, not a preference.
- No dedicated unit tests for any of this — it's thin, declarative CDK/Terraform composition over trusted constructs and providers. Verification is `cdk synth` / `terraform plan` output plus real deploy checks (`dig`, `curl`, a live sign-in run-through).
- The Entra `dev_redirect_uri` change must **add** `https://money.kkbae.com/` — the existing CloudFront redirect URI is kept, never removed.
- `platform/dns` owns only the hosted zone — no certificates, no subdomain records. Each consuming stack (here, `web-client`) owns its own subdomain's certificate and Route 53 records.
- Every `cdk deploy` / `terraform apply` step in this plan is real, live infrastructure with real cost and blast radius. Get explicit user confirmation immediately before running any of them — do not run on your own judgment just because a prior step succeeded.
- The nameserver cutover at the domain registrar (Task 3) is a manual action outside the repo and outside AWS — it cannot be automated or run by an agent.

---

## File Structure

- `platform/dns/package.json`, `cdk.json`, `tsconfig.json`, `.gitignore` — new CDK app scaffold, mirrors `platform/web-client`'s.
- `platform/dns/bin/dns.ts` — CDK app entrypoint, instantiates `DnsStack` in `us-east-1`.
- `platform/dns/lib/dns-stack.ts` — the `HostedZone` for `kkbae.com` plus its outputs.
- `platform/web-client/bin/web-client.ts` — modify: pin `region: 'us-east-1'` instead of ambient `CDK_DEFAULT_REGION`.
- `platform/web-client/lib/web-client-stack.ts` — modify: import the zone, create the cert, attach domain+cert to the distribution, add alias records.
- `platform/entra-external-id/app-registrations/variables.tf` — modify: `dev_redirect_uri` becomes `list(string)`.
- `platform/entra-external-id/app-registrations/main.tf` — modify: drop the now-redundant `[...]` wrapper around `var.dev_redirect_uri`.
- `platform/entra-external-id/CLAUDE.md` — modify: update the stale single-value description of `dev_redirect_uri`.

---

### Task 1: Scaffold `platform/dns` and its `HostedZone` stack

**Files:**
- Create: `platform/dns/package.json`
- Create: `platform/dns/cdk.json`
- Create: `platform/dns/tsconfig.json`
- Create: `platform/dns/.gitignore`
- Create: `platform/dns/lib/dns-stack.ts`
- Create: `platform/dns/bin/dns.ts`

**Interfaces:**
- Produces: a deployable CDK stack `MoneyBaeDns` (in `platform/dns/bin/dns.ts`) exporting CloudFormation outputs `HostedZoneId` and `NameServers` — consumed manually by Task 3 (registrar cutover) and Task 4 (`web-client-stack.ts`'s `HOSTED_ZONE_ID` constant).

- [ ] **Step 1: Create `platform/dns/package.json`**

```json
{
  "name": "money-bae-dns-infra",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "build": "tsc",
    "deploy": "cdk deploy",
    "cdk": "cdk"
  },
  "devDependencies": {
    "@types/node": "^20.11.0",
    "aws-cdk": "^2.150.0",
    "ts-node": "^10.9.2",
    "typescript": "^5.4.5"
  },
  "dependencies": {
    "aws-cdk-lib": "2.268.0",
    "constructs": "^10.3.0",
    "source-map-support": "^0.5.21"
  }
}
```

- [ ] **Step 2: Create `platform/dns/cdk.json`**

```json
{
  "app": "npx ts-node --prefer-ts-exts bin/dns.ts",
  "watch": {
    "include": ["**"],
    "exclude": ["README.md", "cdk*.json", "**/*.d.ts", "**/*.js", "node_modules", "cdk.out"]
  }
}
```

- [ ] **Step 3: Create `platform/dns/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "commonjs",
    "lib": ["ES2022"],
    "declaration": true,
    "strict": true,
    "noImplicitAny": true,
    "strictNullChecks": true,
    "noImplicitThis": true,
    "alwaysStrict": true,
    "noImplicitReturns": true,
    "inlineSourceMap": true,
    "inlineSources": true,
    "experimentalDecorators": true,
    "strictPropertyInitialization": false,
    "typeRoots": ["./node_modules/@types"]
  },
  "exclude": ["node_modules", "cdk.out"]
}
```

- [ ] **Step 4: Create `platform/dns/.gitignore`**

```
node_modules/
cdk.out/
*.js
*.d.ts
```

- [ ] **Step 5: Create `platform/dns/lib/dns-stack.ts`**

```ts
import * as cdk from 'aws-cdk-lib';
import * as route53 from 'aws-cdk-lib/aws-route53';
import { Construct } from 'constructs';

export class DnsStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    const zone = new route53.HostedZone(this, 'KkbaeZone', {
      zoneName: 'kkbae.com',
    });

    new cdk.CfnOutput(this, 'HostedZoneId', { value: zone.hostedZoneId });
    new cdk.CfnOutput(this, 'NameServers', {
      value: cdk.Fn.join(', ', zone.hostedZoneNameServers!),
    });
  }
}
```

- [ ] **Step 6: Create `platform/dns/bin/dns.ts`**

```ts
#!/usr/bin/env node
import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import { DnsStack } from '../lib/dns-stack';

const app = new cdk.App();

new DnsStack(app, 'MoneyBaeDns', {
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: 'us-east-1',
  },
});
```

- [ ] **Step 7: Install dependencies**

Run (from `platform/dns/`): `npm install`
Expected: completes without error, creates `node_modules/` and `package-lock.json`.

- [ ] **Step 8: Build**

Run: `npm run build`
Expected: `tsc` exits 0, no output.

- [ ] **Step 9: Synth and verify the hosted zone is in the template**

Run: `npx cdk synth`
Expected: synthesizes without error; the printed template includes a resource of type `AWS::Route53::HostedZone` with `Name: kkbae.com.`, and `Outputs` containing `HostedZoneId` and `NameServers`.

- [ ] **Step 10: Commit**

```bash
git add platform/dns/
git commit -m "Scaffold platform/dns: Route 53 hosted zone for kkbae.com"
```

---

### Task 2: Deploy `platform/dns`

**Files:** none (deploy action only)

**Interfaces:**
- Consumes: the built `platform/dns` app from Task 1.
- Produces: a live Route 53 hosted zone; its `HostedZoneId` and `NameServers` values, needed by Task 3 and Task 4.

- [ ] **Step 1: Get explicit user confirmation before deploying**

This creates a real, billed AWS resource (a public Route 53 hosted zone). State exactly what will run and wait for an explicit go-ahead before continuing.

- [ ] **Step 2: Deploy**

Run (from `platform/dns/`): `npx cdk deploy`
Expected: CDK prints a changeset summary, prompts for approval (approve it), then deploys and prints the stack's `Outputs`, including `HostedZoneId` and `NameServers`.

- [ ] **Step 3: Record the outputs**

Copy the exact `HostedZoneId` value and the four `NameServers` values from the deploy output — write them down (e.g. in the conversation, not in a file) for Task 3 and Task 4 to use.

---

### Task 3: Manual nameserver cutover at the registrar

**Files:** none (human action, not automatable)

- [ ] **Step 1: Hand the user the 4 nameserver values from Task 2**

- [ ] **Step 2: User updates `kkbae.com`'s nameservers at the registrar**

This is a registrar-UI action outside the repo and outside AWS — an agent cannot do this step. Wait for the user to confirm it's done.

- [ ] **Step 3: Verify propagation**

Run: `dig +short NS kkbae.com`
Expected: eventually returns the same four nameserver hostnames from Task 2's output (may take anywhere from a few minutes to ~48 hours depending on the registrar's prior TTL — re-run periodically rather than looping tightly).

- [ ] **Step 4: Do not proceed to Task 5 until Step 3 passes**

Task 5's ACM certificate validation depends on this delegation being live; deploying before it propagates will leave the certificate stuck in "Pending validation."

---

### Task 4: Attach `money.kkbae.com` to the `web-client` CloudFront distribution

**Files:**
- Modify: `platform/web-client/bin/web-client.ts`
- Modify: `platform/web-client/lib/web-client-stack.ts`

**Interfaces:**
- Consumes: `HostedZoneId` from Task 2's output.
- Produces: `WebClientStack`'s `Distribution` now serves `https://money.kkbae.com` in addition to its `*.cloudfront.net` domain.

- [ ] **Step 1: Pin the region in `platform/web-client/bin/web-client.ts`**

Change:
```ts
new WebClientStack(app, `MoneyBaeWebClient-${stackSuffix}`, {
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: process.env.CDK_DEFAULT_REGION,
  },
});
```
to:
```ts
new WebClientStack(app, `MoneyBaeWebClient-${stackSuffix}`, {
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: 'us-east-1',
  },
});
```

- [ ] **Step 2: Replace `platform/web-client/lib/web-client-stack.ts` with**

```ts
import * as path from 'path';
import * as cdk from 'aws-cdk-lib';
import * as s3 from 'aws-cdk-lib/aws-s3';
import * as cloudfront from 'aws-cdk-lib/aws-cloudfront';
import * as origins from 'aws-cdk-lib/aws-cloudfront-origins';
import * as s3deploy from 'aws-cdk-lib/aws-s3-deployment';
import * as acm from 'aws-cdk-lib/aws-certificatemanager';
import * as route53 from 'aws-cdk-lib/aws-route53';
import * as targets from 'aws-cdk-lib/aws-route53-targets';
import { Construct } from 'constructs';

const SITE_DOMAIN = 'money.kkbae.com';
// From platform/dns's `MoneyBaeDns` stack's `HostedZoneId` output (Task 2
// of docs/superpowers/plans/2026-09-07-platform-dns-implementation.md).
const HOSTED_ZONE_ID = 'HOSTED_ZONE_ID_PLACEHOLDER';

export class WebClientStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    const zone = route53.HostedZone.fromHostedZoneAttributes(this, 'Zone', {
      hostedZoneId: HOSTED_ZONE_ID,
      zoneName: 'kkbae.com',
    });

    const certificate = new acm.Certificate(this, 'SiteCertificate', {
      domainName: SITE_DOMAIN,
      validation: acm.CertificateValidation.fromDns(zone),
    });

    const bucket = new s3.Bucket(this, 'SiteBucket', {
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      autoDeleteObjects: true,
    });

    const distribution = new cloudfront.Distribution(this, 'SiteDistribution', {
      domainNames: [SITE_DOMAIN],
      certificate,
      defaultRootObject: 'index.html',
      defaultBehavior: {
        origin: origins.S3BucketOrigin.withOriginAccessControl(bucket),
        viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
        cachePolicy: cloudfront.CachePolicy.CACHING_OPTIMIZED,
      },
      // TanStack Router does client-side routing, so any path CloudFront can't
      // find in the bucket (a deep link like /ledger, or S3 rejecting a
      // private-bucket miss as 403) needs to fall back to index.html and let
      // the SPA's router take over.
      errorResponses: [
        { httpStatus: 403, responseHttpStatus: 200, responsePagePath: '/index.html' },
        { httpStatus: 404, responseHttpStatus: 200, responsePagePath: '/index.html' },
      ],
    });

    new route53.ARecord(this, 'SiteAliasRecordV4', {
      zone,
      recordName: SITE_DOMAIN,
      target: route53.RecordTarget.fromAlias(new targets.CloudFrontTarget(distribution)),
    });
    new route53.AaaaRecord(this, 'SiteAliasRecordV6', {
      zone,
      recordName: SITE_DOMAIN,
      target: route53.RecordTarget.fromAlias(new targets.CloudFrontTarget(distribution)),
    });

    new s3deploy.BucketDeployment(this, 'SiteDeployment', {
      sources: [s3deploy.Source.asset(path.join(__dirname, '../../../clients/app/dist'))],
      destinationBucket: bucket,
      distribution,
      distributionPaths: ['/*'],
    });

    new cdk.CfnOutput(this, 'DistributionDomainName', {
      value: distribution.distributionDomainName,
    });
    new cdk.CfnOutput(this, 'SiteUrl', { value: `https://${SITE_DOMAIN}` });
  }
}
```

- [ ] **Step 3: Substitute the real hosted zone ID**

Replace `HOSTED_ZONE_ID_PLACEHOLDER` in the file above with the exact `HostedZoneId` value recorded in Task 2, Step 3.

- [ ] **Step 4: Build**

Run (from `platform/web-client/`): `npm run build`
Expected: `tsc` exits 0.

- [ ] **Step 5: Synth and verify**

Run: `npx cdk synth`
Expected: synthesizes without error. The template contains: an `AWS::CertificateManager::Certificate` resource with `DomainName: money.kkbae.com`; two `AWS::Route53::RecordSet` resources (`Type: A` and `Type: AAAA`) with `Name: money.kkbae.com.` and an `AliasTarget`; the `AWS::CloudFront::Distribution` resource's `DistributionConfig.Aliases` includes `money.kkbae.com` and `ViewerCertificate` references the new certificate.

- [ ] **Step 6: Commit**

```bash
git add platform/web-client/bin/web-client.ts platform/web-client/lib/web-client-stack.ts
git commit -m "Attach money.kkbae.com custom domain to the web-client CloudFront distribution"
```

---

### Task 5: Deploy `platform/web-client` and verify the new domain

**Files:** none (deploy action only)

**Interfaces:**
- Consumes: Task 3's completed nameserver propagation; Task 4's committed code.

- [ ] **Step 1: Confirm Task 3's `dig` check passed**

Do not proceed otherwise — the ACM certificate will not validate.

- [ ] **Step 2: Get explicit user confirmation before deploying**

This modifies the live CloudFront distribution serving the real web app.

- [ ] **Step 3: Deploy**

Run (from `platform/web-client/`): `npm run deploy`
Expected: builds the client, then `cdk deploy` runs. This step can take 10-20+ minutes — ACM DNS validation plus CloudFront's global distribution update are both slow. Wait for it to finish; if the certificate stays in "Pending validation" for a long time, re-check Task 3's `dig` result.

- [ ] **Step 4: Verify the new domain serves the app**

Run: `curl -I https://money.kkbae.com`
Expected: `HTTP/2 200`.

- [ ] **Step 5: Verify the old domain is unaffected**

Run: `curl -I https://<distribution-id>.cloudfront.net` (the value from the `DistributionDomainName` output)
Expected: `HTTP/2 200` — unchanged from before this pass.

---

### Task 6: Add `money.kkbae.com` to the SPA's redirect URIs

**Files:**
- Modify: `platform/entra-external-id/app-registrations/variables.tf`
- Modify: `platform/entra-external-id/app-registrations/main.tf`
- Modify: `platform/entra-external-id/CLAUDE.md`

**Interfaces:**
- Produces: `money-bae-dev`'s SPA app registration accepts both `https://d91s2th9i95hi.cloudfront.net/` and `https://money.kkbae.com/` as redirect URIs.

- [ ] **Step 1: Replace the `dev_redirect_uri` variable in `variables.tf`**

Change:
```hcl
variable "dev_redirect_uri" {
  type        = string
  description = "SPA redirect URI for the dev environment: the web client's deployed CloudFront domain (MoneyBaeWebClient-Dev stack's DistributionDomainName output). No custom domain exists yet, so this is pinned to the actual distribution's domain rather than computed — needs a manual update if the distribution is ever replaced."
  # Trailing slash required: Entra rejects a redirect URI with no path
  # segment unless it ends in "/".
  default = "https://d91s2th9i95hi.cloudfront.net/"
}
```
to:
```hcl
variable "dev_redirect_uri" {
  type        = list(string)
  description = "SPA redirect URIs for the dev environment: the web client's deployed CloudFront domain (MoneyBaeWebClient-Dev stack's DistributionDomainName output) plus the money.kkbae.com custom domain (see platform/dns and platform/web-client). Append future entries here rather than replacing existing ones."
  # Trailing slash required: Entra rejects a redirect URI with no path
  # segment unless it ends in "/".
  default = [
    "https://d91s2th9i95hi.cloudfront.net/",
    "https://money.kkbae.com/",
  ]
}
```

- [ ] **Step 2: Update `main.tf`'s `spa_dev` module**

Change:
```hcl
  redirect_uris                   = concat([var.dev_redirect_uri], local.common_redirect_uris)
```
(in the `module "spa_dev"` block) to:
```hcl
  redirect_uris                   = concat(var.dev_redirect_uri, local.common_redirect_uris)
```

- [ ] **Step 3: Update `platform/entra-external-id/CLAUDE.md`**

Change this paragraph (near the end of the "Create the app registrations" section):
```
`dev_redirect_uri` is pinned to `platform/web-client`'s actual deployed
CloudFront domain (see `app-registrations/variables.tf`). get the
current one with `aws cloudformation describe-stacks --stack-name
MoneyBaeWebClient-Dev --query "Stacks[0].Outputs"`. If the distribution
is ever replaced, update the variable's default (or pass
`-var="dev_redirect_uri=https://<new-domain>.cloudfront.net/"` — note
the trailing slash, Entra requires it) and re-apply.
```
to:
```
`dev_redirect_uri` is a list of the SPA's dev-environment redirect
URIs — the CloudFront domain (see `app-registrations/variables.tf`; get
the current one with `aws cloudformation describe-stacks --stack-name
MoneyBaeWebClient-Dev --query "Stacks[0].Outputs"` if the distribution
is ever replaced) plus `https://money.kkbae.com/` (see `platform/dns`
and `platform/web-client`). Append new entries to the list rather than
replacing existing ones when another domain is added later — note the
trailing slash on each, Entra requires it — then re-apply.
```

- [ ] **Step 4: Format check**

Run (from `platform/entra-external-id/`): `terraform -chdir=app-registrations fmt -check`
Expected: no output (already formatted). If it lists a file, run `terraform -chdir=app-registrations fmt` and review the diff.

- [ ] **Step 5: Validate**

Run: `terraform -chdir=app-registrations validate`
Expected: `Success! The configuration is valid.`

- [ ] **Step 6: Plan and review**

Run:
```bash
NEW_TENANT_ID=$(terraform -chdir=tenant output -raw tenant_id)
terraform -chdir=app-registrations plan \
  -var="ciam_tenant_id=$NEW_TENANT_ID" \
  -var-file="environments/shared.tfvars"
```
Expected: an in-place update (`~`) to `module.spa_dev`'s `azuread_application.this`, showing `single_page_application[0].redirect_uris` changing from a 3-element set (CloudFront domain + 2 common URIs) to a 4-element set that still includes the CloudFront domain plus the new `https://money.kkbae.com/` entry. No resource should show as destroyed/recreated, and the existing CloudFront URI must still be present in the "after" list.

- [ ] **Step 7: Commit**

```bash
git add platform/entra-external-id/app-registrations/variables.tf platform/entra-external-id/app-registrations/main.tf platform/entra-external-id/CLAUDE.md
git commit -m "Add money.kkbae.com to the dev SPA's redirect URIs"
```

---

### Task 7: Apply the Entra Terraform change

**Files:** none (apply action only)

- [ ] **Step 1: Show the user Task 6 Step 6's reviewed plan output and get explicit confirmation**

This is live Entra/Azure infrastructure per `platform/entra-external-id/CLAUDE.md`'s warning — never `apply` without a human confirming the plan first.

- [ ] **Step 2: Apply**

Run:
```bash
terraform -chdir=app-registrations apply \
  -var="ciam_tenant_id=$NEW_TENANT_ID" \
  -var-file="environments/shared.tfvars"
```
Expected: applies cleanly, same in-place update as the plan.

- [ ] **Step 3: Verify**

Run: `terraform -chdir=app-registrations output` or check the `money-bae-dev` app registration in the Entra admin portal.
Expected: its SPA redirect URIs include both `https://d91s2th9i95hi.cloudfront.net/` and `https://money.kkbae.com/`.

---

### Task 8: End-to-end verification

**Files:** none (manual verification only)

- [ ] **Step 1: Sign in on the new domain**

Open `https://money.kkbae.com` in a browser, sign in through the MSAL flow. Expected: redirects to Entra, back to `https://money.kkbae.com`, lands signed in.

- [ ] **Step 2: Confirm the app works past sign-in**

Expected: ledger/dashboard data loads (a real API call against `servers/api` succeeds with the acquired token).

- [ ] **Step 3: Confirm the old CloudFront URL still works**

Open `https://<distribution-id>.cloudfront.net`, sign in. Expected: still works exactly as before this pass — its redirect URI was never removed.

---

### Task 9: Fix `VITE_REDIRECT_URI` and redeploy `web-client`

Added mid-execution: Task 8's verification surfaced a real plan gap. Task 6/7 updated Entra's *allowlist* of valid redirect URIs, but never touched `clients/app/.env.dev`'s `VITE_REDIRECT_URI` — a build-time-baked value (loaded by Vite's `--mode dev`, used by `platform/web-client`'s `deploy` script via `npm run build:dev`) that controls the single redirect URI MSAL actually *requests* on sign-in. Left pointed at the old CloudFront domain, sign-in initiated from `money.kkbae.com` would still complete but bounce back to the CloudFront domain instead of staying on the new one.

**Files:**
- Modify: `clients/app/.env.dev`

**Interfaces:**
- Consumes: none new.
- Produces: the deployed SPA's MSAL config requests `https://money.kkbae.com/` as its redirect URI instead of the CloudFront domain.

- [ ] **Step 1: Update `clients/app/.env.dev`**

Change:
```
VITE_REDIRECT_URI=https://d91s2th9i95hi.cloudfront.net/
```
to:
```
VITE_REDIRECT_URI=https://money.kkbae.com/
```

- [ ] **Step 2: Build the client and confirm the value is baked in**

Run (from `clients/app/`): `npm run build:dev`
Expected: build succeeds; `grep -o 'https://money.kkbae.com/' dist/assets/*.js` (or similar) finds the new value in the built output, and the old CloudFront URL is no longer present as the configured `redirectUri` (it may still appear elsewhere, e.g. as a comment or the `.env.dev` file itself outside `dist/`, which does not matter — only the built JS output matters).

- [ ] **Step 3: Commit**

```bash
git add clients/app/.env.dev
git commit -m "Point the dev SPA's MSAL redirect URI at money.kkbae.com"
```

- [ ] **Step 4: Get explicit user confirmation, then redeploy `platform/web-client`**

Run (from `platform/web-client/`): `npm run deploy`
Expected: rebuilds the client (picking up the new `.env.dev` value) and redeploys — same distribution, no infrastructure changes, just new S3 content + CloudFront invalidation.

- [ ] **Step 5: Re-verify sign-in on both domains**

Repeat Task 8 Steps 1-3: sign in on `https://money.kkbae.com` (should now redirect back to itself, not the CloudFront domain) and confirm `https://<distribution-id>.cloudfront.net` still works too.
