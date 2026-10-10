import ReactDOM from 'react-dom/client'
import { RouterProvider, createRouter } from '@tanstack/react-router'
import { MsalProvider } from '@azure/msal-react'
import { ensureValidSession, msalInstance } from '#/auth/msalConfig'
import { routeTree } from './routeTree.gen'

const router = createRouter({
  routeTree,
  defaultPreload: 'intent',
  scrollRestoration: true,
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

const rootElement = document.getElementById('app')!

// initialize() and handleRedirectPromise() must both complete before any
// MSAL hook (useMsal, etc.) is used — handleRedirectPromise() is what
// actually processes the auth code after loginRedirect() sends the user
// back here.
await msalInstance.initialize()
await msalInstance.handleRedirectPromise()
// Refresh the token (or drop to login) before the app renders and fires
// its first API request, and again whenever the user returns to the tab.
await ensureValidSession()
document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'visible') void ensureValidSession()
})

// #app starts with the static loading screen from index.html, so guard
// against double-mounting (HMR) with a flag instead of "is it empty".
if (!rootElement.hasAttribute('data-mounted')) {
  rootElement.setAttribute('data-mounted', '')
  const root = ReactDOM.createRoot(rootElement)
  root.render(
    <MsalProvider instance={msalInstance}>
      <RouterProvider router={router} />
    </MsalProvider>,
  )
}
