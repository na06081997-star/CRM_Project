import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { addDays, format, isSameDay, isToday, parseISO, startOfDay } from 'date-fns'
import { ChevronLeft, ChevronRight, Clock, Info, LayoutDashboard, Send, Sparkles, XCircle } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { CancelDialog } from '@/components/CancelDialog'
import { SimpleHeader } from '@/components/layout/SimpleHeader'
import { MultiSelect } from '@/components/MultiSelect'
import { toast } from '@/components/toast'
import { Badge, Button, Empty, Field, IconButton, Input, Modal, PageLoader, Select, Tabs, Textarea } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { useCan, useUser } from '@/lib/auth'
import { dateLong, fmt, timeRange } from '@/lib/format'
import { typeStyles, weekdayShort } from '@/lib/labels'
import { useActivities, useLookups } from '@/lib/queries'
import { useSocket } from '@/lib/realtime'
import type { Block, Booking, BookingRequest, Room } from '@/lib/types'
import { TimeGrid, type Column } from '../calendar/TimeGrid'
import { visibleRange } from '../calendar/utils'
import { MyRequests } from '../portal/PortalPage'

type Tab = 'calendar' | 'requests' | 'mine'

/** Coordinator (רכז) area: availability calendar, own requests and activities. */
export function RakazPage() {
  const [tab, setTab] = useState<Tab>('calendar')
  const qc = useQueryClient()
  const user = useUser()
  const can = useCan()
  useSocket('/ws', (ev) => {
    if (ev.type === 'booking.changed') {
      qc.invalidateQueries({ queryKey: ['blocks'] })
      qc.invalidateQueries({ queryKey: ['bookings'] })
    }
    if (ev.type === 'request.changed') qc.invalidateQueries({ queryKey: ['requests'] })
    if (ev.type === 'notification') {
      qc.invalidateQueries({ queryKey: ['notifications'] })
      const d = ev.data as { subject?: string }
      if (d?.subject) toast.info(d.subject)
    }
  })

  return (
    <div className="min-h-full">
      <SimpleHeader subtitle="אזור רכז">
        {can('dashboard:view') && (
          <Link to="/admin" className="me-2 hidden sm:block">
            <Button size="sm" variant="secondary" icon={<LayoutDashboard className="h-4 w-4" />}>
              לממשק הניהול
            </Button>
          </Link>
        )}
      </SimpleHeader>
      <div className="bg-gradient-to-b from-brand-soft/70 to-transparent">
        <div className="mx-auto max-w-[1400px] px-4 pb-5 pt-7">
          <h1 className="text-2xl font-bold sm:text-3xl">שלום {user?.fullName.split(' ')[0]} 👋</h1>
          <p className="mt-1 text-muted">בדקו מה פנוי ביומן, בקשו מועד לפעילות שלכם ועקבו אחרי הבקשות.</p>
          <Tabs
            className="mt-5"
            value={tab}
            onChange={setTab}
            items={[
              { value: 'calendar', label: 'יומן זמינות' },
              { value: 'requests', label: 'הבקשות שלי' },
              { value: 'mine', label: 'הפעילויות שלי' },
            ]}
          />
        </div>
      </div>
      <main className="mx-auto max-w-[1400px] px-4 pb-16">
        {tab === 'calendar' && <AvailabilityCalendar onRequested={() => setTab('requests')} />}
        {tab === 'requests' && <MyRequests />}
        {tab === 'mine' && <MyActivities />}
      </main>
    </div>
  )
}

// ── Availability calendar ────────────────────────────────────

function AvailabilityCalendar({ onRequested }: { onRequested: () => void }) {
  const me = useUser()
  const { rooms, buildings } = useLookups()
  const [view, setView] = useState<'week' | 'rooms'>('rooms')
  const [anchor, setAnchor] = useState(startOfDay(new Date()))
  const [buildingId, setBuildingId] = useState('')
  const [roomIds, setRoomIds] = useState<string[]>([])
  const [draft, setDraft] = useState<{ start: Date; end: Date; roomId?: string } | null>(null)
  const [detail, setDetail] = useState<Block | null>(null)

  const bookable = rooms.filter((r) => r.isBookable && (!buildingId || r.buildingId === buildingId))
  const shownRooms = roomIds.length ? bookable.filter((r) => roomIds.includes(r.id)) : bookable
  const { from, to } = visibleRange(view === 'week' ? 'week' : 'day', anchor)

  const { data: blocks = [], isFetching } = useQuery({
    queryKey: ['blocks', from.toISOString(), to.toISOString(), buildingId, roomIds.join()],
    queryFn: () =>
      api.get<Block[]>('/availability/calendar', { from: from.toISOString(), to: to.toISOString(), buildingId: buildingId || undefined, roomIds }),
    placeholderData: (p) => p,
  })

  // TimeGrid speaks Booking; map blocks onto it (titles are already anonymised server-side).
  const asBookings = useMemo(
    () =>
      blocks.map(
        (b) =>
          ({
            ...b,
            coordinatorId: b.mine ? me?.id : null,
            customerName: null,
            seriesId: b.seriesId,
          }) as unknown as Booking,
      ),
    [blocks, me?.id],
  )
  const byId = useMemo(() => new Map(blocks.map((b) => [b.id, b])), [blocks])

  const columns: Column[] = useMemo(() => {
    const head = (d: Date) => (
      <div className="flex flex-col items-center leading-tight">
        <span className="text-[11px] font-medium text-muted">{weekdayShort[d.getDay()]}</span>
        <span className={clsx('mt-0.5 text-lg font-bold tabular-nums', isToday(d) && 'text-brand')}>{fmt(d, 'dd.MM')}</span>
      </div>
    )
    if (view === 'week') {
      const single = shownRooms.length === 1 ? shownRooms[0] : null
      return Array.from({ length: 7 }, (_, i) => {
        const d = addDays(from, i)
        return { key: d.toISOString(), day: d, header: head(d), highlight: isToday(d), roomId: single?.id, match: (b: Booking) => isSameDay(new Date(b.startsAt), d) }
      })
    }
    return shownRooms.map((r) => ({
      key: r.id,
      day: from,
      roomId: r.id,
      header: (
        <span className="flex items-center justify-center gap-1.5 text-xs font-semibold">
          <span className="h-2 w-2 rounded-full" style={{ background: r.color }} />
          <span className="truncate">{r.name}</span>
        </span>
      ),
      match: (b: Booking) => b.roomId === r.id,
    }))
  }, [view, from, shownRooms])

  const busy = (roomId: string, start: Date, end: Date) =>
    blocks.some((b) => !b.request && b.roomId === roomId && new Date(b.startsAt) < end && new Date(b.endsAt) > start)

  const step = (dir: 1 | -1) => setAnchor((a) => addDays(a, (view === 'week' ? 7 : 1) * dir))

  return (
    <>
      <div className="card -mt-1 flex flex-wrap items-end gap-3 p-4">
        <div className="flex items-center gap-1">
          <IconButton label="הקודם" onClick={() => step(-1)}>
            <ChevronRight className="h-5 w-5" />
          </IconButton>
          <Button size="sm" variant="secondary" onClick={() => setAnchor(startOfDay(new Date()))}>
            היום
          </Button>
          <IconButton label="הבא" onClick={() => step(1)}>
            <ChevronLeft className="h-5 w-5" />
          </IconButton>
        </div>
        <div className="min-w-[150px]">
          <span className="label">תאריך</span>
          <Input type="date" value={format(anchor, 'yyyy-MM-dd')} onChange={(e) => e.target.value && setAnchor(startOfDay(parseISO(e.target.value)))} />
        </div>
        <Tabs
          value={view}
          onChange={setView}
          items={[
            { value: 'rooms', label: 'יום לפי חדרים' },
            { value: 'week', label: 'שבוע' },
          ]}
        />
        {buildings.length > 1 && (
          <label className="min-w-[160px]">
            <span className="label">מבנה</span>
            <Select value={buildingId} onChange={(e) => (setBuildingId(e.target.value), setRoomIds([]))}>
              <option value="">כל המבנים</option>
              {buildings.map((b) => (
                <option key={b.id} value={b.id}>
                  {b.name}
                </option>
              ))}
            </Select>
          </label>
        )}
        <MultiSelect className="min-w-[180px]" label="חדר" allLabel="כל החדרים" value={roomIds} onChange={setRoomIds} options={bookable.map((r) => ({ value: r.id, label: r.name, color: r.color }))} />
        <p className="ms-auto text-sm font-semibold">{view === 'week' ? `${fmt(from, 'd.M')} – ${fmt(addDays(from, 6), 'd.M.yyyy')}` : dateLong(anchor)}</p>
      </div>

      <div className="my-3 flex flex-wrap items-center gap-x-5 gap-y-2 text-xs text-muted">
        <span className="flex items-center gap-1.5">
          <span className="h-3 w-3 rounded border-2 border-brand bg-brand-soft" /> שלי
        </span>
        {(['activity', 'rental', 'internal'] as const).map((t) => (
          <span key={t} className="flex items-center gap-1.5">
            <span className={clsx('h-3 w-3 rounded border-s-[3px]', typeStyles[t].bg, typeStyles[t].border)} />
            {{ activity: 'פעילות (רכז אחר)', rental: 'שכירות', internal: 'פעילות פנימית' }[t]}
          </span>
        ))}
        <span className="flex items-center gap-1.5">
          <span className="hatch h-3 w-3 rounded border border-line" /> לא זמין
        </span>
        <span className="flex items-center gap-1.5">
          <span className="h-3 w-3 rounded border border-dashed border-ink/50" /> הבקשה שלי ממתינה
        </span>
        <span className="ms-auto flex items-center gap-1 font-medium text-brand">
          <Info className="h-3.5 w-3.5" /> לחצו וגררו על משבצת פנויה כדי לבקש מועד
        </span>
      </div>

      {view === 'week' && shownRooms.length !== 1 && (
        <p className="mb-3 rounded-xl bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950/40 dark:text-amber-200">
          בתצוגה שבועית מוצגים כל החדרים יחד. בחרו חדר אחד בסינון כדי לראות את הזמינות שלו בלבד.
        </p>
      )}

      {rooms.length === 0 ? (
        <PageLoader />
      ) : (
        <div className="overflow-x-auto">
          <div style={{ minWidth: Math.max(640, columns.length * 130) }}>
            <TimeGrid
              columns={columns}
              bookings={asBookings}
              startHour={7}
              endHour={23}
              lens="coordinator"
              myId={me?.id}
              canEdit
              canMove={false}
              onCreate={(r) => {
                const roomId = r.roomId ?? shownRooms[0]?.id
                if (!roomId) return
                if (r.start < new Date()) return toast.error('לא ניתן לבקש מועד שעבר')
                if (busy(roomId, r.start, r.end)) return toast.error('המועד תפוס בחדר זה – בחרו משבצת פנויה')
                setDraft({ ...r, roomId })
              }}
              onOpen={(b) => {
                const blk = byId.get(b.id)
                if (blk?.mine && !blk.request) setDetail(blk)
                else toast.info(`${blk?.title ?? b.title} · ${timeRange(b.startsAt, b.endsAt)}`)
              }}
              onMove={() => undefined}
            />
          </div>
        </div>
      )}
      {isFetching && <p className="mt-2 text-xs text-muted">מעדכן…</p>}

      <ActivityRequestDialog draft={draft} rooms={rooms} onClose={() => setDraft(null)} onSent={onRequested} />
      <CancelDialog block={detail} onClose={() => setDetail(null)} />
    </>
  )
}

// ── Request dialog (פעילות באמצעות רכז) ───────────────────────

function ActivityRequestDialog({ draft, rooms, onClose, onSent }: { draft: { start: Date; end: Date; roomId?: string } | null; rooms: Room[]; onClose: () => void; onSent: () => void }) {
  const qc = useQueryClient()
  const activities = useActivities().data ?? []
  const { equipment } = useLookups()
  const [f, setF] = useState({ roomId: '', date: '', from: '', to: '', activityId: '', title: '', eventKind: 'פעילות', attendees: 0, tables: 0, chairs: 0, eq: [] as string[], notes: '' })
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [last, setLast] = useState<typeof draft>(null)
  if (draft && draft !== last) {
    setLast(draft)
    setErrors({})
    setF((p) => ({ ...p, roomId: draft.roomId ?? '', date: format(draft.start, 'yyyy-MM-dd'), from: format(draft.start, 'HH:mm'), to: format(draft.end, 'HH:mm'), eq: [] }))
  }
  const room = rooms.find((r) => r.id === f.roomId)
  const isHall = room?.kind === 'event_hall'

  const send = useMutation({
    mutationFn: () =>
      api.post<BookingRequest>('/requests', {
        route: 'activity',
        roomId: f.roomId,
        activityId: f.activityId || null,
        title: f.title,
        eventKind: f.eventKind,
        startsAt: new Date(`${f.date}T${f.from}`).toISOString(),
        endsAt: new Date(`${f.date}T${f.to}`).toISOString(),
        attendees: f.attendees,
        tablesCount: isHall ? f.tables : 0,
        chairsCount: isHall ? f.chairs : 0,
        equipment: f.eq,
        notes: f.notes,
      }),
    onSuccess: () => {
      toast.success('הבקשה נשלחה למנהל המתנ״ס לאישור')
      qc.invalidateQueries({ queryKey: ['requests'] })
      qc.invalidateQueries({ queryKey: ['blocks'] })
      onClose()
      onSent()
    },
    onError: (e) => {
      if (e instanceof ApiError) setErrors(e.fields)
      toast.error(errorMessage(e))
    },
  })

  if (!draft) return null
  return (
    <Modal
      open
      onClose={onClose}
      title="בקשת מועד · פעילות באמצעות רכז"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            ביטול
          </Button>
          <Button icon={<Send className="h-4 w-4" />} loading={send.isPending} disabled={!f.title.trim() && !f.activityId} onClick={() => send.mutate()}>
            שליחה לאישור
          </Button>
        </>
      }
    >
      <p className="mb-4 rounded-xl bg-subtle p-3 text-xs text-muted">זו בקשה בלבד – המועד אינו שמור עד לאישור המנהל. בקשות של רכזים אחרים אינן חוסמות את המועד ואינן מוצגות לך.</p>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="חדר" error={errors.roomId}>
          <Select value={f.roomId} onChange={(e) => setF({ ...f, roomId: e.target.value })}>
            {rooms
              .filter((r) => r.isBookable)
              .map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
          </Select>
        </Field>
        <Field label="פעילות" error={errors.activityId}>
          <Select
            value={f.activityId}
            onChange={(e) => {
              const a = activities.find((x) => x.id === e.target.value)
              setF({ ...f, activityId: e.target.value, title: f.title || a?.name || '' })
            }}
          >
            <option value="">פעילות חדשה / חד־פעמית</option>
            {activities.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="שם הפעילות" error={errors.title} className="sm:col-span-2">
          <Input value={f.title} onChange={(e) => setF({ ...f, title: e.target.value })} placeholder="לדוגמה: ערב הורים, טורניר, הרצאה" autoFocus />
        </Field>
        <div className="grid grid-cols-3 gap-3 sm:col-span-2">
          <Field label="תאריך" error={errors.startsAt}>
            <Input type="date" value={f.date} onChange={(e) => setF({ ...f, date: e.target.value })} />
          </Field>
          <Field label="משעה">
            <Input type="time" step={900} value={f.from} onChange={(e) => setF({ ...f, from: e.target.value })} />
          </Field>
          <Field label="עד שעה" error={errors.endsAt}>
            <Input type="time" step={900} value={f.to} onChange={(e) => setF({ ...f, to: e.target.value })} />
          </Field>
        </div>
        <Field label="סוג פעילות">
          <Select value={f.eventKind} onChange={(e) => setF({ ...f, eventKind: e.target.value })}>
            {['פעילות', 'חוג', 'הרצאה', 'טורניר', 'אירוע קהילתי', 'ישיבה', 'אחר'].map((k) => (
              <option key={k}>{k}</option>
            ))}
          </Select>
        </Field>
        <Field label="משתתפים" error={errors.attendees} hint={room?.capacity ? `קיבולת ${room.capacity}` : undefined}>
          <Input type="number" min={0} value={f.attendees} onChange={(e) => setF({ ...f, attendees: Number(e.target.value) })} />
        </Field>
        {isHall && (
          <>
            <Field label="כמות שולחנות">
              <Input type="number" min={0} value={f.tables} onChange={(e) => setF({ ...f, tables: Number(e.target.value) })} />
            </Field>
            <Field label="כמות כיסאות">
              <Input type="number" min={0} value={f.chairs} onChange={(e) => setF({ ...f, chairs: Number(e.target.value) })} />
            </Field>
          </>
        )}
        {room && room.equipment.length > 0 && (
          <div className="sm:col-span-2">
            <span className="label">דרישות (הגברה, תאורה, אוכל…)</span>
            <div className="flex flex-wrap gap-1.5">
              {room.equipment.map((code) => {
                const on = f.eq.includes(code)
                return (
                  <button
                    key={code}
                    type="button"
                    onClick={() => setF({ ...f, eq: on ? f.eq.filter((x) => x !== code) : [...f.eq, code] })}
                    className={clsx('rounded-xl border px-3 py-1.5 text-xs', on ? 'border-brand bg-brand-soft text-brand-strong' : 'border-line')}
                  >
                    {equipment.find((e) => e.code === code)?.label ?? code}
                  </button>
                )
              })}
            </div>
          </div>
        )}
        <Field label="בקשות מיוחדות / הערות" className="sm:col-span-2">
          <Textarea value={f.notes} onChange={(e) => setF({ ...f, notes: e.target.value })} />
        </Field>
      </div>
    </Modal>
  )
}

// ── My activities + cancellation requests ────────────────────

function MyActivities() {
  const [cancel, setCancel] = useState<Block | null>(null)
  const today = startOfDay(new Date())
  const { data = [], isLoading } = useQuery({
    queryKey: ['bookings', 'mine', today.toISOString()],
    queryFn: () => api.get<Booking[]>('/bookings', { from: today.toISOString(), to: addDays(today, 60).toISOString(), statuses: ['confirmed', 'pending'] }),
  })
  if (isLoading) return <PageLoader />
  if (data.length === 0)
    return (
      <div className="card mt-2">
        <Empty icon={<Sparkles className="h-6 w-6" />} title="אין פעילויות מאושרות בחודשיים הקרובים" hint="בקשו מועד ביומן הזמינות – לאחר אישור המנהל הפעילות תופיע כאן." />
      </div>
    )
  const groups = new Map<string, Booking[]>()
  for (const b of data) {
    const k = dateLong(b.startsAt)
    groups.set(k, [...(groups.get(k) ?? []), b])
  }
  return (
    <div className="mt-2 space-y-5">
      {[...groups.entries()].map(([day, list]) => (
        <section key={day}>
          <h3 className="mb-2 text-sm font-semibold text-muted">{day}</h3>
          <div className="grid gap-2 md:grid-cols-2">
            {list.map((b) => (
              <article key={b.id} className="card flex items-center gap-3 p-3">
                <span className={clsx('h-10 w-1 rounded-full', typeStyles[b.type].dot)} />
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium">{b.title}</p>
                  <p className="flex items-center gap-1 text-xs text-muted">
                    <Clock className="h-3 w-3" /> {timeRange(b.startsAt, b.endsAt)} · {b.roomName}
                  </p>
                </div>
                {b.status === 'pending' && <Badge tone="amber">שיבוץ זמני</Badge>}
                {b.cancelRequestedAt ? (
                  <Badge tone="red">ביטול ממתין לאישור</Badge>
                ) : (
                  <Button size="sm" variant="ghost" icon={<XCircle className="h-4 w-4" />} onClick={() => setCancel({ ...(b as unknown as Block), id: b.id, mine: true, request: false, cancelRequested: false })}>
                    בקשת ביטול
                  </Button>
                )}
              </article>
            ))}
          </div>
        </section>
      ))}
      <CancelDialog block={cancel} onClose={() => setCancel(null)} />
    </div>
  )
}
