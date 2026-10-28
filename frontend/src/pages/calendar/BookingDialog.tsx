import clsx from 'clsx'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { addMinutes, format } from 'date-fns'
import { AlertTriangle, CheckCircle2, CreditCard, Repeat, Trash2, XCircle } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { Attachments } from '@/components/Attachments'
import { CustomerPicker } from '@/components/CustomerPicker'
import { PaymentDialog } from '@/components/PaymentDialog'
import { toast } from '@/components/toast'
import { Badge, Button, Confirm, Field, Input, Modal, Select, Textarea } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { useCan } from '@/lib/auth'
import { dateTime, money, timeRange } from '@/lib/format'
import { bookingStatusLabel, bookingTypeLabel, typeStyles } from '@/lib/labels'
import { useActivities, useLookups, useUsers } from '@/lib/queries'
import type { Booking, BookingStatus, BookingType, Conflict, PriceQuote } from '@/lib/types'

export interface Draft {
  start: Date
  end: Date
  roomId?: string
}

interface Form {
  title: string
  type: BookingType
  status: BookingStatus
  roomId: string
  date: string
  startTime: string
  endTime: string
  coordinatorId: string
  customerId: string | null
  customerName: string | null
  activityId: string
  attendees: number
  equipment: string[]
  price: string // '' = automatic quote
  notes: string
  internalNotes: string
  repeatWeeks: number
}

const TYPES: BookingType[] = ['activity', 'rental', 'internal', 'maintenance', 'closed']

function toForm(b: Booking | null, d: Draft | null, defaultRoom: string): Form {
  const start = b ? new Date(b.startsAt) : d?.start ?? new Date()
  const end = b ? new Date(b.endsAt) : d?.end ?? addMinutes(start, 60)
  return {
    title: b?.title ?? '',
    type: b?.type ?? 'internal',
    status: b?.status ?? 'confirmed',
    roomId: b?.roomId ?? d?.roomId ?? defaultRoom,
    date: format(start, 'yyyy-MM-dd'),
    startTime: format(start, 'HH:mm'),
    endTime: format(end, 'HH:mm'),
    coordinatorId: b?.coordinatorId ?? '',
    customerId: b?.customerId ?? null,
    customerName: b?.customerName ?? null,
    activityId: b?.activityId ?? '',
    attendees: b?.attendees ?? 0,
    equipment: b?.equipment ?? [],
    price: b ? String(b.price) : '',
    notes: b?.notes ?? '',
    internalNotes: b?.internalNotes ?? '',
    repeatWeeks: 1,
  }
}

function range(f: Form) {
  const start = new Date(`${f.date}T${f.startTime}`)
  let end = new Date(`${f.date}T${f.endTime}`)
  if (end <= start) end = new Date(end.getTime() + 86400000) // crosses midnight
  return { start, end }
}

export function BookingDialog({ open, onClose, booking, draft }: { open: boolean; onClose: () => void; booking: Booking | null; draft: Draft | null }) {
  const qc = useQueryClient()
  const can = useCan()
  const canEdit = can('bookings:manage')
  const { rooms, equipment, roomById } = useLookups()
  const coordinators = useUsers('coordinator', canEdit).data ?? []
  const activities = useActivities(canEdit).data ?? []
  const [f, setF] = useState<Form>(() => toForm(booking, draft, rooms[0]?.id ?? ''))
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [check, setCheck] = useState<{ conflicts: Conflict[]; quote: PriceQuote } | null>(null)
  const [confirm, setConfirm] = useState<null | 'cancel' | 'cancelSeries' | 'delete'>(null)
  const [paying, setPaying] = useState(false)

  useEffect(() => {
    if (open) {
      setF(toForm(booking, draft, rooms[0]?.id ?? ''))
      setErrors({})
      setCheck(null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, booking?.id, draft])

  const set = <K extends keyof Form>(k: K, v: Form[K]) => setF((p) => ({ ...p, [k]: v }))
  const room = roomById.get(f.roomId)

  // Live conflict + price check (debounced).
  useEffect(() => {
    if (!open || !f.roomId || !f.date || !f.startTime || !f.endTime) return
    const { start, end } = range(f)
    if (!(end > start)) return
    const t = setTimeout(() => {
      api
        .post<{ conflicts: Conflict[]; quote: PriceQuote }>('/bookings/check', {
          roomId: f.roomId,
          startsAt: start.toISOString(),
          endsAt: end.toISOString(),
          excludeId: booking?.id,
          type: f.type,
          equipment: f.equipment,
        })
        .then(setCheck)
        .catch(() => setCheck(null))
    }, 300)
    return () => clearTimeout(t)
  }, [open, f.roomId, f.date, f.startTime, f.endTime, f.type, f.equipment, booking?.id])

  const activity = useMemo(() => activities.find((a) => a.id === f.activityId), [activities, f.activityId])

  const invalidate = () => {
    for (const k of ['bookings', 'dashboard', 'balances', 'availability']) qc.invalidateQueries({ queryKey: [k] })
  }

  const save = useMutation({
    mutationFn: async () => {
      const { start, end } = range(f)
      const body = {
        roomId: f.roomId,
        title: f.title,
        type: f.type,
        status: f.status,
        startsAt: start.toISOString(),
        endsAt: end.toISOString(),
        coordinatorId: f.coordinatorId || null,
        customerId: f.customerId,
        activityId: f.activityId || null,
        attendees: Number(f.attendees) || 0,
        equipment: f.equipment,
        price: f.price === '' ? null : Number(f.price),
        notes: f.notes,
        internalNotes: f.internalNotes,
        repeatWeeks: booking ? 0 : f.repeatWeeks,
      }
      return booking ? api.put<Booking>(`/bookings/${booking.id}`, body) : api.post<{ created: Booking[] }>('/bookings', body)
    },
    onSuccess: (res) => {
      const n = 'created' in res ? res.created.length : 1
      toast.success(booking ? 'ההזמנה עודכנה' : n > 1 ? `נוצרו ${n} הזמנות` : 'ההזמנה נוצרה')
      invalidate()
      onClose()
    },
    onError: (e) => {
      if (e instanceof ApiError) {
        setErrors(e.fields)
        if (e.conflicts.length) setCheck((c) => (c ? { ...c, conflicts: e.conflicts } : { conflicts: e.conflicts, quote: { hours: 0, billableHours: 0, base: 0, extras: 0, total: 0, lines: [] } }))
      }
      toast.error(errorMessage(e))
    },
  })

  const action = useMutation({
    mutationFn: async (kind: 'cancel' | 'cancelSeries' | 'delete') => {
      if (!booking) return
      if (kind === 'delete') return api.del(`/bookings/${booking.id}`)
      return api.post(`/bookings/${booking.id}/cancel`, undefined, { scope: kind === 'cancelSeries' ? 'series' : 'one' })
    },
    onSuccess: (_, kind) => {
      toast.success(kind === 'delete' ? 'ההזמנה נמחקה' : 'ההזמנה בוטלה')
      invalidate()
      setConfirm(null)
      onClose()
    },
    onError: (e) => toast.error(errorMessage(e)),
  })

  const conflicts = check?.conflicts ?? []
  const quote = check?.quote
  const readOnly = !canEdit
  const balance = booking ? booking.price - booking.paidAmount : 0

  return (
    <>
      <Modal
        open={open}
        onClose={onClose}
        size="lg"
        title={
          <span className="flex items-center gap-2">
            {booking ? 'עריכת הזמנה' : 'הזמנה חדשה'}
            {booking && <Badge tone={booking.status === 'confirmed' ? 'green' : booking.status === 'pending' ? 'amber' : 'gray'}>{bookingStatusLabel[booking.status]}</Badge>}
            {booking?.seriesId && (
              <Badge tone="blue">
                <Repeat className="h-3 w-3" /> סדרה
              </Badge>
            )}
          </span>
        }
        footer={
          readOnly ? (
            <Button variant="secondary" onClick={onClose}>
              סגירה
            </Button>
          ) : (
            <>
              {booking && booking.status !== 'cancelled' && (
                <div className="me-auto flex gap-1">
                  <Button variant="ghost" size="sm" icon={<XCircle className="h-4 w-4" />} onClick={() => setConfirm(booking.seriesId ? 'cancelSeries' : 'cancel')}>
                    ביטול הזמנה
                  </Button>
                  {booking.paidAmount === 0 && (
                    <Button variant="ghost" size="sm" className="text-danger" icon={<Trash2 className="h-4 w-4" />} onClick={() => setConfirm('delete')}>
                      מחיקה
                    </Button>
                  )}
                </div>
              )}
              <Button variant="secondary" onClick={onClose}>
                סגירה
              </Button>
              <Button onClick={() => save.mutate()} loading={save.isPending}>
                {booking ? 'שמירת שינויים' : 'יצירת הזמנה'}
              </Button>
            </>
          )
        }
      >
        <fieldset disabled={readOnly} className="grid gap-5 lg:grid-cols-[1fr_260px]">
          <div className="grid content-start gap-4">
            {/* Type selector, colour-coded like the legend */}
            <div>
              <span className="label">סוג הזמנה</span>
              <div className="flex flex-wrap gap-1.5">
                {TYPES.map((t) => (
                  <button
                    type="button"
                    key={t}
                    onClick={() => set('type', t)}
                    className={clsx(
                      'flex items-center gap-1.5 rounded-xl border px-3 py-1.5 text-xs font-medium transition',
                      f.type === t ? clsx(typeStyles[t].bg, typeStyles[t].border, 'border-2') : 'border-line text-muted hover:bg-subtle',
                    )}
                  >
                    <span className={clsx('h-2 w-2 rounded-full', typeStyles[t].dot)} />
                    {bookingTypeLabel[t]}
                  </button>
                ))}
              </div>
            </div>

            <Field label="כותרת" error={errors.title}>
              <Input value={f.title} onChange={(e) => set('title', e.target.value)} placeholder="לדוגמה: חוג כדורסל, כנס מתנדבים…" autoFocus={!booking} />
            </Field>

            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="חדר / חלל" error={errors.roomId}>
                <Select value={f.roomId} onChange={(e) => set('roomId', e.target.value)}>
                  {rooms.map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.name}
                    </option>
                  ))}
                </Select>
              </Field>
              <Field label="סטטוס">
                <Select value={f.status} onChange={(e) => set('status', e.target.value as BookingStatus)}>
                  <option value="confirmed">מאושר</option>
                  <option value="pending">ממתין לאישור (שיבוץ זמני)</option>
                  {booking && <option value="cancelled">בוטל</option>}
                </Select>
              </Field>
            </div>

            <div className="grid grid-cols-3 gap-3">
              <Field label="תאריך" error={errors.startsAt}>
                <Input type="date" value={f.date} onChange={(e) => set('date', e.target.value)} />
              </Field>
              <Field label="משעה">
                <Input type="time" step={900} value={f.startTime} onChange={(e) => set('startTime', e.target.value)} />
              </Field>
              <Field label="עד שעה" error={errors.endsAt}>
                <Input type="time" step={900} value={f.endTime} onChange={(e) => set('endTime', e.target.value)} />
              </Field>
            </div>

            {!booking && (
              <Field label="חזרה שבועית" hint={f.repeatWeeks > 1 ? `ייווצרו ${f.repeatWeeks} מופעים באותו יום ושעה` : 'מופע יחיד'}>
                <Select value={f.repeatWeeks} onChange={(e) => set('repeatWeeks', Number(e.target.value))}>
                  {[1, 2, 4, 6, 8, 10, 12, 16, 20, 26, 52].map((n) => (
                    <option key={n} value={n}>
                      {n === 1 ? 'ללא חזרה' : `כל שבוע · ${n} פעמים`}
                    </option>
                  ))}
                </Select>
              </Field>
            )}

            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="רכז/ת אחראי/ת">
                <Select value={f.coordinatorId} onChange={(e) => set('coordinatorId', e.target.value)}>
                  <option value="">ללא</option>
                  {coordinators.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.fullName}
                    </option>
                  ))}
                </Select>
              </Field>
              <Field label="פעילות (חוג)">
                <Select
                  value={f.activityId}
                  onChange={(e) => {
                    const a = activities.find((x) => x.id === e.target.value)
                    setF((p) => ({
                      ...p,
                      activityId: e.target.value,
                      title: p.title || a?.name || '',
                      coordinatorId: p.coordinatorId || a?.coordinatorId || '',
                      type: a ? 'activity' : p.type,
                    }))
                  }}
                >
                  <option value="">ללא</option>
                  {activities.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.name}
                    </option>
                  ))}
                </Select>
              </Field>
            </div>

            <div className="grid gap-3 sm:grid-cols-[1fr_120px]">
              <Field label="לקוח / מזמין">
                <CustomerPicker value={f.customerId} valueName={f.customerName} onChange={(id, name) => setF((p) => ({ ...p, customerId: id, customerName: name ?? null }))} />
              </Field>
              <Field label="משתתפים" error={errors.attendees} hint={room?.capacity ? `קיבולת ${room.capacity}` : undefined}>
                <Input type="number" min={0} value={f.attendees} onChange={(e) => set('attendees', Number(e.target.value))} />
              </Field>
            </div>

            {room && room.equipment.length > 0 && (
              <div>
                <span className="label">ציוד ושירותים</span>
                <div className="flex flex-wrap gap-1.5">
                  {room.equipment.map((code) => {
                    const it = equipment.find((e) => e.code === code)
                    const on = f.equipment.includes(code)
                    return (
                      <button
                        type="button"
                        key={code}
                        onClick={() => set('equipment', on ? f.equipment.filter((x) => x !== code) : [...f.equipment, code])}
                        className={clsx('rounded-xl border px-3 py-1.5 text-xs transition', on ? 'border-brand bg-brand-soft text-brand-strong' : 'border-line hover:bg-subtle')}
                      >
                        {it?.label ?? code}
                        {it && it.flatPrice > 0 && <span className="ms-1 text-muted">+{money(it.flatPrice)}</span>}
                      </button>
                    )
                  })}
                </div>
              </div>
            )}

            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="הערות (גלויות ללקוח)">
                <Textarea value={f.notes} onChange={(e) => set('notes', e.target.value)} rows={2} />
              </Field>
              <Field label="הערות פנימיות">
                <Textarea value={f.internalNotes} onChange={(e) => set('internalNotes', e.target.value)} rows={2} />
              </Field>
            </div>

            {booking && <Attachments entity="booking" entityId={booking.id} canDelete={canEdit} />}
          </div>

          {/* Side panel: availability, price, payments */}
          <aside className="grid content-start gap-3">
            <div className={clsx('rounded-2xl border p-4', conflicts.length ? 'border-danger/40 bg-red-50 dark:bg-red-950/30' : 'border-ok/30 bg-emerald-50 dark:bg-emerald-950/20')}>
              {conflicts.length ? (
                <>
                  <p className="flex items-center gap-1.5 text-sm font-semibold text-danger">
                    <AlertTriangle className="h-4 w-4" /> התנגשות במועד
                  </p>
                  <ul className="mt-2 space-y-1.5 text-xs">
                    {conflicts.map((c) => (
                      <li key={c.bookingId} className="rounded-lg bg-surface/70 px-2 py-1.5">
                        <p className="font-medium">{c.title}</p>
                        <p className="text-muted">
                          {timeRange(c.startsAt, c.endsAt)} · {c.reason === 'buffer' ? 'זמן חציצה' : c.reason === 'maintenance' ? 'חסימת תחזוקה' : 'חפיפה'}
                        </p>
                      </li>
                    ))}
                  </ul>
                </>
              ) : (
                <p className="flex items-center gap-1.5 text-sm font-semibold text-ok">
                  <CheckCircle2 className="h-4 w-4" /> החדר פנוי במועד זה
                </p>
              )}
              {room && <p className="mt-2 text-[11px] text-muted">זמן חציצה אחרי כל הזמנה: {room.bufferMinutes} דק׳</p>}
            </div>

            <div className="rounded-2xl border border-line p-4">
              <p className="text-sm font-semibold">תמחור</p>
              {quote && f.type === 'rental' ? (
                <ul className="mt-2 space-y-1 text-xs">
                  {(quote.lines ?? []).map((l) => (
                    <li key={l.label} className="flex justify-between">
                      <span className="text-muted">{l.label}</span>
                      <span>{money(l.amount)}</span>
                    </li>
                  ))}
                  <li className="flex justify-between border-t border-line pt-1 font-semibold">
                    <span>הצעת מחיר ({quote.billableHours} ש׳)</span>
                    <span>{money(quote.total)}</span>
                  </li>
                </ul>
              ) : (
                <p className="mt-1 text-xs text-muted">{f.type === 'rental' ? 'מחשב…' : 'ללא חיוב לסוג זה'}</p>
              )}
              <Field label="מחיר סופי (₪)" hint="ריק = לפי הצעת המחיר" className="mt-3">
                <Input type="number" min={0} value={f.price} placeholder={quote ? String(quote.total) : ''} onChange={(e) => set('price', e.target.value)} />
              </Field>
            </div>

            {booking && booking.price > 0 && (
              <div className="rounded-2xl border border-line p-4 text-sm">
                <p className="font-semibold">תשלומים</p>
                <div className="mt-2 space-y-1 text-xs">
                  <p className="flex justify-between">
                    <span className="text-muted">שולם</span>
                    <span>{money(booking.paidAmount)}</span>
                  </p>
                  <p className="flex justify-between font-semibold">
                    <span>יתרה</span>
                    <span className={balance > 0 ? 'text-danger' : 'text-ok'}>{money(balance)}</span>
                  </p>
                </div>
                {balance > 0 && can('payments:manage') && (
                  <Button size="sm" variant="soft" className="mt-3 w-full" icon={<CreditCard className="h-4 w-4" />} onClick={() => setPaying(true)}>
                    רישום תשלום
                  </Button>
                )}
              </div>
            )}

            {activity && <p className="text-xs text-muted">פעילות: {activity.name}</p>}
            {booking && <p className="text-[11px] text-muted">נוצר {dateTime(booking.createdAt)}</p>}
          </aside>
        </fieldset>
      </Modal>

      <Confirm
        open={confirm === 'cancel' || confirm === 'cancelSeries'}
        onClose={() => setConfirm(null)}
        title="ביטול הזמנה"
        message={
          confirm === 'cancelSeries' ? (
            <div className="space-y-3">
              <p>הזמנה זו היא חלק מסדרה חוזרת. מה לבטל?</p>
              <div className="flex gap-2">
                <Button size="sm" variant="secondary" onClick={() => action.mutate('cancel')}>
                  רק מופע זה
                </Button>
                <Button size="sm" variant="danger" onClick={() => action.mutate('cancelSeries')}>
                  מופע זה וכל הבאים
                </Button>
              </div>
            </div>
          ) : (
            'ההזמנה תסומן כמבוטלת והחדר יתפנה. ניתן לראות אותה בסינון סטטוס "בוטל".'
          )
        }
        confirmLabel="ביטול הזמנה"
        danger
        loading={action.isPending}
        onConfirm={() => action.mutate(confirm === 'cancelSeries' ? 'cancelSeries' : 'cancel')}
      />
      <Confirm
        open={confirm === 'delete'}
        onClose={() => setConfirm(null)}
        title="מחיקת הזמנה"
        message="המחיקה סופית ולא ניתנת לשחזור."
        confirmLabel="מחיקה"
        danger
        loading={action.isPending}
        onConfirm={() => action.mutate('delete')}
      />
      <PaymentDialog booking={booking} open={paying} onClose={() => setPaying(false)} />
    </>
  )
}
