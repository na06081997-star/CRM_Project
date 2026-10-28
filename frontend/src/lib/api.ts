import type { Conflict } from './types'

export class ApiError extends Error {
  status: number
  code: string
  fields: Record<string, string>
  conflicts: Conflict[]

  constructor(status: number, body: { error?: string; message?: string; fields?: Record<string, string>; conflicts?: Conflict[] }) {
    super(body.message || 'אירעה שגיאה')
    this.status = status
    this.code = body.error || 'unknown'
    this.fields = body.fields || {}
    this.conflicts = body.conflicts || []
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
  const res = await fetch(`/api${path}${qs(query)}`, { method, headers, body: payload })
  if (res.status === 204) return undefined as T
  const text = await res.text()
  let data: unknown
  try {
    data = text ? JSON.parse(text) : undefined
  } catch {
    data = undefined
  }
  if (!res.ok) {
    if (res.status === 401 && onUnauthorized) onUnauthorized()
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
  const res = await fetch(`/api/files/${id}`, { headers: authHeaders() })
  if (!res.ok) throw new ApiError(res.status, { message: 'לא ניתן לפתוח את הקובץ' })
  const url = URL.createObjectURL(await res.blob())
  window.open(url, '_blank', 'noopener')
  setTimeout(() => URL.revokeObjectURL(url), 60_000)
}

export function errorMessage(e: unknown): string {
  if (e instanceof ApiError) {
    const first = Object.values(e.fields)[0]
    return first ? `${e.message}: ${first}` : e.message
  }
  return e instanceof Error ? e.message : 'אירעה שגיאה'
}
