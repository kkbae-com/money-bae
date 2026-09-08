# platform/dns

CDK app owning the `kkbae.com` Route 53 public hosted zone (`MoneyBaeDns`
stack). This is intentionally minimal — a single hosted zone, nothing
else (no certificates, no records, no per-environment stack suffix).
Each consuming stack (e.g. `platform/web-client`) owns its own
subdomain's ACM certificate and Route 53 records, importing this zone
by ID rather than by CDK cross-stack reference (this is a separate CDK
app, with its own state).

## Deploy

```bash
npm install
npm run deploy
```

Outputs `HostedZoneId` and `NameServers`.

## One-time manual step: registrar nameserver cutover

`kkbae.com` was registered elsewhere (not Route 53) with no prior DNS
configured. After this stack's first deploy, `kkbae.com`'s nameservers
were manually updated at the registrar to the four `NameServers` output
values — this is a registrar-UI action outside AWS/this repo, not
something CDK manages. Verify propagation with `dig +short NS
kkbae.com` — it should return Route 53's four nameservers.

## Consuming this zone's ID

Other stacks import the zone by ID as a pasted literal (the same
"paste a stable identifier from one stack's output into another"
pattern already used elsewhere in this repo — e.g.
`platform/api/cdk`'s ECR repo and Secrets Manager references), since
this is a separate CDK app with no native cross-stack reference
available. Get the current `HostedZoneId` with:

```bash
aws cloudformation describe-stacks --stack-name MoneyBaeDns --query "Stacks[0].Outputs"
```

If the zone is ever replaced, every consumer's pasted `HOSTED_ZONE_ID`
(currently `platform/web-client/lib/web-client-stack.ts`) needs updating
to match, and the registrar's nameservers need re-pointing at the new
zone's `NameServers`.
