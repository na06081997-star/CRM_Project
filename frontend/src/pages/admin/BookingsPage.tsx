import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { addDays, startOfDay } from 'date-fns'
import { AlertTriangle, Check, CheckCircle2, ClipboardList, Mail, Phone, Search, X } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Attachments } from '@/components/Attachments'
import { toast } from '@/components/toast'
import { Badge, Button, Drawer, Empty, Field, Input, PageHeader, PageLoader, Select, Tabs, Textarea } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { dateLong, dateShort, dateTime, money, relative, timeRange } from '@/lib/format'
import { bookingStatusLabel, bookingTypeShort, requestRouteLabel, requestStatusLabel, roleLabel, typeStyles } from '@/lib/labels'
import { useLookups, useUsers } from '@/lib/queries'
import type { Booking, BookingRequest, Conflict, RequestStatus } from '@/lib/types'
import { BookingDialog } from '../calendar/BookingDialog'

const statusTone: Record<RequestStatus, 'amber' | 'green' | 'red' | 'gray'> = { pending: 'amber', approved: 'green', rejected: 'red', cancelled: 'gray' }

export function BookingsPage() {
  const [params, setParams] = useSearchParams()
  const tab = (params.get('tab') as 'requests' | 'cancellations' | 'bookings') || 'requests'
  return (
    <div>
      <PageHeader
        title="הזמנות ובקשות"
        subtitle="בדיקת בקשות מהפורטל, תמחור ואישור – וניהול כל ההזמנות במקום אחד"
        actions={
          <Tabs
            value={tab}
            onChange={(v) => setParams({ tab: v })}
            items={[
              { value: 'requests', label: 'בקשות מהפורטל' },
              { value: 'cancellations', label: 'בקשות ביטול' },
              { value: 'bookings', label: 'כל ההזמנות' },
            ]}
          />
        }
      />
      {tab === 'requests' && <Requests initial={params.get('request')} />}
      {tab === 'cancellations' && <Cancellations />}
      {tab === 'bookings' && <AllBookings />}
    </div>
  )
}

// ── Requests ─────────────────────────────────────────────────

function Requests({ initial }: { initial: string | null }) {
  const [status, setStatus] = useState<RequestStatus | ''>('pending')
  const [open, setOpen] = useState<BookingRequest | null>(null)
  const { data = [], isLoading } = useQuery({
    queryKey: ['requests', status || 'all'],
    queryFn: () => api.get<BookingRequest[]>('/requests', { status: status || undefined }),
  })
  useEffect(() => {
    if (initial && !open) {
      const r = data.find((x) => x.id === initial)
      if (r) setOpen(r)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initial, data])

  return (
    <>
      <div className="mb-4 flex flex-wrap gap-2">
        {(['pending', 'approved', 'rejected', 'cancelled', ''] as const).map((s) => (
          <button
            key={s || 'all'}
            onClick={() => setStatus(s)}
            className={clsx('rounded-full border px-3.5 py-1.5 text-sm transition', status === s ? 'border-ink bg-ink text-surface' : 'border-line bg-surface hover:bg-subtle')}
          >
            {s ? requestStatusLabel[s] : 'הכל'}
          </button>
        ))}
      </div>
      {isLoading ? (
        <PageLoader />
      ) : data.length === 0 ? (
        <div className="card">
          <Empty icon={<ClipboardList className="h-6 w-6" />} title="אין בקשות להצגה" hint="בקשות שמוגשות בפורטל המשתמשים יופיעו כאן לבדיקה ואישור." />
        </div>
      ) : (
        <div className="grid gap-3 md:grid-cols-2 2xl:grid-cols-3">
          {data.map((r) => (
            <button key={r.id} onClick={() => setOpen(r)} className="card group p-4 text-start transition hover:border-brand/40 hover:shadow-pop">
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <p className="truncate font-semibold">{r.title}</p>
                  <p className="text-xs text-muted">
                    {r.requesterName} ({roleLabel[r.requesterRole]}) · {r.eventKind || 'אירוע'}
                  </p>
                </div>
                <Badge tone={statusTone[r.status]}>{requestStatusLabel[r.status]}</Badge>
              </div>
              <Badge tone={r.route === 'activity' ? 'teal' : 'orange'} className="mt-2">
                {requestRouteLabel[r.route]}
              </Badge>
              <div className="mt-3 grid grid-cols-2 gap-2 text-xs">
                <Info label="חדר" value={r.roomName} />
                <Info label="מועד" value={`${dateShort(r.startsAt)} · ${timeRange(r.startsAt, r.endsAt)}`} />
                <Info label="משתתפים" value={String(r.attendees || '—')} />
                <Info label={r.route === 'activity' ? 'רכז' : 'הצעת מחיר'} value={r.route === 'activity' ? r.requesterName : money(r.finalPrice ?? r.quotedPrice)} />
              </div>
              <div className="mt-3 flex items-center justify-between text-[11px] text-muted">
                <span>התקבלה {relative(r.createdAt)}</span>
                {r.hasConflict ? (
                  <span className="flex items-center gap-1 font-medium text-danger">
                    <AlertTriangle className="h-3.5 w-3.5" /> מתנגש עם הזמנה קיימת
                  </span>
                ) : (
                  r.status === 'pending' && (
                    <span className="flex items-center gap-1 font-medium text-ok">
                      <CheckCircle2 className="h-3.5 w-3.5" /> פנוי
                    </span>
                  )
                )}
              </div>
            </button>
          ))}
        </div>
      )}
      <ReviewDrawer request={open} onClose={() => setOpen(null)} />
    </>
  )
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg bg-subtle px-2.5 py-1.5">
      <p className="text-[10px] text-muted">{label}</p>
      <p className="truncate font-medium">{value}</p>
    </div>
  )
}

function ReviewDrawer({ request, onClose }: { request: BookingRequest | null; onClose: () => void }) {
  const qc = useQueryClient()
  const { equipmentLabel } = useLookups()
  const coordinators = useUsers('coordinator').data ?? []
  const [price, setPrice] = useState('')
  const [notes, setNotes] = useState('')
  const [reason, setReason] = useState('')
  const [coordinatorId, setCoordinatorId] = useState('')
  const [mode, setMode] = useState<'view' | 'reject'>('view')
  const [conflicts, setConflicts] = useState<Conflict[] | null>(null)

  useEffect(() => {
    if (!request) return
    setPrice(String(request.finalPrice ?? request.quotedPrice))
    setNotes(request.internalNotes ?? '')
    setReason('')
    setMode('view')
    setConflicts(null)
    if (request.status === 'pending')
      api
        .post<{ conflicts: Conflict[] }>('/bookings/check', { roomId: request.roomId, startsAt: request.startsAt, endsAt: request.endsAt })
        .then((r) => setConflicts(r.conflicts))
        .catch(() => setConflicts([]))
  }, [request])

  const review = useMutation({
    mutationFn: (approve: boolean) =>
      api.post<BookingRequest>(`/requests/${request!.id}/review`, {
        approve,
        finalPrice: price === '' ? null : Number(price),
        internalNotes: notes,
        rejectionReason: reason,
        coordinatorId: coordinatorId || null,
      }),
    onSuccess: (_, approve) => {
      toast.success(approve ? 'הבקשה אושרה והוזמנה ביומן ✓' : 'הבקשה נדחתה, המבקש/ת עודכנו')
      for (const k of ['requests', 'bookings', 'dashboard', 'balances']) qc.invalidateQueries({ queryKey: [k] })
      onClose()
    },
    onError: (e) => {
      if (e instanceof ApiError && e.conflicts.length) setConflicts(e.conflicts)
      toast.error(errorMessage(e))
    },
  })

  if (!request) return null
  const pending = request.status === 'pending'
  return (
    <Drawer
      open
      onClose={onClose}
      title={request.title}
      footer={
        pending &&
        (mode === 'view' ? (
          <>
            <Button variant="secondary" icon={<X className="h-4 w-4" />} onClick={() => setMode('reject')}>
              דחייה
            </Button>
            <Button icon={<Check className="h-4 w-4" />} onClick={() => review.mutate(true)} loading={review.isPending} disabled={!!conflicts?.length}>
              אישור והזמנה ביומן
            </Button>
          </>
        ) : (
          <>
            <Button variant="secondary" onClick={() => setMode('view')}>
              חזרה
            </Button>
            <Button variant="danger" onClick={() => review.mutate(false)} loading={review.isPending} disabled={!reason.trim()}>
              דחיית הבקשה
            </Button>
          </>
        ))
      }
    >
      <div className="space-y-5">
        <div className="flex items-center gap-2">
          <Badge tone={statusTone[request.status]}>{requestStatusLabel[request.status]}</Badge>
          <span className="text-xs text-muted">הוגשה {dateTime(request.createdAt)}</span>
        </div>

        <section className="rounded-2xl border border-line p-4">
          <p className="text-sm font-semibold">{request.requesterName}</p>
          <div className="mt-2 flex flex-wrap gap-3 text-xs text-muted">
            <a href={`mailto:${request.requesterEmail}`} className="flex items-center gap-1 hover:text-brand">
              <Mail className="h-3.5 w-3.5" /> {request.requesterEmail}
            </a>
            {request.requesterPhone && (
              <a href={`tel:${request.requesterPhone}`} className="flex items-center gap-1 hover:text-brand">
                <Phone className="h-3.5 w-3.5" /> {request.requesterPhone}
              </a>
            )}
          </div>
        </section>

        <section className="grid grid-cols-2 gap-2 text-sm">
          <Info label="חדר" value={request.roomName} />
          <Info label="סוג אירוע" value={request.eventKind || '—'} />
          <Info label="תאריך" value={dateLong(request.startsAt)} />
          <Info label="שעות" value={timeRange(request.startsAt, request.endsAt)} />
          <Info label="משתתפים" value={String(request.attendees)} />
          <Info label="ציוד" value={request.equipment.map(equipmentLabel).join(', ') || '—'} />
          <Info label="מסלול" value={requestRouteLabel[request.route]} />
          {request.route === 'activity' && <Info label="פעילות" value={request.activityName ?? 'חד־פעמית'} />}
          {(request.tablesCount > 0 || request.chairsCount > 0) && <Info label="שולחנות / כיסאות" value={`${request.tablesCount} / ${request.chairsCount}`} />}
        </section>
        {request.notes && <p className="rounded-xl bg-subtle p-3 text-sm">“{request.notes}”</p>}

        {pending && (
          <section
            className={clsx(
              'rounded-2xl border p-4 text-sm',
              conflicts === null ? 'border-line' : conflicts.length ? 'border-danger/40 bg-red-50 dark:bg-red-950/30' : 'border-ok/30 bg-emerald-50 dark:bg-emerald-950/20',
            )}
          >
            {conflicts === null ? (
              'בודק זמינות…'
            ) : conflicts.length ? (
              <>
                <p className="flex items-center gap-1.5 font-semibold text-danger">
                  <AlertTriangle className="h-4 w-4" /> המועד תפוס – לא ניתן לאשר
                </p>
                <ul className="mt-2 space-y-1 text-xs">
                  {conflicts.map((c) => (
                    <li key={c.bookingId}>
                      {c.title} · {timeRange(c.startsAt, c.endsAt)}
                    </li>
                  ))}
                </ul>
                <p className="mt-2 text-xs text-muted">ניתן לדחות עם הצעה למועד חלופי.</p>
              </>
            ) : (
              <p className="flex items-center gap-1.5 font-semibold text-ok">
                <CheckCircle2 className="h-4 w-4" /> החדר פנוי – ניתן לאשר
              </p>
            )}
          </section>
        )}

        {pending && mode === 'view' && (
          <section className="grid gap-3">
            <div className="grid grid-cols-2 gap-3">
              <Field label="מחיר סופי (₪)" hint={`הצעת מחיר אוטומטית: ${money(request.quotedPrice)}`}>
                <Input type="number" min={0} value={price} onChange={(e) => setPrice(e.target.value)} />
              </Field>
              <Field label="רכז/ת מלווה">
                <Select value={coordinatorId} onChange={(e) => setCoordinatorId(e.target.value)}>
                  <option value="">ללא</option>
                  {coordinators.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.fullName}
                    </option>
                  ))}
                </Select>
              </Field>
            </div>
            <Field label="הערות פנימיות">
              <Textarea value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="יוצגו לצוות בלבד" />
            </Field>
          </section>
        )}
        {pending && mode === 'reject' && (
          <Field label="סיבת הדחייה (תישלח למבקש/ת)">
            <Textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={4} autoFocus placeholder="לדוגמה: האולם תפוס במועד זה. נשמח להציע את יום ה׳ באותה שעה." />
          </Field>
        )}
        {request.status === 'rejected' && <p className="rounded-xl bg-red-50 p-3 text-sm text-red-800 dark:bg-red-950/30 dark:text-red-200">סיבת דחייה: {request.rejectionReason}</p>}
        {request.status === 'approved' && <p className="rounded-xl bg-emerald-50 p-3 text-sm text-emerald-800 dark:bg-emerald-950/30 dark:text-emerald-200">אושרה במחיר {money(request.finalPrice ?? 0)} ונוספה ליומן.</p>}

        <Attachments entity="request" entityId={request.id} />
      </div>
    </Drawer>
  )
}

// ── All bookings table ───────────────────────────────────────

function AllBookings() {
  const [q, setQ] = useState('')
  const [debounced, setDebounced] = useState('')
  const [range, setRange] = useState<'upcoming' | 'past' | 'all'>('upcoming')
  const [open, setOpen] = useState<Booking | null>(null)
  useEffect(() => {
    const t = setTimeout(() => setDebounced(q), 250)
    return () => clearTimeout(t)
  }, [q])
  const today = startOfDay(new Date())
  const query = useMemo(() => {
    if (range === 'upcoming') return { from: today.toISOString(), to: addDays(today, 90).toISOString() }
    if (range === 'past') return { from: addDays(today, -90).toISOString(), to: today.toISOString() }
    return { from: addDays(today, -365).toISOString(), to: addDays(today, 365).toISOString() }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [range])
  const { data = [], isLoading } = useQuery({
    queryKey: ['bookings', 'table', range, debounced],
    queryFn: () => api.get<Booking[]>('/bookings', { ...query, q: debounced, statuses: ['pending', 'confirmed', 'cancelled'], limit: 400 }),
  })
  const rows = range === 'past' ? [...data].reverse() : data

  return (
    <div className="card overflow-hidden">
      <div className="flex flex-wrap items-center gap-3 border-b border-line p-4">
        <div className="relative max-w-sm flex-1">
          <Search className="absolute start-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
          <Input className="ps-9" placeholder="חיפוש לפי כותרת, לקוח, רכז או חדר" value={q} onChange={(e) => setQ(e.target.value)} />
        </div>
        <Tabs
          value={range}
          onChange={setRange}
          items={[
            { value: 'upcoming', label: '90 יום קדימה' },
            { value: 'past', label: '90 יום אחורה' },
            { value: 'all', label: 'שנה ±' },
          ]}
        />
        <span className="ms-auto text-xs text-muted">{rows.length} הזמנות</span>
      </div>
      {isLoading ? (
        <PageLoader />
      ) : rows.length === 0 ? (
        <Empty title="לא נמצאו הזמנות" />
      ) : (
        <div className="scroll-thin overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-subtle/70 text-xs text-muted">
              <tr className="[&>th]:px-4 [&>th]:py-2.5 [&>th]:text-start [&>th]:font-medium">
                <th>תאריך ושעה</th>
                <th>כותרת</th>
                <th>חדר</th>
                <th>סוג</th>
                <th>לקוח / רכז</th>
                <th>סטטוס</th>
                <th className="text-end">מחיר</th>
                <th className="text-end">יתרה</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {rows.map((b) => (
                <tr key={b.id} onClick={() => setOpen(b)} className={clsx('cursor-pointer transition hover:bg-subtle [&>td]:px-4 [&>td]:py-2.5', b.status === 'cancelled' && 'text-muted line-through')}>
                  <td className="whitespace-nowrap tabular-nums">
                    {dateShort(b.startsAt)} · {timeRange(b.startsAt, b.endsAt)}
                  </td>
                  <td className="max-w-[240px] truncate font-medium">{b.title}</td>
                  <td className="whitespace-nowrap">{b.roomName}</td>
                  <td>
                    <span className={clsx('chip', typeStyles[b.type].bg)}>
                      <span className={clsx('h-1.5 w-1.5 rounded-full', typeStyles[b.type].dot)} />
                      {bookingTypeShort[b.type]}
                    </span>
                  </td>
                  <td className="max-w-[180px] truncate text-muted">{b.customerName || b.coordinatorName || '—'}</td>
                  <td>
                    <Badge tone={b.status === 'confirmed' ? 'green' : b.status === 'pending' ? 'amber' : 'gray'}>{bookingStatusLabel[b.status]}</Badge>
                  </td>
                  <td className="text-end tabular-nums">{b.price ? money(b.price) : '—'}</td>
                  <td className={clsx('text-end tabular-nums', b.price - b.paidAmount > 0 && 'font-semibold text-danger')}>{b.price ? money(b.price - b.paidAmount) : '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <BookingDialog open={!!open} booking={open} draft={null} onClose={() => setOpen(null)} />
    </div>
  )
}

// ── Cancellation requests (from coordinators / portal users) ──

function Cancellations() {
  const qc = useQueryClient()
  const [notes, setNotes] = useState<Record<string, string>>({})
  const { data = [], isLoading } = useQuery({
    queryKey: ['bookings', 'cancel-requests'],
    queryFn: () => api.get<Booking[]>('/bookings', { cancelRequested: 'true' }),
  })
  const resolve = useMutation({
    mutationFn: ({ id, approve }: { id: string; approve: boolean }) => api.post(`/bookings/${id}/cancel-request/resolve`, { approve, note: notes[id] ?? '' }),
    onSuccess: (_, v) => {
      toast.success(v.approve ? 'ההזמנה בוטלה והחדר שוחרר' : 'בקשת הביטול נדחתה – ההזמנה בתוקף')
      for (const k of ['bookings', 'dashboard', 'requests']) qc.invalidateQueries({ queryKey: [k] })
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  if (isLoading) return <PageLoader />
  if (data.length === 0)
    return (
      <div className="card">
        <Empty icon={<ClipboardList className="h-6 w-6" />} title="אין בקשות ביטול פתוחות" hint="כשרכז או לקוח מבקשים לבטל הזמנה מאושרת, הבקשה מופיעה כאן. עד ההחלטה – החדר נשאר שמור." />
      </div>
    )
  return (
    <div className="grid gap-3 md:grid-cols-2">
      {data.map((b) => (
        <article key={b.id} className="card p-4">
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              <p className="truncate font-semibold">{b.title}</p>
              <p className="text-xs text-muted">
                {b.roomName} · {dateShort(b.startsAt)} · {timeRange(b.startsAt, b.endsAt)}
              </p>
              <p className="text-xs text-muted">{b.coordinatorName ? `רכז: ${b.coordinatorName}` : b.customerName}</p>
            </div>
            <Badge tone="red">ביטול ממתין</Badge>
          </div>
          <p className="mt-3 rounded-xl bg-subtle p-3 text-sm">“{b.cancelRequestReason}”</p>
          {b.cancelRequestedAt && <p className="mt-1 text-[11px] text-muted">התבקש {relative(b.cancelRequestedAt)}</p>}
          <Input className="mt-3" placeholder="הערה למבקש/ת (לא חובה)" value={notes[b.id] ?? ''} onChange={(e) => setNotes({ ...notes, [b.id]: e.target.value })} />
          <div className="mt-3 flex justify-end gap-2">
            <Button variant="secondary" size="sm" icon={<X className="h-4 w-4" />} loading={resolve.isPending && resolve.variables?.id === b.id && !resolve.variables.approve} onClick={() => resolve.mutate({ id: b.id, approve: false })}>
              דחיית הביטול
            </Button>
            <Button variant="danger" size="sm" icon={<Check className="h-4 w-4" />} loading={resolve.isPending && resolve.variables?.id === b.id && resolve.variables.approve} onClick={() => resolve.mutate({ id: b.id, approve: true })}>
              אישור ביטול
            </Button>
          </div>
        </article>
      ))}
    </div>
  )
}
