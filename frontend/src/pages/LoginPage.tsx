import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Monitor } from 'lucide-react'
import { useState } from 'react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom'
import { Logo, PoweredBy, ProductMark } from '@/components/layout/Logo'
import { toast } from '@/components/toast'
import { Avatar, Button, Spinner } from '@/components/ui'
import { api, errorMessage } from '@/lib/api'
import { homeFor, useAuth } from '@/lib/auth'
import { PRODUCT, useOrg } from '@/lib/brand'
import { roleLabel, staffKindLabel } from '@/lib/labels'
import type { Role, User } from '@/lib/types'

const ORDER: Role[] = ['super_admin', 'admin', 'coordinator', 'staff', 'user']

export function LoginPage() {
  const { session, config, devLogin, googleLogin } = useAuth()
  const org = useOrg()
  const nav = useNavigate()
  const loc = useLocation()
  const [busy, setBusy] = useState<string | null>(null)
  const dev = config?.authMode === 'dev'
  const users = useQuery({ queryKey: ['dev-users'], queryFn: () => api.get<User[]>('/auth/dev-users'), enabled: dev })

  if (session) return <Navigate to={(loc.state as { from?: string })?.from ?? homeFor(session.user.role)} replace />

  const after = () => {
    const s = useAuth.getState().session
    if (s) nav((loc.state as { from?: string })?.from ?? homeFor(s.user.role), { replace: true })
  }

  return (
    <div className="flex min-h-full">
      {/* Brand panel */}
      <div
        className="relative hidden w-[46%] overflow-hidden bg-gradient-to-br from-teal-700 via-teal-800 to-slate-900 p-12 text-white lg:flex lg:flex-col"
        style={org?.brandColor ? { backgroundImage: `linear-gradient(135deg, ${org.brandColor}, #0f172a 85%)` } : undefined}
      >
        <div className="absolute -start-24 -top-24 h-96 w-96 rounded-full bg-teal-400/20 blur-3xl" />
        <div className="absolute -bottom-32 -end-16 h-96 w-96 rounded-full bg-violet-500/20 blur-3xl" />
        <div className="relative flex items-center gap-4">
          {org?.logoUrl || org?.markUrl ? (
            <span className="flex h-28 w-28 shrink-0 items-center justify-center rounded-3xl bg-white p-2 shadow-xl shadow-black/20">
              <img src={org.logoUrl || org.markUrl} alt={org.name} className="h-full w-full object-contain" />
            </span>
          ) : (
            <ProductMark className="h-16 w-16" />
          )}
          <div>
            <p className="text-3xl font-extrabold leading-tight">{org ? org.name : PRODUCT.name}</p>
            <p className="text-teal-100/80">{org ? 'מערכת ניהול חללים, הזמנות ופעילויות' : PRODUCT.tagline}</p>
          </div>
        </div>
        <div className="relative mt-auto max-w-md space-y-6">
          <h1 className="text-4xl font-bold leading-tight">כל החדרים, ההזמנות והפעילויות – במקום אחד.</h1>
          <ul className="space-y-3 text-teal-50/90">
            <li>📅 יומן הזמנות חכם עם מניעת התנגשויות</li>
            <li>✅ אישור בקשות ותמחור אוטומטי</li>
            <li>💳 מעקב תשלומים ויתרות</li>
            <li>📺 שילוט דיגיטלי בזמן אמת</li>
          </ul>
          <Link to="/signage" className="inline-flex items-center gap-2 rounded-xl bg-white/10 px-4 py-2 text-sm backdrop-blur hover:bg-white/20">
            <Monitor className="h-4 w-4" /> צפייה בשילוט הציבורי
          </Link>
        </div>
        {org && (
          <p className="relative mt-10 flex items-center gap-2 text-xs text-white/60">
            <ProductMark className="h-5 w-5" /> מופעל על ידי <b className="font-semibold text-white/85">{PRODUCT.name}</b> · {PRODUCT.tagline}
          </p>
        )}
      </div>

      {/* Login panel */}
      <div className="flex flex-1 items-center justify-center p-6">
        <div className="w-full max-w-md">
          <div className="mb-8 lg:hidden">
            <Logo />
          </div>
          <h2 className="text-2xl font-bold">ברוכים הבאים</h2>
          <p className="mt-1 text-sm text-muted">התחברו כדי לנהל הזמנות, להגיש בקשות או לצפות במשימות.</p>

          {config?.authMode === 'firebase' && (
            <Button
              size="lg"
              variant="secondary"
              className="mt-8 w-full"
              loading={busy === 'google'}
              onClick={async () => {
                setBusy('google')
                try {
                  await googleLogin()
                  after()
                } catch (e) {
                  toast.error(errorMessage(e))
                } finally {
                  setBusy(null)
                }
              }}
            >
              <GoogleIcon /> התחברות עם Google
            </Button>
          )}

          {dev && (
            <div className="mt-8">
              <div className="mb-3 rounded-xl border border-amber-300/50 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950/40 dark:text-amber-200">
                מצב הדגמה: בחרו משתמש לכניסה. בהפעלת Firebase יופיע כאן כפתור התחברות עם Google.
              </div>
              {users.isLoading && <Spinner />}
              <div className="space-y-4">
                {ORDER.map((role) => {
                  const list = (users.data ?? []).filter((u) => u.role === role)
                  if (!list.length) return null
                  return (
                    <div key={role}>
                      <p className="mb-1.5 text-xs font-semibold text-muted">{roleLabel[role]}</p>
                      <div className="grid gap-1.5">
                        {list.map((u) => (
                          <button
                            key={u.id}
                            disabled={!!busy}
                            onClick={async () => {
                              setBusy(u.id)
                              try {
                                await devLogin(u.email)
                                after()
                              } catch (e) {
                                toast.error(errorMessage(e))
                              } finally {
                                setBusy(null)
                              }
                            }}
                            className="group flex items-center gap-3 rounded-xl border border-line bg-surface px-3 py-2.5 text-start transition hover:border-brand/50 hover:shadow-card"
                          >
                            <Avatar name={u.fullName} color={u.avatarColor} size={34} />
                            <span className="min-w-0 flex-1">
                              <span className="block text-sm font-medium">{u.fullName}</span>
                              <span className="block truncate text-xs text-muted">
                                {u.email}
                                {u.staffKind ? ` · ${staffKindLabel[u.staffKind] ?? u.staffKind}` : ''}
                              </span>
                            </span>
                            {busy === u.id ? <Spinner className="h-4 w-4" /> : <ArrowLeft className="h-4 w-4 text-muted opacity-0 transition group-hover:opacity-100" />}
                          </button>
                        ))}
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          )}
          {!config && <p className="mt-8 text-sm text-danger">השרת אינו זמין כרגע. נסו לרענן בעוד רגע.</p>}
          <PoweredBy className="mt-10 justify-center lg:hidden" />
        </div>
      </div>
    </div>
  )
}

function GoogleIcon() {
  return (
    <svg viewBox="0 0 48 48" className="h-5 w-5" aria-hidden>
      <path fill="#FFC107" d="M43.6 20.5H42V20H24v8h11.3C33.7 32.7 29.2 36 24 36c-6.6 0-12-5.4-12-12s5.4-12 12-12c3.1 0 5.8 1.2 7.9 3.1l5.7-5.7C34 6.1 29.3 4 24 4 12.9 4 4 12.9 4 24s8.9 20 20 20 20-8.9 20-20c0-1.3-.1-2.4-.4-3.5z" />
      <path fill="#FF3D00" d="m6.3 14.7 6.6 4.8C14.7 15.1 19 12 24 12c3.1 0 5.8 1.2 7.9 3.1l5.7-5.7C34 6.1 29.3 4 24 4 16.3 4 9.7 8.3 6.3 14.7z" />
      <path fill="#4CAF50" d="M24 44c5.2 0 9.9-2 13.4-5.2l-6.2-5.2C29.2 35.1 26.7 36 24 36c-5.2 0-9.6-3.3-11.3-8l-6.5 5C9.5 39.6 16.2 44 24 44z" />
      <path fill="#1976D2" d="M43.6 20.5H42V20H24v8h11.3c-.8 2.2-2.2 4.2-4.1 5.6l6.2 5.2C37 39.2 44 34 44 24c0-1.3-.1-2.4-.4-3.5z" />
    </svg>
  )
}
