import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { addDays, startOfWeek } from 'date-fns'
import { CalendarDays, FileSignature, Mail, Phone, Send, UserPlus } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ContractDialog, ContractFields, coordinatorTemplate as contractTemplate, type ContractResult as CoordinatorResult, ScanUpload, SendContractDialog, SignLink } from '@/components/Contracts'
import { toast } from '@/components/toast'
import { Avatar, Badge, Button, Empty, Field, Input, Modal, PageHeader, PageLoader } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { useCan } from '@/lib/auth'
import { useOrg } from '@/lib/brand'
import { dateShort, ltr, timeRange } from '@/lib/format'
import { contractStatusLabel } from '@/lib/labels'
import { useActivities, useUsers } from '@/lib/queries'
import type { Booking, Contract, User } from '@/lib/types'

type ContractMode = 'digital' | 'scan' | 'none'

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
      <SendContractDialog target={contractFor && { kind: 'user', user: contractFor }} onClose={() => setContractFor(null)} />
      <ContractDialog contract={viewing} onClose={() => setViewing(null)} />
    </div>
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
          {result.signUrl && <SignLink url={result.signUrl} sentVia={result.sentVia} phone={result.user.phone} name={result.user.fullName} />}
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
