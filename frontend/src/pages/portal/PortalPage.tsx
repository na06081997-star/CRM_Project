import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { addDays, format } from 'date-fns'
import { CalendarSearch, CheckCircle2, Clock, History, LayoutDashboard, Search, Send, Users, XCircle } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Attachments } from '@/components/Attachments'
import { SimpleHeader } from '@/components/layout/SimpleHeader'
import { toast } from '@/components/toast'
import { Badge, Button, Empty, Field, Input, Modal, PageLoader, Select, Tabs, Textarea } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { useCan, useUser } from '@/lib/auth'
import { dateLong, dateShort, money, relative, time, timeRange } from '@/lib/format'
import { requestRouteLabel, requestStatusLabel, roomKindLabel } from '@/lib/labels'
import { CancelDialog } from '@/components/CancelDialog'
import { useLookups } from '@/lib/queries'
import { useSocket } from '@/lib/realtime'
import type { BookingRequest, RequestStatus, RoomAvailability } from '@/lib/types'

const statusTone: Record<RequestStatus, 'amber' | 'green' | 'red' | 'gray'> = { pending: 'amber', approved: 'green', rejected: 'red', cancelled: 'gray' }

export function PortalPage() {
  const [tab, setTab] = useState<'search' | 'mine'>('search')
  const qc = useQueryClient()
  const user = useUser()
  const can = useCan()
  useSocket('/ws', (ev) => {
    if (ev.type === 'request.changed') qc.invalidateQueries({ queryKey: ['requests'] })
    if (ev.type === 'notification') {
      qc.invalidateQueries({ queryKey: ['notifications'] })
      const d = ev.data as { subject?: string }
      if (d?.subject) toast.info(d.subject)
    }
  })

  return (
    <div className="min-h-full">
      <SimpleHeader subtitle="פורטל הזמנות">
        {can('dashboard:view') && (
          <Link to="/admin" className="me-2 hidden sm:block">
            <Button size="sm" variant="secondary" icon={<LayoutDashboard className="h-4 w-4" />}>
              לממשק הניהול
            </Button>
          </Link>
        )}
      </SimpleHeader>
      <div className="bg-gradient-to-b from-brand-soft/70 to-transparent">
        <div className="mx-auto max-w-6xl px-4 pb-6 pt-8">
          <h1 className="text-2xl font-bold sm:text-3xl">שלום {user?.fullName.split(' ')[0]} 👋</h1>
          <p className="mt-1 text-muted">מצאו חלל פנוי לאירוע, לסדנה או לחזרה – ושלחו בקשה בקלות.</p>
          <Tabs
            className="mt-5"
            value={tab}
            onChange={setTab}
            items={[
              { value: 'search', label: 'חיפוש והזמנה' },
              { value: 'mine', label: 'הבקשות שלי' },
            ]}
          />
        </div>
      </div>
      <main className="mx-auto max-w-6xl px-4 pb-16">{tab === 'search' ? <SearchRooms onSent={() => setTab('mine')} /> : <MyRequests />}</main>
    </div>
  )
}

function SearchRooms({ onSent }: { onSent: () => void }) {
  const [date, setDate] = useState(format(addDays(new Date(), 7), 'yyyy-MM-dd'))
  const [from, setFrom] = useState('18:00')
  const [to, setTo] = useState('21:00')
  const [capacity, setCapacity] = useState(0)
  const [kind, setKind] = useState('')
  const [pick, setPick] = useState<RoomAvailability | null>(null)
  const { equipmentLabel, buildingById } = useLookups()
  const { data, isFetching } = useQuery({
    queryKey: ['availability', date, from, to, capacity, kind],
    queryFn: () => api.get<RoomAvailability[]>('/availability', { date, from, to, capacity: capacity || undefined, kind: kind || undefined }),
  })
  const results = [...(data ?? [])].sort((a, b) => Number(b.windowFree) - Number(a.windowFree))

  return (
    <>
      <div className="card -mt-1 grid gap-3 p-4 sm:grid-cols-2 lg:grid-cols-[1.2fr_1fr_1fr_1fr_1.2fr]">
        <Field label="תאריך">
          <Input type="date" value={date} min={format(new Date(), 'yyyy-MM-dd')} onChange={(e) => setDate(e.target.value)} />
        </Field>
        <Field label="משעה">
          <Input type="time" step={900} value={from} onChange={(e) => setFrom(e.target.value)} />
        </Field>
        <Field label="עד שעה">
          <Input type="time" step={900} value={to} onChange={(e) => setTo(e.target.value)} />
        </Field>
        <Field label="משתתפים">
          <Input type="number" min={0} value={capacity || ''} placeholder="כמה?" onChange={(e) => setCapacity(Number(e.target.value))} />
        </Field>
        <Field label="סוג חלל">
          <Select value={kind} onChange={(e) => setKind(e.target.value)}>
            <option value="">הכל</option>
            {Object.entries(roomKindLabel).map(([k, v]) => (
              <option key={k} value={k}>
                {v}
              </option>
            ))}
          </Select>
        </Field>
      </div>

      <div className="mb-3 mt-6 flex items-center gap-2 text-sm text-muted">
        <Search className="h-4 w-4" /> {isFetching ? 'מחפש…' : `${results.filter((r) => r.windowFree).length} חללים פנויים ב${dateLong(`${date}T12:00:00`)}, ${from}–${to}`}
      </div>

      {!data ? (
        <PageLoader />
      ) : results.length === 0 ? (
        <div className="card">
          <Empty icon={<CalendarSearch className="h-6 w-6" />} title="לא נמצאו חללים מתאימים" hint="נסו תאריך או קיבולת אחרים" />
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {results.map((r) => (
            <article key={r.room.id} className={clsx('card flex flex-col overflow-hidden transition', r.windowFree ? 'hover:shadow-pop' : 'opacity-75')}>
              <div className="h-1.5" style={{ background: r.room.color }} />
              <div className="flex flex-1 flex-col p-4">
                <div className="flex items-start justify-between gap-2">
                  <div>
                    <h3 className="font-semibold">{r.room.name}</h3>
                    <p className="text-xs text-muted">
                      {buildingById.get(r.room.buildingId)?.name} · {roomKindLabel[r.room.kind]}
                    </p>
                  </div>
                  {r.windowFree ? (
                    <Badge tone="green">
                      <CheckCircle2 className="h-3 w-3" /> פנוי
                    </Badge>
                  ) : (
                    <Badge tone="red">
                      <XCircle className="h-3 w-3" /> תפוס
                    </Badge>
                  )}
                </div>
                <div className="mt-3 flex flex-wrap gap-3 text-xs text-muted">
                  <span className="flex items-center gap-1">
                    <Users className="h-3.5 w-3.5" /> עד {r.room.capacity}
                  </span>
                  <span>{money(r.room.hourlyRate)} לשעה</span>
                  {r.room.minHours > 1 && <span>מינימום {r.room.minHours} ש׳</span>}
                </div>
                {r.room.equipment.length > 0 && (
                  <p className="mt-2 text-xs text-muted">ציוד: {r.room.equipment.map(equipmentLabel).join(' · ')}</p>
                )}
                <div className="mt-3">
                  <p className="mb-1.5 text-[11px] font-medium text-muted">זמנים פנויים ביום זה</p>
                  <div className="flex flex-wrap gap-1">
                    {(r.freeSlots ?? []).slice(0, 6).map((s) => (
                      <button
                        key={s.start}
                        onClick={() => {
                          setFrom(time(s.start))
                          const end = Math.min(new Date(s.end).getTime(), new Date(s.start).getTime() + 3 * 3600_000)
                          setTo(time(new Date(end)))
                        }}
                        className="rounded-lg bg-emerald-50 px-2 py-0.5 text-[11px] tabular-nums text-emerald-700 hover:bg-emerald-100 dark:bg-emerald-950 dark:text-emerald-300"
                      >
                        {timeRange(s.start, s.end)}
                      </button>
                    ))}
                    {(r.freeSlots ?? []).length === 0 && <span className="text-[11px] text-muted">אין זמנים פנויים</span>}
                  </div>
                </div>
                <div className="mt-auto flex items-center justify-between pt-4">
                  <span className="text-sm">
                    {r.estimatedCost != null && (
                      <>
                        <span className="text-xs text-muted">הערכה: </span>
                        <b>{money(r.estimatedCost)}</b>
                      </>
                    )}
                  </span>
                  <Button size="sm" disabled={!r.windowFree} icon={<Send className="h-3.5 w-3.5" />} onClick={() => setPick(r)}>
                    שליחת בקשה
                  </Button>
                </div>
              </div>
            </article>
          ))}
        </div>
      )}
      <RequestDialog pick={pick} date={date} from={from} to={to} attendees={capacity} onClose={() => setPick(null)} onSent={onSent} />
    </>
  )
}

function RequestDialog({ pick, date, from, to, attendees, onClose, onSent }: { pick: RoomAvailability | null; date: string; from: string; to: string; attendees: number; onClose: () => void; onSent: () => void }) {
  const qc = useQueryClient()
  const user = useUser()
  const { equipment } = useLookups()
  const [title, setTitle] = useState('')
  const [eventKind, setEventKind] = useState('אירוע פרטי')
  const [count, setCount] = useState(attendees)
  const [eq, setEq] = useState<string[]>([])
  const [notes, setNotes] = useState('')
  const [phone, setPhone] = useState(user?.phone ?? '')
  const [errors, setErrors] = useState<Record<string, string>>({})
  useEffect(() => {
    if (pick) {
      setCount(attendees)
      setEq([])
      setErrors({})
    }
  }, [pick, attendees])

  const send = useMutation({
    mutationFn: () =>
      api.post<BookingRequest>('/requests', {
        roomId: pick!.room.id,
        title,
        eventKind,
        startsAt: new Date(`${date}T${from}`).toISOString(),
        endsAt: new Date(`${date}T${to}`).toISOString(),
        attendees: count,
        equipment: eq,
        notes,
        phone,
      }),
    onSuccess: () => {
      toast.success('הבקשה נשלחה! נעדכן אתכם ברגע שתאושר')
      qc.invalidateQueries({ queryKey: ['requests'] })
      qc.invalidateQueries({ queryKey: ['availability'] })
      onClose()
      onSent()
    },
    onError: (e) => {
      if (e instanceof ApiError) setErrors(e.fields)
      toast.error(errorMessage(e))
    },
  })

  if (!pick) return null
  const extras = eq.reduce((s, c) => s + (equipment.find((e) => e.code === c)?.flatPrice ?? 0), 0)
  return (
    <Modal
      open
      onClose={onClose}
      title={`בקשת הזמנה · ${pick.room.name}`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            ביטול
          </Button>
          <Button onClick={() => send.mutate()} loading={send.isPending} disabled={!title.trim()} icon={<Send className="h-4 w-4" />}>
            שליחת הבקשה
          </Button>
        </>
      }
    >
      <div className="mb-4 flex flex-wrap items-center gap-3 rounded-xl bg-subtle p-3 text-sm">
        <Clock className="h-4 w-4 text-muted" />
        <span>
          {dateLong(`${date}T12:00:00`)} · {from}–{to}
        </span>
        {pick.estimatedCost != null && <span className="ms-auto font-semibold">כ־{money(pick.estimatedCost + extras)}</span>}
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="שם האירוע" error={errors.title} className="sm:col-span-2">
          <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="לדוגמה: יום הולדת 40" autoFocus />
        </Field>
        <Field label="סוג">
          <Select value={eventKind} onChange={(e) => setEventKind(e.target.value)}>
            {['אירוע פרטי', 'סדנה', 'הרצאה', 'חזרה', 'ישיבה', 'אירוע קהילתי', 'אחר'].map((k) => (
              <option key={k}>{k}</option>
            ))}
          </Select>
        </Field>
        <Field label="מספר משתתפים" error={errors.attendees} hint={`קיבולת: ${pick.room.capacity}`}>
          <Input type="number" min={0} value={count} onChange={(e) => setCount(Number(e.target.value))} />
        </Field>
        <Field label="טלפון ליצירת קשר" className="sm:col-span-2">
          <Input dir="ltr" value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="050-0000000" />
        </Field>
        {pick.room.equipment.length > 0 && (
          <div className="sm:col-span-2">
            <span className="label">ציוד ושירותים נוספים</span>
            <div className="flex flex-wrap gap-1.5">
              {pick.room.equipment.map((code) => {
                const it = equipment.find((e) => e.code === code)
                const on = eq.includes(code)
                return (
                  <button
                    key={code}
                    type="button"
                    onClick={() => setEq(on ? eq.filter((x) => x !== code) : [...eq, code])}
                    className={clsx('rounded-xl border px-3 py-1.5 text-xs', on ? 'border-brand bg-brand-soft text-brand-strong' : 'border-line')}
                  >
                    {it?.label ?? code} {it && it.flatPrice > 0 && <span className="text-muted">+{money(it.flatPrice)}</span>}
                  </button>
                )
              })}
            </div>
          </div>
        )}
        <Field label="הערות ובקשות מיוחדות" className="sm:col-span-2" error={errors.startsAt}>
          <Textarea value={notes} onChange={(e) => setNotes(e.target.value)} />
        </Field>
      </div>
      {pick.room.rules && <p className="mt-3 text-xs text-muted">כללי החדר: {pick.room.rules}</p>}
    </Modal>
  )
}

export function MyRequests() {
  const qc = useQueryClient()
  const [cancelBooking, setCancelBooking] = useState<BookingRequest | null>(null)
  const { data = [], isLoading } = useQuery({ queryKey: ['requests', 'mine'], queryFn: () => api.get<BookingRequest[]>('/requests') })
  const [open, setOpen] = useState<string | null>(null)
  const cancel = useMutation({
    mutationFn: (id: string) => api.post(`/requests/${id}/cancel`),
    onSuccess: () => {
      toast.success('הבקשה בוטלה')
      qc.invalidateQueries({ queryKey: ['requests'] })
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  if (isLoading) return <PageLoader />
  if (data.length === 0)
    return (
      <div className="card mt-2">
        <Empty icon={<History className="h-6 w-6" />} title="עדיין לא שלחתם בקשות" hint="חפשו חלל פנוי ושלחו בקשה – היא תופיע כאן עם סטטוס בזמן אמת." />
      </div>
    )

  return (
    <div className="mt-2 space-y-3">
      {data.map((r) => (
        <article key={r.id} className="card p-4">
          <div className="flex flex-wrap items-start gap-3">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <h3 className="font-semibold">{r.title}</h3>
                {r.bookingStatus === 'cancelled' ? <Badge tone="gray">ההזמנה בוטלה</Badge> : <Badge tone={statusTone[r.status]}>{requestStatusLabel[r.status]}</Badge>}
                {r.route === 'activity' && <Badge tone="teal">{requestRouteLabel.activity}</Badge>}
                {r.cancelRequested && <Badge tone="red">ביטול ממתין לאישור</Badge>}
              </div>
              <p className="mt-1 text-sm text-muted">
                {r.roomName} · {dateShort(r.startsAt)} · {timeRange(r.startsAt, r.endsAt)} · {r.attendees} משתתפים
              </p>
              <p className="mt-0.5 text-xs text-muted">נשלחה {relative(r.createdAt)}</p>
            </div>
            {r.route !== 'activity' && (
              <div className="text-end">
                <p className="text-xs text-muted">{r.status === 'approved' ? 'מחיר סופי' : 'הערכת מחיר'}</p>
                <p className="text-lg font-bold">{money(r.finalPrice ?? r.quotedPrice)}</p>
              </div>
            )}
          </div>
          {r.status === 'pending' && (
            <div className="mt-3 flex items-center gap-2 rounded-xl bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950/40 dark:text-amber-200">
              <Clock className="h-4 w-4" /> הבקשה בבדיקה אצל צוות המתנ״ס. תקבלו הודעה עם ההחלטה.
            </div>
          )}
          {r.status === 'approved' && r.bookingStatus !== 'cancelled' && (
            <div className="mt-3 flex items-center gap-2 rounded-xl bg-emerald-50 px-3 py-2 text-xs text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200">
              <CheckCircle2 className="h-4 w-4" /> ההזמנה אושרה ושוריינה ביומן. פרטי תשלום יישלחו אליכם.
            </div>
          )}
          {r.status === 'rejected' && r.rejectionReason && (
            <div className="mt-3 rounded-xl bg-red-50 px-3 py-2 text-xs text-red-800 dark:bg-red-950/40 dark:text-red-200">סיבה: {r.rejectionReason}</div>
          )}
          <div className="mt-3 flex gap-2">
            <Button size="sm" variant="ghost" onClick={() => setOpen(open === r.id ? null : r.id)}>
              {open === r.id ? 'הסתרת קבצים' : 'קבצים מצורפים'}
            </Button>
            {r.status === 'pending' && (
              <Button size="sm" variant="ghost" className="text-danger" onClick={() => cancel.mutate(r.id)} loading={cancel.isPending && cancel.variables === r.id}>
                ביטול הבקשה
              </Button>
            )}
            {r.status === 'approved' && r.bookingId && r.bookingStatus !== 'cancelled' && !r.cancelRequested && (
              <Button size="sm" variant="ghost" className="text-danger" onClick={() => setCancelBooking(r)}>
                בקשת ביטול
              </Button>
            )}
          </div>
          {open === r.id && (
            <div className="mt-3">
              <Attachments entity="request" entityId={r.id} canDelete={false} />
            </div>
          )}
        </article>
      ))}
      <CancelDialog
        block={cancelBooking && { id: cancelBooking.bookingId!, title: cancelBooking.title, startsAt: cancelBooking.startsAt, endsAt: cancelBooking.endsAt, roomName: cancelBooking.roomName, cancelRequested: false }}
        onClose={() => setCancelBooking(null)}
      />
    </div>
  )
}
