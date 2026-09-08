# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this directory.

## Project Overview

`platform/entra-external-id` is the Terraform (IaC) for money-bae's Microsoft
Entra External ID (CIAM) tenant and its app registrations. One tenant is
shared across environments — mirroring how `../db` shares one Postgres
server across two databases — holding four app registrations:

- `money-bae-local` / `money-bae-dev` — SPA (PKCE) clients for `../../clients/app`
- `money-bae-api-local` / `money-bae-api-dev` — API resources for `../../servers/api`,
  each exposing an `access_as_user` scope that its matching SPA app is
  pre-consented for.

Design spec: `../../docs/superpowers/specs/2026-09-04-entra-external-id-design.md`

## ⚠️ This Is Live Infrastructure

Real Azure/Entra resources. Never run `terraform apply` or `terraform
destroy` without explicit human confirmation of the `terraform plan`
output first. `terraform plan`/`validate`/`fmt`/`state list` are safe to
run freely.

## Bootstrap the state backend (one-time, already done)

`rg-kkbae-tfstate-shared` / `stkkbtfstatecus` are a dedicated,
company-level Terraform state backend for this project — separate from
`platform/db`'s existing `rg-moneybae-tfstate-shared`/`stmbtfstateshared`
(money-bae product-level state), because this tenant is company-level
shared infrastructure (company `kkbae`), not money-bae-specific.

This lives in a personal Azure subscription
(`085f952f-488d-4c4d-bd33-0fcf8fd37e17`, tenant `kmerecidolive.onmicrosoft.com`),
not the original money-bae subscription (`c6f1212c-ec19-425f-96a0-41f2db717ea8`,
still used by `platform/db`) — creating a new Entra tenant requires the
Tenant Creator directory role, which wasn't available in the original
subscription's tenant. The storage account is named `stkkbtfstatecus`
rather than `stkkbtfstateshared` (the name used for the equivalent
resource group) because `stkkbtfstateshared` was already taken by an
abandoned bootstrap attempt in the old subscription (storage account
names are globally unique across all of Azure, not just within a
subscription) — see the GitHub issue tracking that cleanup.

This backend doesn't exist until it's created — a classic bootstrap
chicken-and-egg (Terraform's `azurerm` backend needs the storage account
to already exist before `terraform init` can target it) — so it's created
via `az` CLI, once, not Terraform:

```bash
az group create \
  --name rg-kkbae-tfstate-shared \
  --location centralus \
  --subscription 085f952f-488d-4c4d-bd33-0fcf8fd37e17

az storage account create \
  --name stkkbtfstatecus \
  --resource-group rg-kkbae-tfstate-shared \
  --location centralus \
  --sku Standard_LRS \
  --kind StorageV2 \
  --min-tls-version TLS1_2 \
  --allow-blob-public-access false \
  --subscription 085f952f-488d-4c4d-bd33-0fcf8fd37e17

az storage container create \
  --name tfstate \
  --account-name stkkbtfstatecus \
  --auth-mode login \
  --subscription 085f952f-488d-4c4d-bd33-0fcf8fd37e17
```

This has already been run — the resource group, storage account, and
container all exist. Only needed again if this backend is ever recreated
from scratch.

## Isolated az CLI credentials

The repo root has a `.envrc` (direnv, not committed) setting
`AZURE_CONFIG_DIR=$PWD/.azure` — this gives money-bae's own `az` CLI
session (accounts, cached tokens, current subscription) a completely
separate store from your global `~/.azure`, or any other project's own
isolated one. This matters here specifically because this project's
Terraform juggles multiple Azure accounts/tenants/subscriptions at once
(the money-bae subscription, the personal subscription hosting the CIAM
tenant, and the CIAM tenant itself) — keeping that scoped to this repo
avoids it clobbering whatever `az` context you're using elsewhere.

If your shell has direnv's hook installed (see direnv's own setup docs)
and you've run `direnv allow` once in this repo, `cd`-ing into
money-bae automatically loads this. Otherwise, `export
AZURE_CONFIG_DIR=$PWD/.azure` yourself (from the repo root) before
running any command below — including `az login`.

## Two separate Terraform states, applied in order

`tenant/` and `app-registrations/` are independent root modules with
independent state, because the `azuread` provider needs the new tenant's
ID before it can do anything — which doesn't exist until `tenant/` has
already been applied.

All commands below run from `platform/entra-external-id/` using
`terraform -chdir=<dir>` — never `cd` into `tenant/` or
`app-registrations/` first (`-chdir` is always relative to where you run
`terraform` from, so combining a `cd` with `-chdir` resolves to a
nonexistent nested directory).

### 0. Log in and select the right subscription

```bash
az login  # opens a browser (or use --use-device-code from a headless shell)
az account set --subscription 085f952f-488d-4c4d-bd33-0fcf8fd37e17
```

The `az account set` step matters even right after logging in: if your
account has access to multiple subscriptions across multiple tenants
(likely, for a personal account), `az login` picks whichever one it
finds first as "current" — which may not be
`085f952f-488d-4c4d-bd33-0fcf8fd37e17` (the one that owns the
`rg-kkbae-tfstate-shared`/`stkkbtfstatecus` backend). If `terraform
init` fails with `InvalidAuthenticationTokenTenant: The access token is
from the wrong issuer`, this is why — run `az account set` again and
retry.

### 1. Create the tenant

```bash
terraform -chdir=tenant init \
  -backend-config="resource_group_name=rg-kkbae-tfstate-shared" \
  -backend-config="storage_account_name=stkkbtfstatecus" \
  -backend-config="container_name=tfstate" \
  -backend-config="key=identity/tenant.tfstate" \
  -backend-config="subscription_id=085f952f-488d-4c4d-bd33-0fcf8fd37e17"
terraform -chdir=tenant plan -var-file="environments/shared.tfvars"
# review the plan carefully, then:
terraform -chdir=tenant apply -var-file="environments/shared.tfvars"
```

### 2. Re-authenticate against the new tenant

The account that creates a CIAM tenant automatically becomes its Global
Administrator, but your Azure CLI session is still authenticated against
your normal subscription tenant. CIAM tenants have no subscription
attached, so:

```bash
NEW_TENANT_ID=$(terraform -chdir=tenant output -raw tenant_id)
az login --tenant "$NEW_TENANT_ID" --allow-no-subscriptions
```

### 3. Create the app registrations

```bash
terraform -chdir=app-registrations init \
  -backend-config="resource_group_name=rg-kkbae-tfstate-shared" \
  -backend-config="storage_account_name=stkkbtfstatecus" \
  -backend-config="container_name=tfstate" \
  -backend-config="key=identity/app-registrations.tfstate" \
  -backend-config="subscription_id=085f952f-488d-4c4d-bd33-0fcf8fd37e17"
terraform -chdir=app-registrations plan \
  -var="ciam_tenant_id=$NEW_TENANT_ID" \
  -var-file="environments/shared.tfvars"
# review the plan carefully, then:
terraform -chdir=app-registrations apply \
  -var="ciam_tenant_id=$NEW_TENANT_ID" \
  -var-file="environments/shared.tfvars"
```

`dev_redirect_uri` is a list of the SPA's dev-environment redirect
URIs — the CloudFront domain (see `app-registrations/variables.tf`; get
the current one with `aws cloudformation describe-stacks --stack-name
MoneyBaeWebClient-Dev --query "Stacks[0].Outputs"` if the distribution
is ever replaced) plus `https://money.kkbae.com/` (see `platform/dns`
and `platform/web-client`). Append new entries to the list rather than
replacing existing ones when another domain is added later — note the
trailing slash on each, Entra requires it — then re-apply.

### 4. Wire up the Postman collection

Once `app-registrations/` is applied, fill in the OAuth variables in
`../../servers/api/docs/collections/money-bae-api-local.postman_environment.json`
and `money-bae-api-dev.postman_environment.json` using:

```bash
terraform -chdir=app-registrations output -raw spa_client_id_local
terraform -chdir=app-registrations output -raw api_identifier_uri_local
terraform -chdir=tenant output -raw tenant_domain
```

`authUrl`/`tokenUrl` follow the pattern
`https://<ciam_tenant_name>.ciamlogin.com/<tenant_domain>/oauth2/v2.0/{authorize,token}`
— e.g. for this tenant:
`https://kkbaeexternalid.ciamlogin.com/kkbaeexternalid.onmicrosoft.com/oauth2/v2.0/authorize`.
`scope` is `<api_identifier_uri>/access_as_user`.

## Deferred (not covered by this Terraform)

- The CIAM user flow (sign-up/sign-in experience) — no Terraform
  resource for this yet in `azuread` or `azapi` (ARM-only); it's a
  Microsoft Graph `identity/` concept. Configure manually in the Portal
  for now.
- Identity provider config — local-account policy and social logins
  (Google, Facebook, Apple) are Microsoft Graph
  `identity/identityProviders` (`socialIdentityProvider`) resources,
  same gap as user flows.
- See the GitHub issue tracking research into automating both (filed
  after this Terraform ships).
