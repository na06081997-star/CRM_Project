import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Monitor } from 'lucide-react'
import { useState } from 'react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom'
import { PasswordLogin } from '@/components/AuthForms'
import { Logo, PoweredBy, ProductMark } from '@/components/layout/Logo'
import { toast } from '@/components/toast'
import { Avatar, Button, Input, Spinner } from '@/components/ui'
import { api, errorMessage } from '@/lib/api'
import { homeFor, useAuth } from '@/lib/auth'
import { PRODUCT, useOrg } from '@/lib/brand'
import { roleLabel, staffKindLabel } from '@/lib/labels'
import type { Role, User } from '@/lib/types'

const ORDER: Role[] = ['super_admin', 'admin', 'coordinator', 'staff', 'user']

export function LoginPage() {
  const { session, config, devLogin } = useAuth()
  const org = useOrg()
  const nav = useNavigate()
  const loc = useLocation()
  const [busy, setBusy] = useState<string | null>(null)
  const dev = config?.authMode === 'dev'
  const needCode = dev && !!config?.demoCodeRequired
  // Public demo instances guard the user picker with an access code.
  const [code, setCode] = useState(() => readDemoCode())
  const [codeInput, setCodeInput] = useState('')
  const users = useQuery({
    queryKey: ['dev-users', code],
    queryFn: () => api.get<User[]>('/auth/dev-users', { code: code || undefined }),
    enabled: dev && (!needCode || !!code),
    retry: false,
  })
  const codeRejected = needCode && !!code && users.isError

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

          {config?.authMode === 'password' && (
            <div className="mt-8">
              <PasswordLogin onDone={after} />
            </div>
          )}

          {dev && (
            <div className="mt-8">
              <div className="mb-3 rounded-xl border border-amber-300/50 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950/40 dark:text-amber-200">
                מצב הדגמה: בחרו משתמש לכניסה, ללא סיסמה.
              </div>
              {needCode && (!code || codeRejected) && (
                <form
                  className="mb-4 flex items-end gap-2"
                  onSubmit={(e) => {
                    e.preventDefault()
                    const c = codeInput.trim()
                    if (!c) return
                    saveDemoCode(c)
                    setCode(c)
                  }}
                >
                  <label className="block flex-1">
                    <span className="label">קוד גישה לסביבת ההדגמה</span>
                    <Input type="password" autoFocus value={codeInput} onChange={(e) => setCodeInput(e.target.value)} />
                    {codeRejected && <span className="mt-1 block text-xs text-danger">קוד שגוי</span>}
                  </label>
                  <Button type="submit">כניסה</Button>
                </form>
              )}
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
                                await devLogin(u.email, code || undefined)
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

const DEMO_CODE_KEY = 'crm.demoCode'

function readDemoCode() {
  try {
    return localStorage.getItem(DEMO_CODE_KEY) ?? ''
  } catch {
    return ''
  }
}

function saveDemoCode(code: string) {
  try {
    localStorage.setItem(DEMO_CODE_KEY, code)
  } catch {
    /* private mode: the code then lasts for this page only */
  }
}
