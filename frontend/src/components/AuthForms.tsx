import clsx from 'clsx'
import { Check, Eye, EyeOff, MailCheck, RotateCcw } from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { Button, Field, Input } from '@/components/ui'
import { OrgMark, PoweredBy } from '@/components/layout/Logo'
import { api, ApiError, errorMessage } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { PRODUCT, useOrg } from '@/lib/brand'
import type { Challenge } from '@/lib/types'

/** Centered card used by the activation, reset and forgot-password pages. */
export function AuthShell({ title, subtitle, children }: { title: string; subtitle?: ReactNode; children: ReactNode }) {
  const org = useOrg()
  return (
    <div className="flex min-h-full flex-col items-center bg-subtle/60 px-4 py-8 sm:justify-center">
      <div className="w-full max-w-md">
        <div className="mb-6 flex items-center gap-3">
          {org && (org.markUrl || org.logoUrl) && <OrgMark src={org.markUrl || org.logoUrl} name={org.name} />}
          <div className="min-w-0">
            <p className="truncate font-semibold">{org?.name ?? PRODUCT.name}</p>
            <p className="text-xs text-muted">{PRODUCT.name}</p>
          </div>
        </div>
        <div className="card p-5 sm:p-6">
          <h1 className="text-xl font-bold">{title}</h1>
          {subtitle && <div className="mt-1 text-sm text-muted">{subtitle}</div>}
          <div className="mt-5">{children}</div>
        </div>
        <PoweredBy className="mt-8 justify-center" />
      </div>
    </div>
  )
}

export function PasswordInput({ value, onChange, autoComplete, autoFocus, id }: { value: string; onChange: (v: string) => void; autoComplete: string; autoFocus?: boolean; id?: string }) {
  const [show, setShow] = useState(false)
  return (
    <div className="relative">
      <Input
        id={id}
        dir="ltr"
        type={show ? 'text' : 'password'}
        autoComplete={autoComplete}
        autoFocus={autoFocus}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="pl-11"
      />
      <button
        type="button"
        onClick={() => setShow((s) => !s)}
        className="absolute inset-y-0 left-0 flex w-11 items-center justify-center text-muted hover:text-ink"
        aria-label={show ? 'הסתרת הסיסמה' : 'הצגת הסיסמה'}
      >
        {show ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
      </button>
    </div>
  )
}

/** The rules the server enforces, shown live. */
function rules(pw: string, username?: string) {
  const lower = pw.toLowerCase()
  return [
    { ok: [...pw].length >= 10, label: 'לפחות 10 תווים' },
    { ok: /\p{L}/u.test(pw) && /\d/.test(pw), label: 'אותיות וספרות' },
    { ok: !username || username.length < 3 || !lower.includes(username.toLowerCase()), label: 'בלי שם המשתמש' },
  ]
}

export function NewPasswordFields({
  password,
  confirm,
  onPassword,
  onConfirm,
  errors,
  username,
}: {
  password: string
  confirm: string
  onPassword: (v: string) => void
  onConfirm: (v: string) => void
  errors: Record<string, string>
  username?: string
}) {
  return (
    <div className="space-y-4">
      <Field label="סיסמה חדשה" error={errors.password}>
        <PasswordInput value={password} onChange={onPassword} autoComplete="new-password" autoFocus />
      </Field>
      <ul className="grid grid-cols-1 gap-1 text-xs sm:grid-cols-3">
        {rules(password, username).map((r) => (
          <li key={r.label} className={clsx('flex items-center gap-1.5', r.ok ? 'text-emerald-600 dark:text-emerald-400' : 'text-muted')}>
            <Check className={clsx('h-3.5 w-3.5', !r.ok && 'opacity-30')} /> {r.label}
          </li>
        ))}
      </ul>
      <Field label="אימות סיסמה" error={errors.passwordConfirm}>
        <PasswordInput value={confirm} onChange={onConfirm} autoComplete="new-password" />
      </Field>
    </div>
  )
}

function useCountdown(seconds: number) {
  const [left, setLeft] = useState(seconds)
  useEffect(() => setLeft(seconds), [seconds])
  useEffect(() => {
    if (left <= 0) return
    const t = setTimeout(() => setLeft((l) => l - 1), 1000)
    return () => clearTimeout(t)
  }, [left])
  return [left, setLeft] as const
}

/** Second step of every login: the 6-digit code sent by e-mail. */
export function CodeStep({ challenge, onDone, onRestart }: { challenge: Challenge; onDone: () => void; onRestart: () => void }) {
  const verifyCode = useAuth((s) => s.verifyCode)
  const [ch, setCh] = useState(challenge)
  const [code, setCode] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [resending, setResending] = useState(false)
  const [left, setLeft] = useCountdown(ch.sent ? ch.resendIn : 0)
  const input = useRef<HTMLInputElement>(null)
  const expired = useRef(false)

  const submit = async (value = code) => {
    if (value.length !== 6 || busy) return
    setBusy(true)
    setError('')
    try {
      await verifyCode(ch.challenge, value)
      onDone()
    } catch (e) {
      const err = e as ApiError
      setError(errorMessage(e))
      if (err.code === 'challenge_expired') expired.current = true
      setCode('')
      input.current?.focus()
    } finally {
      setBusy(false)
    }
  }

  const resend = async () => {
    setResending(true)
    setError('')
    try {
      const next = await api.post<Challenge>('/auth/resend', { challenge: ch.challenge })
      setCh(next)
      setLeft(next.sent ? next.resendIn : 0)
      setCode('')
      input.current?.focus()
    } catch (e) {
      const err = e as ApiError
      if (err.retryAfter) setLeft(err.retryAfter)
      if (err.code === 'challenge_expired') expired.current = true
      setError(errorMessage(e))
    } finally {
      setResending(false)
    }
  }

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        submit()
      }}
      className="space-y-4"
    >
      {ch.sent ? (
        <div className="flex gap-3 rounded-xl bg-brand-soft/50 p-3 text-sm">
          <MailCheck className="mt-0.5 h-5 w-5 shrink-0 text-brand" />
          <p>
            שלחנו קוד בן 6 ספרות לכתובת <b dir="ltr">{ch.email}</b>. הקוד בתוקף {Math.round(ch.codeTtl / 60)} דקות.
          </p>
        </div>
      ) : (
        <p className="rounded-xl bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-950/40 dark:text-amber-200">
          שליחת האימייל נכשלה. לחצו על "שליחת קוד חדש" כדי לנסות שוב.
        </p>
      )}
      <Field label="קוד אימות" error={error}>
        <Input
          ref={input}
          dir="ltr"
          inputMode="numeric"
          autoComplete="one-time-code"
          pattern="[0-9]*"
          maxLength={6}
          autoFocus
          value={code}
          onChange={(e) => {
            const v = e.target.value.replace(/\D/g, '').slice(0, 6)
            setCode(v)
            if (v.length === 6) submit(v)
          }}
          className="h-14 text-center font-mono text-2xl tracking-[0.5em]"
          aria-label="קוד אימות בן 6 ספרות"
        />
      </Field>
      <Button type="submit" size="lg" className="w-full" loading={busy} disabled={code.length !== 6}>
        כניסה
      </Button>
      <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
        {expired.current ? (
          <button type="button" onClick={onRestart} className="font-medium text-brand hover:underline">
            התחברות מחדש
          </button>
        ) : (
          <button type="button" onClick={resend} disabled={left > 0 || resending} className="inline-flex items-center gap-1.5 font-medium text-brand hover:underline disabled:text-muted disabled:no-underline">
            <RotateCcw className="h-4 w-4" />
            {left > 0 ? `שליחת קוד חדש בעוד ${left} שנ׳` : 'שליחת קוד חדש'}
          </button>
        )}
        <button type="button" onClick={onRestart} className="text-muted hover:text-ink">
          חזרה
        </button>
      </div>
      <p className="text-xs text-muted">לא מצאתם את האימייל? בדקו גם בתיקיית הספאם / קידומי מכירות.</p>
    </form>
  )
}

/** Identifier + password, then the code step. */
export function PasswordLogin({ onDone }: { onDone: () => void }) {
  const passwordLogin = useAuth((s) => s.passwordLogin)
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [challenge, setChallenge] = useState<Challenge | null>(null)

  if (challenge) return <CodeStep challenge={challenge} onDone={onDone} onRestart={() => setChallenge(null)} />

  return (
    <form
      className="space-y-4"
      onSubmit={async (e) => {
        e.preventDefault()
        setBusy(true)
        setError('')
        setErrors({})
        try {
          setChallenge(await passwordLogin(identifier.trim(), password))
          setPassword('')
        } catch (err) {
          if (err instanceof ApiError && Object.keys(err.fields).length) setErrors(err.fields)
          else setError(errorMessage(err))
        } finally {
          setBusy(false)
        }
      }}
    >
      <Field label="שם משתמש או אימייל" error={errors.identifier}>
        <Input dir="ltr" autoComplete="username" autoCapitalize="none" spellCheck={false} autoFocus value={identifier} onChange={(e) => setIdentifier(e.target.value)} />
      </Field>
      <Field label="סיסמה" error={errors.password}>
        <PasswordInput value={password} onChange={setPassword} autoComplete="current-password" />
      </Field>
      {error && <p className="rounded-xl bg-rose-50 p-3 text-sm text-rose-700 dark:bg-rose-950/40 dark:text-rose-200">{error}</p>}
      <Button type="submit" size="lg" className="w-full" loading={busy}>
        המשך
      </Button>
      <div className="text-center text-sm">
        <Link to="/forgot" className="text-brand hover:underline">
          שכחתי סיסמה
        </Link>
      </div>
    </form>
  )
}

/** Reads the secret token from the URL fragment (#…): fragments never reach servers or their logs. */
export function useHashToken() {
  const key = 'crm.link' + window.location.pathname
  const [token] = useState(() => {
    const fromHash = window.location.hash.replace(/^#/, '').trim()
    try {
      if (fromHash) sessionStorage.setItem(key, fromHash)
      return fromHash || sessionStorage.getItem(key) || ''
    } catch {
      return fromHash
    }
  })
  useEffect(() => {
    // Drop it from the address bar (and history) once read; a reload of this tab still works.
    if (window.location.hash) window.history.replaceState(null, '', window.location.pathname + window.location.search)
  }, [])
  return token
}
