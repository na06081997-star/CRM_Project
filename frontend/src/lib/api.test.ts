import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, download, setToken } from './api'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  setToken(null)
})

function stubDownload(response: Response) {
  const fetchMock = vi.fn(async () => response)
  vi.stubGlobal('fetch', fetchMock)
  const created = vi.fn(() => 'blob:x')
  vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: created, revokeObjectURL: vi.fn() }))
  const saved: string[] = []
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
    saved.push(this.download)
  })
  return { fetchMock, saved }
}

describe('download', () => {
  it('sends the session and the active filters, and saves under the Hebrew name chosen by the server', async () => {
    setToken('tok')
    const { fetchMock, saved } = stubDownload(
      new Response(new Blob(['xlsx']), {
        status: 200,
        headers: { 'Content-Disposition': `attachment; filename="bookings.xlsx"; filename*=UTF-8''${encodeURIComponent('הזמנות_2026-11-01_2026-11-30.xlsx')}` },
      }),
    )
    await download('/bookings/export.xlsx', { from: '2026-11-01', roomIds: ['r1', 'r2'], coordinatorId: undefined }, 'fallback.xlsx')

    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect(url).toBe('/api/bookings/export.xlsx?from=2026-11-01&roomIds=r1%2Cr2')
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer tok')
    expect(saved).toEqual(['הזמנות_2026-11-01_2026-11-30.xlsx'])
  })

  it('reports the server error in Hebrew instead of saving a broken file', async () => {
    const { saved } = stubDownload(new Response(JSON.stringify({ error: 'forbidden', message: 'אין הרשאה' }), { status: 403 }))
    await expect(download('/payments/export.xlsx', undefined, 'p.xlsx')).rejects.toBeInstanceOf(ApiError)
    expect(saved).toEqual([])
  })
})
