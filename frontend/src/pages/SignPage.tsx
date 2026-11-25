import { useMutation, useQuery } from '@tanstack/react-query'
import { CheckCircle2, Eraser, ExternalLink, FileWarning, Loader2, PenLine } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { OrgMark, PoweredBy } from '@/components/layout/Logo'
import { Button, Field, Input, PageLoader } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { useOrg } from '@/lib/brand'
import { dateTime } from '@/lib/format'
import type { PublicContract } from '@/lib/types'

/**
 * Public page behind the link sent by e-mail: read, sign, done.
 * With OpenSign (signUrl set) the signer reads the contract here and signs the
 * PDF in OpenSign, which sends them back with ?done=1; otherwise they sign with
 * a finger on this page.
 */
export function SignPage() {
  const { token = '' } = useParams()
  const [params] = useSearchParams()
  const back = params.get('done') === '1'
  const org = useOrg()
  const q = useQuery({
    queryKey: ['public-contract', token],
    queryFn: () => api.get<PublicContract>(`/public/contracts/${token}`),
    retry: false,
    // Back from OpenSign: poll until the signature is recorded.
    refetchInterval: (query) => (back && query.state.data?.status === 'pending' ? 3000 : false),
  })
  const [name, setName] = useState('')
  const [agree, setAgree] = useState(false)
  const [signature, setSignature] = useState<string | null>(null)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const c = q.data
  useEffect(() => {
    if (c?.name && !name) setName(c.name)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [c?.name])

  const sign = useMutation({
    mutationFn: () => api.post<PublicContract>(`/public/contracts/${token}/sign`, { name, agree, signature }),
    onSuccess: () => q.refetch(),
    onError: (e) => e instanceof ApiError && setErrors(e.fields),
  })

  return (
    <div className="min-h-full bg-subtle/60">
      <header className="border-b border-line bg-surface">
        <div className="mx-auto flex max-w-2xl items-center gap-3 px-4 py-3">
          {org && <OrgMark src={org.markUrl || org.logoUrl} name={org.name} />}
          <p className="font-semibold">{org?.name}</p>
        </div>
      </header>
      <main className="mx-auto max-w-2xl px-4 py-6 pb-16">
        {q.isLoading ? (
          <PageLoader />
        ) : q.isError || !c ? (
          <Notice icon={<FileWarning className="h-7 w-7" />} title="הקישור אינו תקין" text="ייתכן שהקישור הועתק באופן חלקי. פנו למתנ״ס לקבלת קישור חדש." />
        ) : c.status === 'signed' ? (
          <Notice
            ok
            icon={<CheckCircle2 className="h-7 w-7" />}
            title="החוזה נחתם, תודה!"
            text={`החשבון שלך במערכת נפתח${c.signedAt ? ` (נחתם ${dateTime(c.signedAt)})` : ''}. אם זו הכניסה הראשונה, נשלח אליך אימייל לבחירת סיסמה.`}
            action={
              <a href="/login">
                <Button className="w-full">כניסה למערכת</Button>
              </a>
            }
          />
        ) : c.status === 'expired' ? (
          <Notice icon={<FileWarning className="h-7 w-7" />} title="תוקף הקישור פג" text="פנו למתנ״ס ונשלח לך קישור חדש לחתימה." />
        ) : c.status !== 'pending' ? (
          <Notice icon={<FileWarning className="h-7 w-7" />} title="החוזה בוטל" text="החוזה אינו זמין עוד לחתימה. לפרטים פנו למתנ״ס." />
        ) : c.signUrl && back ? (
          <Notice
            ok
            icon={<Loader2 className="h-7 w-7 animate-spin" />}
            title="החתימה התקבלה, מעדכנים את המערכת…"
            text="זה לוקח בדרך כלל פחות מדקה. אפשר לסגור את הדף: נשלח אליך אימייל כשהחשבון ייפתח."
            action={
              <a href={c.signUrl}>
                <Button variant="secondary" className="w-full">
                  לא סיימתי לחתום – חזרה לחתימה
                </Button>
              </a>
            }
          />
        ) : c.signUrl ? (
          <div className="space-y-4">
            <article className="card p-4 sm:p-6">
              <h1 className="text-xl font-bold">{c.title}</h1>
              <p className="mt-1 text-xs text-muted">לחתימה עד {dateTime(c.expiresAt)}</p>
              <div className="mt-4 whitespace-pre-line text-[15px] leading-relaxed">{c.body}</div>
            </article>
            <section className="card space-y-3 p-4 sm:p-6">
              <p className="text-sm">החתימה נעשית על מסמך PDF מאובטח. בסיום נשמר עותק חתום עם אישור חתימה, והחשבון שלך במערכת נפתח.</p>
              <a href={c.signUrl} className="block">
                <Button size="lg" className="w-full">
                  <PenLine className="h-4 w-4" /> מעבר לחתימה
                  <ExternalLink className="h-4 w-4" />
                </Button>
              </a>
            </section>
          </div>
        ) : (
          <div className="space-y-4">
            <article className="card p-4 sm:p-6">
              <h1 className="text-xl font-bold">{c.title}</h1>
              <p className="mt-1 text-xs text-muted">לחתימה עד {dateTime(c.expiresAt)}</p>
              <div className="mt-4 whitespace-pre-line text-[15px] leading-relaxed">{c.body}</div>
            </article>
            <section className="card space-y-4 p-4 sm:p-6">
              <Field label="שם מלא *" error={errors.name}>
                <Input value={name} onChange={(e) => setName(e.target.value)} autoComplete="name" />
              </Field>
              <div>
                <span className="label flex items-center gap-1">
                  <PenLine className="h-3.5 w-3.5" /> חתימה *
                </span>
                <SignaturePad onChange={setSignature} />
                {errors.signature && <p className="mt-1 text-xs text-danger">{errors.signature}</p>}
              </div>
              <label className="flex items-start gap-2 text-sm">
                <input type="checkbox" checked={agree} onChange={(e) => setAgree(e.target.checked)} className="mt-0.5 h-5 w-5 shrink-0 accent-[rgb(var(--brand))]" />
                קראתי את החוזה ואני מסכים/ה לתנאיו. חתימה זו מחייבת כחתימה בכתב יד.
              </label>
              {errors.agree && <p className="text-xs text-danger">{errors.agree}</p>}
              {sign.isError && !Object.keys(errors).length && <p className="text-sm text-danger">{errorMessage(sign.error)}</p>}
              {errors.token && <p className="text-sm text-danger">{errors.token}</p>}
              <Button size="lg" className="w-full" disabled={!agree || !signature || name.trim().length < 2} loading={sign.isPending} onClick={() => sign.mutate()}>
                חתימה ושליחה
              </Button>
            </section>
          </div>
        )}
        <PoweredBy className="mt-8 justify-center" />
      </main>
    </div>
  )
}

function Notice({ icon, title, text, ok, action }: { icon: React.ReactNode; title: string; text: string; ok?: boolean; action?: React.ReactNode }) {
  return (
    <div className="card p-6 text-center sm:p-8">
      <div className={`mx-auto flex h-14 w-14 items-center justify-center rounded-full ${ok ? 'bg-emerald-50 text-ok dark:bg-emerald-950' : 'bg-amber-50 text-amber-600 dark:bg-amber-950'}`}>{icon}</div>
      <h1 className="mt-4 text-xl font-bold">{title}</h1>
      <p className="mt-2 text-sm text-muted">{text}</p>
      {action && <div className="mt-6">{action}</div>}
    </div>
  )
}

/** Finger / mouse signature on a canvas; reports a PNG data URL (null when empty). */
function SignaturePad({ onChange }: { onChange: (dataUrl: string | null) => void }) {
  const canvas = useRef<HTMLCanvasElement>(null)
  const drawing = useRef(false)
  const strokes = useRef(0)

  useEffect(() => {
    const el = canvas.current
    if (!el) return
    const ratio = window.devicePixelRatio || 1
    el.width = el.offsetWidth * ratio
    el.height = el.offsetHeight * ratio
    const ctx = el.getContext('2d')
    if (!ctx) return // no canvas support (very old browsers, tests)
    ctx.scale(ratio, ratio)
    ctx.lineWidth = 2.4
    ctx.lineCap = 'round'
    ctx.lineJoin = 'round'
    ctx.strokeStyle = '#111827'
  }, [])

  const point = (e: React.PointerEvent) => {
    const r = canvas.current!.getBoundingClientRect()
    return { x: e.clientX - r.left, y: e.clientY - r.top }
  }
  const clear = () => {
    const el = canvas.current!
    el.getContext('2d')!.clearRect(0, 0, el.width, el.height)
    strokes.current = 0
    onChange(null)
  }

  return (
    <div>
      <canvas
        ref={canvas}
        className="h-40 w-full touch-none rounded-xl border-2 border-dashed border-line bg-white"
        onPointerDown={(e) => {
          e.currentTarget.setPointerCapture(e.pointerId)
          drawing.current = true
          const ctx = canvas.current!.getContext('2d')!
          const p = point(e)
          ctx.beginPath()
          ctx.moveTo(p.x, p.y)
          ctx.lineTo(p.x + 0.1, p.y + 0.1)
          ctx.stroke()
        }}
        onPointerMove={(e) => {
          if (!drawing.current) return
          const ctx = canvas.current!.getContext('2d')!
          const p = point(e)
          ctx.lineTo(p.x, p.y)
          ctx.stroke()
        }}
        onPointerUp={() => {
          if (!drawing.current) return
          drawing.current = false
          strokes.current++
          onChange(canvas.current!.toDataURL('image/png'))
        }}
        onPointerCancel={() => (drawing.current = false)}
        aria-label="תיבת חתימה"
      />
      <div className="mt-1 flex justify-between text-xs text-muted">
        <span>חתמו עם האצבע או העכבר</span>
        <button type="button" onClick={clear} className="flex items-center gap-1 font-medium text-brand">
          <Eraser className="h-3.5 w-3.5" /> ניקוי
        </button>
      </div>
    </div>
  )
}
