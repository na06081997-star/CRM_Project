import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import type { ReactElement } from 'react'
import { MemoryRouter } from 'react-router-dom'
import { vi } from 'vitest'
import { setToken } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import type { Role, User } from '@/lib/types'

/** Fake API: path (without /api and query string) → JSON body, or a function of the request. */
export type ApiRoutes = Record<string, unknown | ((url: URL, init?: RequestInit) => unknown)>

/** Every API call not listed fails the test loudly instead of hanging. */
export function mockApi(routes: ApiRoutes) {
  const calls: string[] = []
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input), 'http://localhost')
    const path = url.pathname.replace(/^\/api/, '')
    const key = `${(init?.method ?? 'GET').toUpperCase()} ${path}`
    calls.push(key)
    const hit = key in routes ? routes[key] : path in routes ? routes[path] : undefined
    if (hit === undefined) return new Response(JSON.stringify({ error: 'not_mocked', message: key }), { status: 599 })
    const body = typeof hit === 'function' ? (hit as (u: URL, i?: RequestInit) => unknown)(url, init) : hit
    return new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })
  })
  vi.stubGlobal('fetch', fetchMock)
  return { fetchMock, calls }
}

const allPermissions = [
  'dashboard:view', 'calendar:view', 'availability:view', 'bookings:manage', 'requests:review', 'requests:submit',
  'rooms:manage', 'payments:manage', 'payments:view', 'customers:manage', 'users:manage', 'coordinators:manage',
  'settings:manage', 'tasks:manage', 'signage:manage', 'activities:manage', 'reports:view', 'audit:view', 'files:upload',
]

export function testUser(role: Role = 'super_admin', over: Partial<User> = {}): User {
  return {
    id: 'u-1', username: 'tester', email: 'tester@kehila-test.co.il', fullName: 'בודק', phone: null, role, staffKind: null,
    avatarColor: '#0f766e', isActive: true, createdAt: '2026-01-01T00:00:00Z', status: 'active', hasPassword: true,
    activatedAt: '2026-01-01T00:00:00Z', firstLoginAt: null, lastLoginAt: null, loginHistoryKnown: true, ...over,
  }
}

/** Renders a page as a signed-in user, with fresh query cache and router. */
export function renderPage(ui: ReactElement, { route = '/', role = 'super_admin' as Role, permissions = allPermissions } = {}) {
  setToken('test-token')
  useAuth.setState({ ready: true, session: { token: 'test-token', user: testUser(role), permissions }, config: null })
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[route]}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  )
}
