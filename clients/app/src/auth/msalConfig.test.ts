import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@azure/msal-browser', () => {
  class InteractionRequiredAuthError extends Error {}
  return {
    InteractionRequiredAuthError,
    PublicClientApplication: vi.fn().mockImplementation(() => ({
      getActiveAccount: vi.fn(() => null),
      getAllAccounts: vi.fn(() => [{ homeAccountId: 'test-account' }]),
      acquireTokenSilent: vi.fn(),
      acquireTokenRedirect: vi.fn(),
    })),
  }
})

const { InteractionRequiredAuthError } = await import('@azure/msal-browser')
const { msalInstance, getAccessToken } = await import('./msalConfig')

const acquireTokenSilent = vi.mocked(msalInstance.acquireTokenSilent)
const acquireTokenRedirect = vi.mocked(msalInstance.acquireTokenRedirect)

beforeEach(() => {
  acquireTokenSilent.mockReset()
  acquireTokenRedirect.mockReset()
})

describe('getAccessToken', () => {
  it('returns the token when silent acquisition succeeds', async () => {
    acquireTokenSilent.mockResolvedValueOnce({
      accessToken: 'token-a',
    } as never)

    await expect(getAccessToken()).resolves.toBe('token-a')
    expect(acquireTokenSilent).toHaveBeenCalledTimes(1)
  })

  it('redirects and rethrows when silent acquisition requires interaction', async () => {
    const err = new InteractionRequiredAuthError(
      'interaction_required',
      'test-correlation-id',
    )
    acquireTokenSilent.mockRejectedValueOnce(err)
    acquireTokenRedirect.mockResolvedValueOnce(undefined)

    await expect(getAccessToken()).rejects.toBe(err)
    expect(acquireTokenRedirect).toHaveBeenCalledTimes(1)
  })

  it('retries once with a forced refresh when silent acquisition fails with a non-interaction error', async () => {
    acquireTokenSilent
      .mockRejectedValueOnce(new Error('monitor_window_timeout'))
      .mockResolvedValueOnce({ accessToken: 'token-b' } as never)

    await expect(getAccessToken()).resolves.toBe('token-b')
    expect(acquireTokenSilent).toHaveBeenCalledTimes(2)
    expect(acquireTokenSilent).toHaveBeenLastCalledWith(
      expect.objectContaining({ forceRefresh: true }),
    )
    expect(acquireTokenRedirect).not.toHaveBeenCalled()
  })

  it('throws without redirecting when the forced retry also fails with a non-interaction error', async () => {
    acquireTokenSilent
      .mockRejectedValueOnce(new Error('monitor_window_timeout'))
      .mockRejectedValueOnce(new Error('monitor_window_timeout'))

    await expect(getAccessToken()).rejects.toThrow('monitor_window_timeout')
    expect(acquireTokenSilent).toHaveBeenCalledTimes(2)
    expect(acquireTokenRedirect).not.toHaveBeenCalled()
  })
})
