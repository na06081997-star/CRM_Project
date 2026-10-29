import { create } from 'zustand'
import { api, setToken, setUnauthorizedHandler } from './api'
import type { Me, PublicConfig, Role, User } from './types'

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
  devLogin: (email: string) => Promise<void>
  googleLogin: () => Promise<void>
  logout: () => Promise<void>
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

// Firebase is loaded lazily so dev-mode bundles never download it.
async function firebaseAuth(cfg: PublicConfig) {
  const [{ initializeApp, getApps }, authMod] = await Promise.all([import('firebase/app'), import('firebase/auth')])
  const app = getApps()[0] ?? initializeApp(cfg.firebase)
  return { auth: authMod.getAuth(app), mod: authMod }
}

export const useAuth = create<AuthState>((set, get) => ({
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
      // Firebase ID tokens live one hour: refresh silently on boot.
      if (config?.authMode === 'firebase') {
        try {
          const { auth } = await firebaseAuth(config)
          await auth.authStateReady()
          if (auth.currentUser) {
            const fresh = await auth.currentUser.getIdToken()
            setToken(fresh)
            set({ session: { ...saved, token: fresh } })
            persist({ ...saved, token: fresh })
          }
        } catch {
          /* fall through: /me below decides */
        }
      }
      await get().refreshMe().catch(() => undefined)
    }
    set({ ready: true })
  },

  async devLogin(email) {
    const res = await api.post<Me & { token: string }>('/auth/dev-login', { email })
    const session = { token: res.token, user: res.user, permissions: res.permissions }
    setToken(res.token)
    persist(session)
    set({ session })
  },

  async googleLogin() {
    const cfg = get().config
    if (!cfg) throw new Error('תצורה לא זמינה')
    const { auth, mod } = await firebaseAuth(cfg)
    const cred = await mod.signInWithPopup(auth, new mod.GoogleAuthProvider())
    const idToken = await cred.user.getIdToken()
    setToken(idToken)
    const me = await api.get<Me>('/me')
    const session = { token: idToken, user: me.user, permissions: me.permissions }
    persist(session)
    set({ session })
    // Keep the stored token fresh while the tab is open.
    mod.onIdTokenChanged(auth, async (u) => {
      if (!u) return
      const t = await u.getIdToken()
      setToken(t)
      const cur = get().session
      if (cur) {
        const next = { ...cur, token: t }
        persist(next)
        set({ session: next })
      }
    })
  },

  async logout() {
    const cfg = get().config
    if (cfg?.authMode === 'firebase') {
      const { auth, mod } = await firebaseAuth(cfg)
      await mod.signOut(auth).catch(() => undefined)
    }
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
}))

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
