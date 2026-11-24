import { create } from 'zustand'
import { api, setToken, setUnauthorizedHandler } from './api'
import type { Challenge, Me, PublicConfig, Role, SessionResponse, User } from './types'

const STORAGE_KEY = 'crm.session'

interface Session {
  token: string
  user: User
  permissions: string[]
}

interface AuthState {
  ready: boolean
  config: PublicConfig | null
  session: Session | null
  init: () => Promise<void>
  reloadConfig: () => Promise<void>
  /** Demo instances only: pick a user. */
  devLogin: (email: string, code?: string) => Promise<void>
  /** Step 1: identifier + password → a code is e-mailed. */
  passwordLogin: (identifier: string, password: string) => Promise<Challenge>
  /** Step 2: the e-mailed code opens the session. */
  verifyCode: (challenge: string, code: string) => Promise<void>
  logout: () => void
  refreshMe: () => Promise<void>
  setUser: (me: Me) => void
}

function persist(s: Session | null) {
  try {
    if (s) localStorage.setItem(STORAGE_KEY, JSON.stringify(s))
    else localStorage.removeItem(STORAGE_KEY)
  } catch {
    /* private mode */
  }
}

function load(): Session | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? (JSON.parse(raw) as Session) : null
  } catch {
    return null
  }
}

export const useAuth = create<AuthState>((set, get) => {
  const open = (res: SessionResponse) => {
    const session = { token: res.token, user: res.user, permissions: res.permissions }
    setToken(res.token)
    persist(session)
    set({ session })
  }
  return {
    ready: false,
    config: null,
    session: null,

    async init() {
      setUnauthorizedHandler(() => {
        persist(null)
        setToken(null)
        set({ session: null })
      })
      const config = await api.get<PublicConfig>('/public/config').catch(() => null)
      set({ config })
      const saved = load()
      if (saved) {
        setToken(saved.token)
        set({ session: saved })
        await get().refreshMe().catch(() => undefined)
      }
      set({ ready: true })
    },

    async reloadConfig() {
      const config = await api.get<PublicConfig>('/public/config').catch(() => null)
      if (config) set({ config })
    },

    async devLogin(email, code) {
      open(await api.post<SessionResponse>('/auth/dev-login', { email, code }))
    },

    passwordLogin(identifier, password) {
      return api.post<Challenge>('/auth/login', { identifier, password })
    },

    async verifyCode(challenge, code) {
      open(await api.post<SessionResponse>('/auth/verify', { challenge, code }))
    },

    logout() {
      persist(null)
      setToken(null)
      set({ session: null })
    },

    async refreshMe() {
      const me = await api.get<Me>('/me')
      get().setUser(me)
    },

    setUser(me) {
      const cur = get().session
      if (!cur) return
      const next = { ...cur, user: me.user, permissions: me.permissions }
      persist(next)
      set({ session: next })
    },
  }
})

export const useUser = () => useAuth((s) => s.session?.user ?? null)

export function useCan() {
  const perms = useAuth((s) => s.session?.permissions)
  return (p: string) => !!perms?.includes(p)
}

/** Where each role lands after login. */
export function homeFor(role: Role) {
  switch (role) {
    case 'super_admin':
    case 'admin':
      return '/admin'
    case 'staff':
      return '/staff'
    default:
      return '/portal'
  }
}
