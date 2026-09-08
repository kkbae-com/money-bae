# Platform DNS — Design Spec

## Purpose

`kkbae.com` is registered but has no DNS configured — nothing exists at
it yet (no other subdomains, no email/MX). `platform/web-client`'s
CloudFront distribution is only reachable at its raw
`*.cloudfront.net` domain today. This pass provisions Route 53 DNS for
`kkbae.com` and gives the web app a real custom domain
(`money.kkbae.com`), plus updates the Entra app registration's redirect
URIs so sign-in keeps working on the new domain. Tracked as
[#82](https://github.com/mrcunninghamz/money-bae/issues/82).

It does not set up `api.kkbae.com` / API Gateway routing to
`servers/api` — that's a separate follow-up
([#83](https://github.com/mrcunninghamz/money-bae/issues/83)) once this
DNS foundation exists.

## Scope of this pass

In scope:
- New `platform/dns` CDK app: one Route 53 public hosted zone for the
  `kkbae.com` apex.
- `platform/web-client`: an ACM certificate for `money.kkbae.com`
  (`us-east-1`, required by CloudFront), DNS-validated against the
  imported zone; the CloudFront distribution gets `domainNames` +
  `certificate`; a Route 53 alias record `money.kkbae.com` → the
  distribution.
- `platform/entra-external-id`: `dev_redirect_uri` becomes a list,
  adding `https://money.kkbae.com/` alongside the existing CloudFront
  redirect URI (kept, not replaced).
- Manual, non-Terraform/CDK step: once the hosted zone exists, update
  `kkbae.com`'s nameservers at the registrar to Route 53's.

Deferred / out of scope:
- `api.kkbae.com` and API Gateway routing to `servers/api` (#83).
- Migrating any existing DNS records (none exist to migrate).
- `local_redirect_uri` / the local dev sign-in flow — unaffected.

## Why a separate `platform/dns` app, and why it only owns the zone

`platform/dns` and `platform/web-client` are separate CDK apps (separate
state, separate `cdk.json`), so there's no native CDK cross-stack
reference between them — consistent with how the rest of this repo
already bridges separately-deployed AWS/Terraform projects: a stable
identifier created by one stack gets pasted as a literal into the stack
that consumes it (`platform/api/cdk/lib/api-stack.ts` imports the ECR
repo by a fixed name and the Secrets Manager secret by a fixed name;
`platform/entra-external-id`'s OIDC issuer URL/client ID are pasted into
`api-stack.ts` as literals).

`platform/dns` is kept to *only* the hosted zone (not the cert, not any
records) so that surface stays minimal and reusable: the only thing
another stack needs to borrow is the zone's ID, a value that never
changes once the zone exists. Each consuming stack then owns everything
specific to its own subdomain — its own ACM certificate, its own Route
53 record — the same shape #83's future `api.kkbae.com` stack will
follow later, borrowing the same zone ID without touching
`platform/dns` again. This avoids a fragile ACM-ARN hand-off (ARNs
aren't as stable/predictable a "paste this literal" value as a hosted
zone ID) and keeps `platform/dns` a true singleton — one zone, not
per-environment like `web-client`/`api`'s `-Dev`/`-Prod` stack suffixes,
since there's only one `kkbae.com`.

## Repo layout

Mirrors `platform/web-client/`'s structure exactly:

```
platform/dns/
├── bin/dns.ts
├── lib/dns-stack.ts
├── cdk.json
├── package.json
└── tsconfig.json
```

## `platform/dns` — `DnsStack`

```ts
// lib/dns-stack.ts
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

```ts
// bin/dns.ts
new DnsStack(app, 'MoneyBaeDns', {
  env: { account: process.env.CDK_DEFAULT_ACCOUNT, region: 'us-east-1' },
});
```

No env-suffix (`-Dev`/`-Prod`) — unlike `web-client`/`api`, this isn't
a per-environment stack; there's exactly one `kkbae.com` zone shared by
everything under it, the same "one shared thing, not per-environment"
shape `platform/entra-external-id`'s tenant already uses.

## `platform/web-client` changes

`bin/web-client.ts` currently lets `region` come from ambient
`CDK_DEFAULT_REGION`. Since an ACM cert for CloudFront *must* be
`us-east-1` regardless of the deployer's AWS CLI profile default, pin it
explicitly there too — same fix as `platform/dns`'s `bin/dns.ts` above —
rather than relying on the deployer's profile happening to default to
`us-east-1` (it does today, since App Runner already deploys there, but
pinning removes the ambiguity rather than depending on it).

```ts
// lib/web-client-stack.ts — additions
const zone = route53.HostedZone.fromHostedZoneAttributes(this, 'Zone', {
  hostedZoneId: '<paste from platform/dns's HostedZoneId output>',
  zoneName: 'kkbae.com',
});

const certificate = new acm.Certificate(this, 'SiteCertificate', {
  domainName: 'money.kkbae.com',
  validation: acm.CertificateValidation.fromDns(zone),
});

const distribution = new cloudfront.Distribution(this, 'SiteDistribution', {
  domainNames: ['money.kkbae.com'],
  certificate,
  // ...existing defaultBehavior/errorResponses unchanged
});

new route53.ARecord(this, 'SiteAliasRecordV4', {
  zone,
  recordName: 'money.kkbae.com',
  target: route53.RecordTarget.fromAlias(new targets.CloudFrontTarget(distribution)),
});
new route53.AaaaRecord(this, 'SiteAliasRecordV6', {
  zone,
  recordName: 'money.kkbae.com',
  target: route53.RecordTarget.fromAlias(new targets.CloudFrontTarget(distribution)),
});
```

`aws-cdk-lib` already bundles `aws-certificatemanager`, `aws-route53`,
and `aws-route53-targets` (all part of the single `aws-cdk-lib` v2
package) — no new npm dependencies needed.

## `platform/entra-external-id` changes

```hcl
# app-registrations/variables.tf
variable "dev_redirect_uri" {
  type        = list(string)
  description = "SPA redirect URIs for the dev environment: the web client's CloudFront domain (kept, for direct distribution access) plus the money.kkbae.com custom domain."
  default = [
    "https://d91s2th9i95hi.cloudfront.net/",
    "https://money.kkbae.com/",
  ]
}
```

```hcl
# app-registrations/main.tf
module "spa_dev" {
  # ...
  redirect_uris = concat(var.dev_redirect_uri, local.common_redirect_uris)
  # was: concat([var.dev_redirect_uri], local.common_redirect_uris)
}
```

Live infrastructure per `platform/entra-external-id/CLAUDE.md` — `plan`
output gets reviewed before any `apply`.

That same `CLAUDE.md` (step 3) currently documents `dev_redirect_uri` as
a single value pinned to the CloudFront domain — its prose needs
updating alongside the variable to describe the list and the manual
step of appending a new entry (rather than replacing the default)
whenever the domain changes again.

## Deploy order / rollout

1. Deploy `platform/dns` → read `HostedZoneId` and `NameServers` from
   its outputs.
2. Manually update `kkbae.com`'s nameservers at the registrar to those
   `NameServers` values (outside the repo/AWS — a registrar UI action).
   Verify propagation with `dig NS kkbae.com` before continuing.
3. Paste the `HostedZoneId` into `web-client-stack.ts`, deploy
   `platform/web-client`. ACM's DNS validation stays "pending" until
   step 2's delegation has actually propagated, so this step blocks on
   that.
4. `terraform -chdir=app-registrations plan`/`apply` for the
   `dev_redirect_uri` change.
5. Verify: `curl -I https://money.kkbae.com` serves the SPA (200), a
   full sign-in run-through works on the new domain, and the existing
   `*.cloudfront.net` URL still works unchanged.

## Testing

No dedicated unit tests — this is thin, declarative CDK/Terraform
composition over trusted constructs and providers (a hosted zone, a
DNS-validated cert, a CloudFront alias, a redirect URI list), the same
reasoning `web-client-stack.ts` already has no tests today. Verification
is the real deploy plus the `dig`/`curl`/sign-in checks in step 5, not
synth-time assertions.

## Cost / risk

- Hosted zone: ~$0.50/month + ~$0.40/million queries. ACM cert: free.
- No risk to the existing service: `money.kkbae.com` is an *added*
  alternate domain on the same distribution — nothing about the current
  `*.cloudfront.net` access path changes or is removed. The Entra change
  is additive (existing redirect URI kept, not replaced).
- Nothing else depends on `kkbae.com` yet, so the hosted zone itself is
  low-risk to unwind if needed (destroy it, revert the nameserver change
  at the registrar).

## Deferred / out of scope (recap)

- `api.kkbae.com` / API Gateway routing to `servers/api` — #83.
- `local_redirect_uri` / local dev sign-in flow — unaffected.
- Migrating existing DNS records — none exist.
