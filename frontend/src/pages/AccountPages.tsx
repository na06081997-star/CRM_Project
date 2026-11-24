import { useQuery } from '@tanstack/react-query'
import { CheckCircle2, FileWarning, MailCheck } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { AuthShell, CodeStep, NewPasswordFields, useHashToken } from '@/components/AuthForms'
import { Button, Field, Input, PageLoader } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { homeFor, useAuth } from '@/lib/auth'
import { dateTime } from '@/lib/format'
import type { Challenge, LinkInfo } from '@/lib/types'

function Notice({ icon, title, text, ok, action }: { icon: ReactNode; title: string; text: ReactNode; ok?: boolean; action?: ReactNode }) {
  return (
    <div className="text-center">
      <div className={`mx-auto mb-3 flex h-14 w-14 items-center justify-center rounded-full ${ok ? 'bg-emerald-50 text-emerald-600 dark:bg-emerald-950/40' : 'bg-amber-50 text-amber-600 dark:bg-amber-950/40'}`}>{icon}</div>
      <p className="font-semibold">{title}</p>
      <div className="mt-1 text-sm text-muted">{text}</div>
      {action && <div className="mt-5">{action}</div>}
    </div>
  )
}

const toLogin = (
  <Link to="/login">
    <Button className="w-full">למסך הכניסה</Button>
  </Link>
)

/** Explains a link that can no longer be used. */
function LinkProblem({ info, purpose }: { info: LinkInfo | null; purpose: 'invite' | 'reset' }) {
  const ask = purpose === 'invite' ? 'פנו למנהל המערכת וישלח לכם הזמנה חדשה.' : 'בקשו קישור חדש ב"שכחתי סיסמה".'
  const again =
    purpose === 'reset' ? (
      <Link to="/forgot">
        <Button className="w-full">קישור חדש</Button>
      </Link>
    ) : (
      toLogin
    )
  switch (info?.status) {
    case 'used':
      return <Notice icon={<CheckCircle2 className="h-7 w-7" />} title="הקישור כבר נוצל" text="הסיסמה כבר נקבעה – התחברו עם שם המשתמש והסיסמה." action={toLogin} />
    case 'expired':
      return <Notice icon={<FileWarning className="h-7 w-7" />} title="תוקף הקישור פג" text={ask} action={again} />
    case 'replaced':
      return <Notice icon={<FileWarning className="h-7 w-7" />} title="נשלח קישור חדש יותר" text="השתמשו בקישור שבאימייל האחרון שקיבלתם." action={again} />
    case 'disabled':
      return <Notice icon={<FileWarning className="h-7 w-7" />} title="החשבון אינו פעיל" text="פנו למנהל המערכת." />
    default:
      return <Notice icon={<FileWarning className="h-7 w-7" />} title="הקישור אינו תקין" text={`ייתכן שהקישור הועתק באופן חלקי. ${ask}`} action={again} />
  }
}

function useLink(purpose: 'invite' | 'reset', token: string) {
  return useQuery({
    queryKey: ['auth-link', purpose, token],
    queryFn: () => api.post<LinkInfo>('/auth/link', { purpose, token }),
    enabled: !!token,
    retry: false,
    staleTime: Infinity,
  })
}

/** /activate#token — choose a password from the invitation, then the e-mail code. */
export function ActivatePage() {
  const token = useHashToken()
  const nav = useNavigate()
  const link = useLink('invite', token)
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [challenge, setChallenge] = useState<Challenge | null>(null)
  const info = link.data

  const done = () => {
    const s = useAuth.getState().session
    nav(s ? homeFor(s.user.role) : '/login', { replace: true })
  }

  if (challenge)
    return (
      <AuthShell title="אימות האימייל" subtitle="שלב אחרון: הקוד שקיבלתם באימייל.">
        <CodeStep challenge={challenge} onDone={done} onRestart={() => nav('/login', { replace: true })} />
      </AuthShell>
    )

  return (
    <AuthShell title="הפעלת החשבון" subtitle={info?.status === 'valid' ? <>שלום {info.fullName}, בחרו סיסמה לחשבון.</> : undefined}>
      {!token || link.isError ? (
        <LinkProblem info={null} purpose="invite" />
      ) : link.isLoading || !info ? (
        <PageLoader />
      ) : info.status !== 'valid' ? (
        <LinkProblem info={info} purpose="invite" />
      ) : (
        <form
          className="space-y-4"
          onSubmit={async (e) => {
            e.preventDefault()
            setBusy(true)
            setErrors({})
            setError('')
            try {
              setChallenge(await api.post<Challenge>('/auth/activate', { token, password, passwordConfirm: confirm }))
            } catch (err) {
              if (err instanceof ApiError && Object.keys(err.fields).length) setErrors(err.fields)
              else {
                setError(errorMessage(err))
                link.refetch()
              }
            } finally {
              setBusy(false)
            }
          }}
        >
          <div className="rounded-xl bg-subtle p-3 text-sm">
            <p>
              שם המשתמש שלך: <b dir="ltr">{info.username}</b>
            </p>
            <p className="mt-1 text-xs text-muted">
              איתו (או עם האימייל <span dir="ltr">{info.email}</span>) תתחברו בכל פעם. הקישור בתוקף עד {dateTime(info.expiresAt)}.
            </p>
          </div>
          <NewPasswordFields password={password} confirm={confirm} onPassword={setPassword} onConfirm={setConfirm} errors={errors} username={info.username} />
          {error && <p className="rounded-xl bg-rose-50 p-3 text-sm text-rose-700 dark:bg-rose-950/40 dark:text-rose-200">{error}</p>}
          <Button type="submit" size="lg" className="w-full" loading={busy}>
            שמירת הסיסמה ושליחת קוד
          </Button>
          <p className="text-xs text-muted">לאחר השמירה יישלח קוד אימות בן 6 ספרות לאימייל שלכם.</p>
        </form>
      )}
    </AuthShell>
  )
}

/** /forgot — always the same answer, whether the account exists or not. */
export function ForgotPage() {
  const [identifier, setIdentifier] = useState('')
  const [sent, setSent] = useState<string | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  return (
    <AuthShell title="שכחתי סיסמה" subtitle={sent ? undefined : 'נשלח אליכם קישור לבחירת סיסמה חדשה.'}>
      {sent ? (
        <Notice ok icon={<MailCheck className="h-7 w-7" />} title="בדקו את תיבת הדואר" text={sent} action={toLogin} />
      ) : (
        <form
          className="space-y-4"
          onSubmit={async (e) => {
            e.preventDefault()
            setBusy(true)
            setError('')
            try {
              const res = await api.post<{ message: string }>('/auth/forgot', { identifier: identifier.trim() })
              setSent(res.message)
            } catch (err) {
              setError(errorMessage(err))
            } finally {
              setBusy(false)
            }
          }}
        >
          <Field label="שם משתמש או אימייל">
            <Input dir="ltr" autoComplete="username" autoCapitalize="none" autoFocus value={identifier} onChange={(e) => setIdentifier(e.target.value)} />
          </Field>
          {error && <p className="rounded-xl bg-rose-50 p-3 text-sm text-rose-700 dark:bg-rose-950/40 dark:text-rose-200">{error}</p>}
          <Button type="submit" size="lg" className="w-full" loading={busy} disabled={!identifier.trim()}>
            שליחת קישור
          </Button>
          <div className="text-center text-sm">
            <Link to="/login" className="text-muted hover:text-ink">
              חזרה לכניסה
            </Link>
          </div>
        </form>
      )}
    </AuthShell>
  )
}

/** /reset#token — new password; every session ends, next login asks for a code. */
export function ResetPage() {
  const token = useHashToken()
  const link = useLink('reset', token)
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState(false)
  const logout = useAuth((s) => s.logout)
  const info = link.data

  return (
    <AuthShell title="בחירת סיסמה חדשה">
      {done ? (
        <Notice ok icon={<CheckCircle2 className="h-7 w-7" />} title="הסיסמה עודכנה" text="כל החיבורים הקודמים נותקו. התחברו עם הסיסמה החדשה – יישלח אליכם קוד אימות." action={toLogin} />
      ) : !token || link.isError ? (
        <LinkProblem info={null} purpose="reset" />
      ) : link.isLoading || !info ? (
        <PageLoader />
      ) : info.status !== 'valid' ? (
        <LinkProblem info={info} purpose="reset" />
      ) : (
        <form
          className="space-y-4"
          onSubmit={async (e) => {
            e.preventDefault()
            setBusy(true)
            setErrors({})
            setError('')
            try {
              await api.post('/auth/reset', { token, password, passwordConfirm: confirm })
              logout()
              setDone(true)
            } catch (err) {
              if (err instanceof ApiError && Object.keys(err.fields).length) setErrors(err.fields)
              else {
                setError(errorMessage(err))
                link.refetch()
              }
            } finally {
              setBusy(false)
            }
          }}
        >
          <p className="text-sm text-muted">
            חשבון: <b dir="ltr">{info.username}</b>
          </p>
          <NewPasswordFields password={password} confirm={confirm} onPassword={setPassword} onConfirm={setConfirm} errors={errors} username={info.username} />
          {error && <p className="rounded-xl bg-rose-50 p-3 text-sm text-rose-700 dark:bg-rose-950/40 dark:text-rose-200">{error}</p>}
          <Button type="submit" size="lg" className="w-full" loading={busy}>
            שמירת הסיסמה
          </Button>
        </form>
      )}
    </AuthShell>
  )
}
