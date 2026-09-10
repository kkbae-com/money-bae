import {
  InteractionRequiredAuthError,
  PublicClientApplication,
} from '@azure/msal-browser'
import type { Configuration } from '@azure/msal-browser'

// kkbaeexternalid is money-bae's Entra External ID (CIAM) tenant —
// company-level shared infrastructure, see platform/entra-external-id/.
// The authority's tenant segment is the tenant ID, not its name: CIAM's
// issuer (what MSAL validates tokens against) only matches when queried
// this way, confirmed against a real token's `iss` claim.
const TENANT_AUTHORITY =
  'https://kkbaeexternalid.ciamlogin.com/6da7bb61-fb8f-4d95-aa49-7808c0b05d51'

const msalConfig: Configuration = {
  auth: {
    // money-bae-local's client ID — override with money-bae-dev's for a
    // real deploy build (VITE_MSAL_CLIENT_ID), matching how
    // VITE_API_BASE_URL is already handled in src/data/api.ts.
    clientId:
      import.meta.env.VITE_MSAL_CLIENT_ID ??
      'e92e517b-d90a-485d-8b2f-5d6bae83d3ee',
    authority: TENANT_AUTHORITY,
    // Must exactly match a redirect URI registered on this app in Entra —
    // a constant, not derived from window.location (which defaults to
    // whatever page the user happened to be on, e.g. /login, when MSAL
    // defaults this). Override with VITE_REDIRECT_URI for a real deploy
    // build, matching VITE_MSAL_CLIENT_ID/VITE_API_SCOPE/VITE_API_BASE_URL.
    redirectUri: import.meta.env.VITE_REDIRECT_URI ?? 'http://localhost:3000/',
  },
  cache: {
    // Persists the session across page reloads/tabs, same as the
    // mock auth flag it replaces (data/store.tsx's old AUTH_STORAGE_KEY).
    cacheLocation: 'localStorage',
  },
}

export const msalInstance = new PublicClientApplication(msalConfig)

// Requests an ID token with basic profile claims — used for sign-in itself.
// email is required to actually get the `email` claim: servers/api's
// Claims.Email maps to the JWT's "email" field specifically (not
// preferred_username), and user_resolver.go uses it when creating a new
// user row. given_name/family_name aren't in this tenant's
// claims_supported at all (confirmed against the real discovery
// document), so there's no scope that would surface a first/last name.
export const loginRequest = {
  scopes: ['openid', 'profile', 'email'],
}

// Requests an access token scoped to the money-bae API. Kept separate from
// loginRequest: access tokens are audience-specific per resource, so the
// API's scope is acquired independently of the sign-in request.
const apiRequest = {
  scopes: [
    import.meta.env.VITE_API_SCOPE ??
      'api://money-bae-api-local/access_as_user',
  ],
}

// Used by data/api.ts for every request — acquires (silently refreshing if
// needed) an access token for the money-bae API, scoped to whichever
// account is currently signed in. Pass forceRefresh to bypass the cached
// token (data/api.ts does this after a 401, in case the cached token is
// stale in a way MSAL itself didn't detect).
export async function getAccessToken(
  options: { forceRefresh?: boolean } = {},
): Promise<string> {
  const account =
    msalInstance.getActiveAccount() ?? msalInstance.getAllAccounts()[0]
  // tsconfig doesn't set noUncheckedIndexedAccess, so TS types
  // getAllAccounts()[0] as always-defined — this guard is still needed at
  // runtime for the empty-array case (signed out).
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
  if (!account) {
    throw new Error('not signed in')
  }

  try {
    const result = await msalInstance.acquireTokenSilent({
      ...apiRequest,
      account,
      forceRefresh: options.forceRefresh ?? false,
    })
    return result.accessToken
  } catch (err) {
    if (err instanceof InteractionRequiredAuthError) {
      // Navigates away — acquireTokenRedirect never resolves on this page.
      await msalInstance.acquireTokenRedirect({ ...apiRequest, account })
      throw err
    }
    // Not (yet) known to require interaction — could be a transient
    // failure (blocked hidden iframe, network blip, a concurrent renewal
    // already in flight elsewhere). One forced retry before giving up, so
    // a single hiccup doesn't strand the app until a manual reload.
    if (options.forceRefresh) {
      throw err
    }
    return getAccessToken({ forceRefresh: true })
  }
}
