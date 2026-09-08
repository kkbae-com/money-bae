locals {
  # oauth.pstmn.io is Postman's OAuth2 callback proxy, added to every SPA
  # app's redirect URIs so the Postman collection (see ../CLAUDE.md) can
  # obtain real tokens for manual API testing without a second app
  # registration. CIAM rejects a SPA app's token exchange without a
  # request that looks like a real cross-origin browser request
  # (AADSTS9002327) — fixed via the collection's tokenRequestParams
  # Origin header, not via a different redirect URI/callback mechanism.
  # jwt.ms is Microsoft's own token-decoding tool, useful for inspecting
  # claims during manual testing directly in a browser.
  common_redirect_uris = [
    "https://oauth.pstmn.io/v1/callback",
    "https://jwt.ms/",
  ]
}

module "api_local" {
  source = "./modules/api-registration"

  display_name = "money-bae-api-local"
}

module "api_dev" {
  source = "./modules/api-registration"

  display_name = "money-bae-api-dev"
}

module "spa_local" {
  source = "./modules/spa-registration"

  display_name                    = "money-bae-local"
  redirect_uris                   = concat([var.local_redirect_uri], local.common_redirect_uris)
  api_client_id                   = module.api_local.client_id
  api_scope_id                    = module.api_local.scope_id
  api_service_principal_object_id = module.api_local.service_principal_object_id
}

module "spa_dev" {
  source = "./modules/spa-registration"

  display_name                    = "money-bae-dev"
  redirect_uris                   = concat(var.dev_redirect_uri, local.common_redirect_uris)
  api_client_id                   = module.api_dev.client_id
  api_scope_id                    = module.api_dev.scope_id
  api_service_principal_object_id = module.api_dev.service_principal_object_id
}
