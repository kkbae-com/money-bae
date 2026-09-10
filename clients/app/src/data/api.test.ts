import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('#/auth/msalConfig', () => ({
  getAccessToken: vi.fn(),
}))

const { getAccessToken } = await import('#/auth/msalConfig')
const { listIncomes } = await import('./api')

const getAccessTokenMock = vi.mocked(getAccessToken)
const fetchMock = vi.fn()
vi.stubGlobal('fetch', fetchMock)

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status })
}

beforeEach(() => {
  getAccessTokenMock.mockReset()
  fetchMock.mockReset()
})

describe('requestOnce (via listIncomes)', () => {
  it('retries once with a force-refreshed token after a 401, returning the retried result', async () => {
    getAccessTokenMock
      .mockResolvedValueOnce('stale-token')
      .mockResolvedValueOnce('fresh-token')
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, {}))
      .mockResolvedValueOnce(jsonResponse(200, []))

    await expect(listIncomes()).resolves.toEqual([])

    expect(getAccessTokenMock).toHaveBeenNthCalledWith(2, {
      forceRefresh: true,
    })
    expect(fetchMock).toHaveBeenCalledTimes(2)
    const secondCallHeaders = fetchMock.mock.calls[1]?.[1]?.headers as Record<
      string,
      string
    >
    expect(secondCallHeaders.Authorization).toBe('Bearer fresh-token')
  })

  it('throws after a second 401 even after the forced retry', async () => {
    getAccessTokenMock
      .mockResolvedValueOnce('stale-token')
      .mockResolvedValueOnce('still-stale-token')
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401, {}))
      .mockResolvedValueOnce(jsonResponse(401, {}))

    await expect(listIncomes()).rejects.toThrow('failed: 401')
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })
})
