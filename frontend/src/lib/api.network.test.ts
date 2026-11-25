import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError, errorMessage, network, setToken } from './api'

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const netError = () => new TypeError('Failed to fetch')

beforeEach(() => {
  network.retryDelaysMs = [0, 0, 0]
  network.lastOk = Date.now()
})
afterEach(() => {
  vi.unstubAllGlobals()
  setToken(null)
})

describe('network resilience', () => {
  it('retries a read that failed at the network level, then succeeds', async () => {
    const fetchMock = vi.fn().mockRejectedValueOnce(netError()).mockRejectedValueOnce(netError()).mockResolvedValue(json({ ok: 1 }))
    vi.stubGlobal('fetch', fetchMock)
    await expect(api.get('/rooms')).resolves.toEqual({ ok: 1 })
    expect(fetchMock).toHaveBeenCalledTimes(3)
  })

  it('gives a clear Hebrew message when the server stays unreachable', async () => {
    const fetchMock = vi.fn().mockRejectedValue(netError())
    vi.stubGlobal('fetch', fetchMock)
    const err = (await api.get('/rooms').catch((e) => e)) as ApiError
    expect(err).toBeInstanceOf(ApiError)
    expect(err.code).toBe('network')
    expect(errorMessage(err)).toContain('אין חיבור לשרת')
    expect(fetchMock).toHaveBeenCalledTimes(4) // first try + 3 retries
  })

  it('never re-sends a change (it may have reached the server)', async () => {
    const fetchMock = vi.fn().mockRejectedValue(netError())
    vi.stubGlobal('fetch', fetchMock)
    const err = (await api.post('/coordinators', { fullName: 'x' }).catch((e) => e)) as ApiError
    expect(err.code).toBe('network')
    expect(fetchMock.mock.calls.filter(([u]) => String(u).includes('/coordinators'))).toHaveLength(1)
  })

  it('after a pause, opens a fresh connection with a safe request before sending a change', async () => {
    network.lastOk = Date.now() - 5 * 60_000
    const fetchMock = vi
      .fn()
      .mockRejectedValueOnce(netError()) // the first new connection fails (filtered / stale)
      .mockResolvedValueOnce(json({ status: 'ok' })) // health retried
      .mockResolvedValueOnce(json({ id: 'c-1' }, 201)) // the change itself
    vi.stubGlobal('fetch', fetchMock)
    await expect(api.post('/coordinators', { fullName: 'x' })).resolves.toEqual({ id: 'c-1' })
    const urls = fetchMock.mock.calls.map(([u]) => String(u))
    expect(urls).toEqual(['/api/health', '/api/health', '/api/coordinators'])
  })

  it('does not warm up when the connection was just used', async () => {
    const fetchMock = vi.fn().mockResolvedValue(json({ id: 'c-1' }, 201))
    vi.stubGlobal('fetch', fetchMock)
    await api.post('/coordinators', {})
    expect(fetchMock.mock.calls.map(([u]) => String(u))).toEqual(['/api/coordinators'])
  })
})
