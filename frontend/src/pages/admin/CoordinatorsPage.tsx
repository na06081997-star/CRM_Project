import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { addDays, startOfWeek } from 'date-fns'
import { CalendarDays, Copy, FileSignature, FileText, Mail, Phone, Send, UserPlus } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Attachments } from '@/components/Attachments'
import { PhotoGallery } from '@/components/Photos'
import { toast } from '@/components/toast'
import { Avatar, Badge, Button, Empty, Field, Input, Modal, PageHeader, PageLoader, Textarea } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { useCan } from '@/lib/auth'
import { useOrg } from '@/lib/brand'
import { dateShort, dateTime, ltr, timeRange } from '@/lib/format'
import { contractStatusLabel } from '@/lib/labels'
import { useActivities, useUsers } from '@/lib/queries'
import type { Booking, Contract, User } from '@/lib/types'

interface CoordinatorResult {
  user: User
  contract: Contract | null
  signUrl?: string
}

type ContractMode = 'digital' | 'scan' | 'none'

function contractTemplate(org: string, name: string) {
  return `הסכם התקשרות – רכז/ת פעילות ב${org}

1. ${name || 'הרכז/ת'} ירכז/תרכז פעילויות קהילתיות בהתאם לתוכנית העבודה שתיקבע עם הנהלת המתנ״ס.
2. הזמנת חללים תיעשה דרך המערכת בלבד ובכפוף לאישור המתנ״ס.
3. הרכז/ת אחראי/ת לשמירה על החללים והציוד, ולדיווח על כל תקלה דרך המערכת.
4. הרכז/ת ישמור/תשמור על נהלי הבטיחות, הניקיון ושעות הפעילות של המתנ״ס.
5. הרכז/ת ישמור/תשמור על פרטיות המשתתפים ועל סודיות המידע שיגיע לידיעתו/ה.
6. כל צד רשאי לסיים את ההתקשרות בהודעה מוקדמת בכתב.`
}

export function CoordinatorsPage() {
  const can = useCan()
  const manage = can('coordinators:manage')
  const { data: coordinators, isLoading } = useUsers('coordinator')
  const activities = useActivities().data ?? []
  const contracts = useQuery({ queryKey: ['contracts'], queryFn: () => api.get<Contract[]>('/contracts'), enabled: manage }).data ?? []
  const from = startOfWeek(new Date(), { weekStartsOn: 0 })
  const { data: week = [] } = useQuery({
    queryKey: ['bookings', 'coord-week', from.toISOString()],
    queryFn: () => api.get<Booking[]>('/bookings', { from: from.toISOString(), to: addDays(from, 7).toISOString(), statuses: ['confirmed', 'pending'] }),
  })
  const [adding, setAdding] = useState(false)
  const [contractFor, setContractFor] = useState<User | null>(null)
  const [viewing, setViewing] = useState<Contract | null>(null)
  if (isLoading) return <PageLoader />
  const list = [...(coordinators ?? [])].sort((a, b) => Number(b.isActive) - Number(a.isActive))
  const now = Date.now()
  const latest = (uid: string) => contracts.find((c) => c.userId === uid && c.status !== 'cancelled')

  return (
    <div>
      <PageHeader
        title="רכזים"
        subtitle="עומס שבועי, פעילויות וחוזים לכל רכז/ת"
        actions={
          manage && (
            <Button icon={<UserPlus className="h-4 w-4" />} onClick={() => setAdding(true)}>
              רכז/ת חדש/ה
            </Button>
          )
        }
      />
      {list.length === 0 ? (
        <div className="card">
          <Empty title="אין רכזים במערכת" hint="הוסיפו רכז/ת ושלחו חוזה לחתימה דיגיטלית" />
        </div>
      ) : (
        <div className="grid gap-4 lg:grid-cols-2 2xl:grid-cols-3">
          {list.map((c) => {
            const mine = week.filter((b) => b.coordinatorId === c.id)
            const hours = mine.reduce((s, b) => s + (new Date(b.endsAt).getTime() - new Date(b.startsAt).getTime()) / 3600000, 0)
            const acts = activities.filter((a) => a.coordinatorId === c.id)
            const upcoming = mine.filter((b) => new Date(b.endsAt).getTime() > now).slice(0, 4)
            const contract = latest(c.id)
            return (
              <article key={c.id} className={clsx('card p-4 sm:p-5', !c.isActive && 'border-dashed')}>
                <div className="flex items-center gap-3">
                  <Avatar name={c.fullName} color={c.avatarColor} size={44} />
                  <div className="min-w-0 flex-1">
                    <h3 className="flex flex-wrap items-center gap-2 font-semibold">
                      {c.fullName}
                      {!c.isActive && <Badge tone="amber">{contract?.status === 'pending' ? 'ממתין/ה לחתימה' : 'לא פעיל/ה'}</Badge>}
                    </h3>
                    <div className="flex flex-wrap gap-x-3 text-xs text-muted">
                      {c.phone && (
                        <a href={`tel:${c.phone}`} className="flex items-center gap-1 hover:text-brand">
                          <Phone className="h-3 w-3" /> {ltr(c.phone)}
                        </a>
                      )}
                      <a href={`mailto:${c.email}`} className="flex min-w-0 items-center gap-1 truncate hover:text-brand">
                        <Mail className="h-3 w-3" /> {c.email}
                      </a>
                    </div>
                  </div>
                </div>
                <div className="mt-4 grid grid-cols-3 gap-2 text-center">
                  <div className="rounded-xl bg-subtle p-2">
                    <p className="text-lg font-bold">{mine.length}</p>
                    <p className="text-[11px] text-muted">מפגשים השבוע</p>
                  </div>
                  <div className="rounded-xl bg-subtle p-2">
                    <p className="text-lg font-bold">{hours.toFixed(1)}</p>
                    <p className="text-[11px] text-muted">שעות</p>
                  </div>
                  <div className="rounded-xl bg-subtle p-2">
                    <p className="text-lg font-bold">{acts.length}</p>
                    <p className="text-[11px] text-muted">פעילויות</p>
                  </div>
                </div>
                <ul className="mt-4 space-y-1.5 text-sm">
                  {upcoming.length === 0 && <li className="text-xs text-muted">אין מפגשים נוספים השבוע</li>}
                  {upcoming.map((b) => (
                    <li key={b.id} className="flex justify-between gap-2 text-xs">
                      <span className="truncate">{b.title}</span>
                      <span className="shrink-0 tabular-nums text-muted">
                        {dateShort(b.startsAt)} {timeRange(b.startsAt, b.endsAt)}
                      </span>
                    </li>
                  ))}
                </ul>
                {manage && (
                  <div className="mt-4 flex items-center gap-2 rounded-xl border border-line px-3 py-2 text-sm">
                    <FileSignature className="h-4 w-4 shrink-0 text-muted" />
                    <span className="min-w-0 flex-1 truncate">
                      {contract ? (
                        <>
                          חוזה: <b>{contractStatusLabel[contract.status]}</b>
                          <span className="text-xs text-muted"> · {contract.method === 'digital' ? 'חתימה דיגיטלית' : 'סריקה'}</span>
                        </>
                      ) : (
                        <span className="text-muted">אין חוזה במערכת</span>
                      )}
                    </span>
                    {contract ? (
                      <Button size="sm" variant="soft" onClick={() => setViewing(contract)}>
                        {contract.status === 'signed' ? 'צפייה' : 'ניהול'}
                      </Button>
                    ) : (
                      <Button size="sm" variant="soft" onClick={() => setContractFor(c)}>
                        שליחת חוזה
                      </Button>
                    )}
                  </div>
                )}
                <Link to={`/admin/calendar?view=week&coordinator=${c.id}`}>
                  <Button variant="secondary" size="sm" className="mt-3 w-full" icon={<CalendarDays className="h-3.5 w-3.5" />}>
                    היומן של {c.fullName.split(' ')[0]}
                  </Button>
                </Link>
              </article>
            )
          })}
        </div>
      )}
      <NewCoordinatorDialog open={adding} onClose={() => setAdding(false)} />
      <SendContractDialog user={contractFor} onClose={() => setContractFor(null)} />
      <ContractDialog contract={viewing} onClose={() => setViewing(null)} />
    </div>
  )
}

/** Signing link to copy (also sent by SMS / e-mail). */
function SignLink({ url }: { url: string }) {
  return (
    <div className="rounded-xl border border-ok/30 bg-emerald-50 p-3 text-sm dark:bg-emerald-950/20">
      <p className="font-semibold text-ok">הקישור לחתימה נשלח ב־SMS ובאימייל</p>
      <div className="mt-2 flex items-center gap-2">
        <code dir="ltr" className="min-w-0 flex-1 truncate rounded-lg bg-surface px-2 py-1.5 text-xs">
          {url}
        </code>
        <Button
          size="sm"
          variant="secondary"
          icon={<Copy className="h-3.5 w-3.5" />}
          onClick={() => navigator.clipboard.writeText(url).then(() => toast.success('הקישור הועתק'), () => toast.error('לא ניתן להעתיק'))}
        >
          העתקה
        </Button>
      </div>
      <p className="mt-2 text-xs text-muted">החשבון ייפתח אוטומטית מיד לאחר החתימה. הקישור בתוקף 14 יום.</p>
    </div>
  )
}

function ContractFields({ title, body, onTitle, onBody, errors }: { title: string; body: string; onTitle: (v: string) => void; onBody: (v: string) => void; errors: Record<string, string> }) {
  return (
    <>
      <Field label="כותרת החוזה">
        <Input value={title} onChange={(e) => onTitle(e.target.value)} />
      </Field>
      <Field label="נוסח החוזה" error={errors.body} hint="נוסח לדוגמה – ערכו לפי ההסכם של המתנ״ס">
        <Textarea value={body} onChange={(e) => onBody(e.target.value)} rows={8} />
      </Field>
    </>
  )
}

function NewCoordinatorDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const qc = useQueryClient()
  const org = useOrg()
  const orgName = org?.name ?? 'המתנ״ס'
  const blank = { fullName: '', email: '', phone: '', contract: 'digital' as ContractMode, title: 'הסכם התקשרות – רכז/ת', body: '' }
  const [f, setF] = useState(blank)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [result, setResult] = useState<CoordinatorResult | null>(null)
  const set = <K extends keyof typeof f>(k: K, v: (typeof f)[K]) => setF((p) => ({ ...p, [k]: v }))
  const body = f.body || contractTemplate(orgName, f.fullName)

  const close = () => {
    setF(blank)
    setErrors({})
    setResult(null)
    onClose()
  }
  const save = useMutation({
    mutationFn: () => api.post<CoordinatorResult>('/coordinators', { ...f, body: f.contract === 'none' ? '' : body }),
    onSuccess: (r) => {
      qc.invalidateQueries({ queryKey: ['users'] })
      qc.invalidateQueries({ queryKey: ['contracts'] })
      if (f.contract === 'none') {
        toast.success('הרכז/ת נוסף/ה והחשבון פעיל')
        close()
      } else setResult(r)
    },
    onError: (e) => {
      if (e instanceof ApiError) setErrors(e.fields)
      toast.error(errorMessage(e))
    },
  })

  const modes: { value: ContractMode; label: string; hint: string }[] = [
    { value: 'digital', label: 'חתימה דיגיטלית', hint: 'קישור ב־SMS ובאימייל; החשבון נפתח אחרי החתימה' },
    { value: 'scan', label: 'חוזה חתום בסריקה', hint: 'מעלים צילום / PDF של החוזה החתום' },
    { value: 'none', label: 'ללא חוזה', hint: 'החשבון פעיל מיד' },
  ]

  return (
    <Modal
      open={open}
      onClose={close}
      size="lg"
      title={result ? 'הרכז/ת נוסף/ה' : 'רכז/ת חדש/ה'}
      footer={
        result ? (
          <Button onClick={close}>סיום</Button>
        ) : (
          <>
            <Button variant="secondary" onClick={close}>
              ביטול
            </Button>
            <Button icon={f.contract === 'digital' ? <Send className="h-4 w-4" /> : <UserPlus className="h-4 w-4" />} loading={save.isPending} onClick={() => save.mutate()}>
              {f.contract === 'digital' ? 'הוספה ושליחת חוזה' : 'הוספה'}
            </Button>
          </>
        )
      }
    >
      {result ? (
        <div className="space-y-4">
          <p className="text-sm">
            <b>{result.user.fullName}</b> נוסף/ה כרכז/ת. החשבון ייפתח לאחר חתימה על החוזה.
          </p>
          {result.signUrl && <SignLink url={result.signUrl} />}
          {result.contract?.method === 'scan' && <ScanUpload contract={result.contract} onSigned={close} />}
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="שם מלא *" error={errors.fullName}>
            <Input value={f.fullName} onChange={(e) => set('fullName', e.target.value)} autoFocus />
          </Field>
          <Field label="טלפון נייד" error={errors.phone} hint={f.contract === 'digital' ? 'לשליחת הקישור ב־SMS' : undefined}>
            <Input type="tel" value={f.phone} onChange={(e) => set('phone', e.target.value)} placeholder="050-000-0000" />
          </Field>
          <Field label="אימייל (חשבון Google) *" error={errors.email} hint="איתו הרכז/ת יתחבר/תתחבר למערכת" className="sm:col-span-2">
            <Input type="email" value={f.email} onChange={(e) => set('email', e.target.value)} />
          </Field>
          <div className="sm:col-span-2">
            <span className="label">חוזה</span>
            <div className="grid gap-2 sm:grid-cols-3">
              {modes.map((m) => (
                <button
                  key={m.value}
                  type="button"
                  onClick={() => set('contract', m.value)}
                  className={clsx('rounded-xl border p-3 text-start transition', f.contract === m.value ? 'border-brand bg-brand-soft ring-1 ring-brand/40' : 'border-line hover:bg-subtle')}
                >
                  <p className={clsx('text-sm font-semibold', f.contract === m.value && 'text-brand-strong')}>{m.label}</p>
                  <p className="mt-0.5 text-[11px] leading-tight text-muted">{m.hint}</p>
                </button>
              ))}
            </div>
          </div>
          {f.contract !== 'none' && (
            <div className="grid gap-4 sm:col-span-2">
              <ContractFields title={f.title} body={body} onTitle={(v) => set('title', v)} onBody={(v) => set('body', v)} errors={errors} />
            </div>
          )}
        </div>
      )}
    </Modal>
  )
}

function SendContractDialog({ user, onClose }: { user: User | null; onClose: () => void }) {
  const qc = useQueryClient()
  const org = useOrg()
  const [method, setMethod] = useState<'digital' | 'scan'>('digital')
  const [title, setTitle] = useState('הסכם התקשרות – רכז/ת')
  const [body, setBody] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [result, setResult] = useState<CoordinatorResult | null>(null)
  const text = body || contractTemplate(org?.name ?? 'המתנ״ס', user?.fullName ?? '')
  const close = () => {
    setResult(null)
    setBody('')
    setErrors({})
    onClose()
  }
  const send = useMutation({
    mutationFn: () => api.post<CoordinatorResult>('/contracts', { userId: user!.id, method, title, body: text }),
    onSuccess: (r) => {
      qc.invalidateQueries({ queryKey: ['contracts'] })
      setResult(r)
    },
    onError: (e) => {
      if (e instanceof ApiError) setErrors(e.fields)
      toast.error(errorMessage(e))
    },
  })
  return (
    <Modal
      open={!!user}
      onClose={close}
      size="lg"
      title={`חוזה ל${user?.fullName ?? ''}`}
      footer={
        result ? (
          <Button onClick={close}>סיום</Button>
        ) : (
          <>
            <Button variant="secondary" onClick={close}>
              ביטול
            </Button>
            <Button loading={send.isPending} onClick={() => send.mutate()}>
              {method === 'digital' ? 'שליחה לחתימה' : 'יצירה'}
            </Button>
          </>
        )
      }
    >
      {result ? (
        <div className="space-y-4">
          {result.signUrl && <SignLink url={result.signUrl} />}
          {result.contract?.method === 'scan' && <ScanUpload contract={result.contract} onSigned={close} />}
        </div>
      ) : (
        <div className="grid gap-4">
          <div className="grid grid-cols-2 gap-2">
            {(['digital', 'scan'] as const).map((m) => (
              <button
                key={m}
                type="button"
                onClick={() => setMethod(m)}
                className={clsx('rounded-xl border p-3 text-sm font-semibold', method === m ? 'border-brand bg-brand-soft text-brand-strong' : 'border-line')}
              >
                {m === 'digital' ? 'חתימה דיגיטלית' : 'סריקת חוזה חתום'}
              </button>
            ))}
          </div>
          {method === 'digital' && !user?.phone && <p className="text-xs text-amber-700">אין טלפון לרכז/ת – הקישור יישלח באימייל בלבד.</p>}
          <ContractFields title={title} body={text} onTitle={setTitle} onBody={setBody} errors={errors} />
        </div>
      )}
    </Modal>
  )
}

/** Upload a scanned signed contract, then confirm (opens the account). */
function ScanUpload({ contract, onSigned }: { contract: Contract; onSigned: () => void }) {
  const qc = useQueryClient()
  const mark = useMutation({
    mutationFn: () => api.post<Contract>(`/contracts/${contract.id}/mark-signed`),
    onSuccess: () => {
      toast.success('החוזה סומן כחתום והחשבון הופעל')
      qc.invalidateQueries({ queryKey: ['contracts'] })
      qc.invalidateQueries({ queryKey: ['users'] })
      onSigned()
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  return (
    <div className="space-y-3 rounded-xl border border-line p-3">
      <p className="text-sm font-semibold">העלאת החוזה החתום</p>
      <Attachments entity="contract" entityId={contract.id} />
      <Button className="w-full" icon={<FileSignature className="h-4 w-4" />} loading={mark.isPending} onClick={() => mark.mutate()}>
        אישור – החוזה נחתם, הפעלת החשבון
      </Button>
    </div>
  )
}

function ContractDialog({ contract, onClose }: { contract: Contract | null; onClose: () => void }) {
  const qc = useQueryClient()
  const [link, setLink] = useState<string | null>(null)
  const resend = useMutation({
    mutationFn: () => api.post<CoordinatorResult>(`/contracts/${contract!.id}/resend`),
    onSuccess: (r) => {
      setLink(r.signUrl ?? null)
      qc.invalidateQueries({ queryKey: ['contracts'] })
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  const cancel = useMutation({
    mutationFn: () => api.post(`/contracts/${contract!.id}/cancel`),
    onSuccess: () => {
      toast.success('החוזה בוטל')
      qc.invalidateQueries({ queryKey: ['contracts'] })
      close()
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  const close = () => {
    setLink(null)
    onClose()
  }
  if (!contract) return null
  const pending = contract.status === 'pending'
  return (
    <Modal
      open
      onClose={close}
      size="lg"
      title={
        <span className="flex items-center gap-2">
          {contract.title}
          <Badge tone={contract.status === 'signed' ? 'green' : pending ? 'amber' : 'gray'}>{contractStatusLabel[contract.status]}</Badge>
        </span>
      }
      footer={
        <>
          {pending && (
            <Button variant="ghost" className="me-auto text-danger" loading={cancel.isPending} onClick={() => cancel.mutate()}>
              ביטול החוזה
            </Button>
          )}
          {pending && contract.method === 'digital' && (
            <Button variant="secondary" icon={<Send className="h-4 w-4" />} loading={resend.isPending} onClick={() => resend.mutate()}>
              שליחה חוזרת
            </Button>
          )}
          <Button onClick={close}>סגירה</Button>
        </>
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-muted">
          {contract.userName} · {contract.method === 'digital' ? 'חתימה דיגיטלית' : 'סריקה'}
          {contract.sentAt && ` · נשלח ${dateTime(contract.sentAt)}`}
        </p>
        {link && <SignLink url={link} />}
        {contract.status === 'signed' && (
          <div className="rounded-xl bg-emerald-50 p-3 text-sm dark:bg-emerald-950/20">
            נחתם ע״י <b>{contract.signedName}</b> {contract.signedAt && `ב־${dateTime(contract.signedAt)}`}
          </div>
        )}
        <PhotoGallery entity="contract" entityId={contract.id} title="חתימה" />
        {pending && contract.method === 'scan' ? (
          <ScanUpload contract={contract} onSigned={close} />
        ) : (
          <Attachments entity="contract" entityId={contract.id} canDelete={pending} />
        )}
        <details className="rounded-xl border border-line p-3">
          <summary className="flex cursor-pointer items-center gap-2 text-sm font-medium">
            <FileText className="h-4 w-4 text-muted" /> נוסח החוזה
          </summary>
          <p className="mt-3 whitespace-pre-line text-sm leading-relaxed">{contract.body}</p>
        </details>
      </div>
    </Modal>
  )
}
