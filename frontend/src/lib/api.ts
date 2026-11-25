import type { Conflict } from './types'

export class ApiError extends Error {
  status: number
  code: string
  fields: Record<string, string>
  conflicts: Conflict[]
  /** Seconds to wait before retrying (rate limits, resend cooldown). */
  retryAfter: number

  constructor(status: number, body: { error?: string; message?: string; fields?: Record<string, string>; conflicts?: Conflict[]; retryAfter?: number }) {
    super(body.message || 'אירעה שגיאה')
    this.status = status
    this.code = body.error || 'unknown'
    this.fields = body.fields || {}
    this.conflicts = body.conflicts || []
    this.retryAfter = body.retryAfter || 0
  }
}

let token: string | null = null
let onUnauthorized: (() => void) | null = null

export function setToken(t: string | null) {
  token = t
}
export function getToken() {
  return token
}
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

type Query = Record<string, string | number | boolean | string[] | null | undefined>

function qs(q?: Query) {
  if (!q) return ''
  const p = new URLSearchParams()
  for (const [k, v] of Object.entries(q)) {
    if (v === undefined || v === null || v === '') continue
    if (Array.isArray(v)) {
      if (v.length) p.set(k, v.join(','))
    } else p.set(k, String(v))
  }
  const s = p.toString()
  return s ? `?${s}` : ''
}

/**
 * Network resilience. Some networks (ISP content filters, mobile hand-overs,
 * sleeping laptops) break new connections now and then: the browser reports
 * "Failed to fetch" before any answer. Reads are retried with a short backoff;
 * changes are never re-sent (they may have reached the server), but after a
 * pause a safe request first opens a fresh connection, so the change travels
 * on a connection that works.
 */
export const network = {
  retryDelaysMs: [400, 1200, 2500],
  /** Idle time after which a change is preceded by a warm-up request. */
  idleMs: 20_000,
  lastOk: Date.now(),
}

const NETWORK_MESSAGE = 'אין חיבור לשרת כרגע. בדקו את החיבור לאינטרנט ונסו שוב.'

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

function networkError(): ApiError {
  return new ApiError(0, { error: 'network', message: NETWORK_MESSAGE })
}

/** fetch that retries failures before any HTTP answer (never for changes). */
async function send(url: string, init: RequestInit = {}): Promise<Response> {
  const method = (init.method ?? 'GET').toUpperCase()
  const safe = method === 'GET' || method === 'HEAD'
  if (!safe && Date.now() - network.lastOk > network.idleMs) await warmUp()
  const delays = safe ? network.retryDelaysMs : []
  for (let attempt = 0; ; attempt++) {
    try {
      const res = await fetch(url, init)
      network.lastOk = Date.now()
      return res
    } catch (e) {
      if ((e as Error)?.name === 'AbortError') throw e
      if (attempt >= delays.length) throw networkError()
      await sleep(delays[attempt])
    }
  }
}

/** Opens a fresh, working connection before a change; failures are ignored. */
async function warmUp() {
  try {
    await send('/api/health', { cache: 'no-store' })
  } catch {
    // the change below reports the problem
  }
}

function authHeaders(): Record<string, string> {
  return token ? { Authorization: `Bearer ${token}` } : {}
}

async function request<T>(method: string, path: string, body?: unknown, query?: Query): Promise<T> {
  const headers = authHeaders()
  let payload: BodyInit | undefined
  if (body instanceof FormData) payload = body
  else if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    payload = JSON.stringify(body)
  }
  const res = await send(`/api${path}${qs(query)}`, { method, headers, body: payload })
  if (res.status === 204) return undefined as T
  const text = await res.text()
  let data: unknown
  try {
    data = text ? JSON.parse(text) : undefined
  } catch {
    data = undefined
  }
  if (!res.ok) {
    if (res.status === 401 && token && onUnauthorized) onUnauthorized()
    throw new ApiError(res.status, (data as object) || { message: `שגיאת שרת (${res.status})` })
  }
  return data as T
}

export const api = {
  get: <T>(path: string, query?: Query) => request<T>('GET', path, undefined, query),
  post: <T>(path: string, body?: unknown, query?: Query) => request<T>('POST', path, body, query),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  del: <T = void>(path: string) => request<T>('DELETE', path),
}

export const publicFileUrl = (id: string) => `/api/public/files/${id}`

/** Opens a private attachment (fetched with the auth header) in a new tab. */
export async function openFile(id: string) {
  const res = await send(`/api/files/${id}`, { headers: authHeaders() })
  if (!res.ok) throw new ApiError(res.status, { message: 'לא ניתן לפתוח את הקובץ' })
  const url = URL.createObjectURL(await res.blob())
  window.open(url, '_blank', 'noopener')
  setTimeout(() => URL.revokeObjectURL(url), 60_000)
}

/** Fetch a generated file (e.g. an Excel export) with the session and save it under the server's name. */
export async function download(path: string, query: Query | undefined, fallbackName: string) {
  const res = await send(`/api${path}${qs(query)}`, { headers: authHeaders() })
  if (!res.ok) {
    let body: object | undefined
    try {
      body = await res.json()
    } catch {
      body = undefined
    }
    if (res.status === 401 && token && onUnauthorized) onUnauthorized()
    throw new ApiError(res.status, body || { message: `שגיאת שרת (${res.status})` })
  }
  const m = /filename\*=UTF-8''([^;]+)/i.exec(res.headers.get('Content-Disposition') ?? '')
  const name = m ? decodeURIComponent(m[1]) : fallbackName
  const url = URL.createObjectURL(await res.blob())
  const a = document.createElement('a')
  a.href = url
  a.download = name
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 60_000)
}

export function errorMessage(e: unknown): string {
  if (e instanceof ApiError) {
    const first = Object.values(e.fields)[0]
    return first ? `${e.message}: ${first}` : e.message
  }
  return e instanceof Error ? e.message : 'אירעה שגיאה'
}
