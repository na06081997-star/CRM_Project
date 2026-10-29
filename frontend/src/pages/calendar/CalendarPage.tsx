import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { addDays, isSameDay, isToday, parseISO, startOfDay } from 'date-fns'
import { ChevronLeft, ChevronRight, FilterX, PanelRightClose, PanelRightOpen, Plus, Printer } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { MultiSelect } from '@/components/MultiSelect'
import { toast } from '@/components/toast'
import { Button, IconButton, Select, Spinner, Tabs } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { useCan, useUser } from '@/lib/auth'
import { fmt, timeRange } from '@/lib/format'
import { bookingStatusLabel, bookingTypeLabel, typeStyles, weekdayShort } from '@/lib/labels'
import { usePrefs } from '@/lib/prefs'
import { useLookups, useUsers } from '@/lib/queries'
import type { Block, Booking, BookingStatus, BookingType } from '@/lib/types'
import { BookingDialog, type Draft } from './BookingDialog'
import { MiniMonth } from './MiniMonth'
import { MonthView } from './MonthView'
import { TimeGrid, type Column, type Lens } from './TimeGrid'
import { byDay, step, visibleRange, type View } from './utils'
import { YearView } from './YearView'

const VIEWS: { value: View; label: string }[] = [
  { value: 'day', label: 'יום' },
  { value: 'week', label: 'שבוע' },
  { value: 'month', label: 'חודש' },
  { value: 'year', label: 'שנה' },
  { value: 'rooms', label: 'לפי חדר' },
  { value: 'coordinators', label: 'לפי רכז' },
]
const TYPES: BookingType[] = ['activity', 'rental', 'internal', 'maintenance', 'closed']
const STATUSES: BookingStatus[] = ['confirmed', 'pending', 'cancelled']

/**
 * mode="admin": the full management calendar.
 * mode="portal": the same views for coordinators / portal users, fed by the
 * anonymised availability API. Others' bookings show only type and
 * coordinator name; drawing on a free slot starts a booking request.
 */
export function CalendarPage({ mode = 'admin' }: { mode?: 'admin' | 'portal' }) {
  const portal = mode === 'portal'
  const nav = useNavigate()
  const [params, setParams] = useSearchParams()
  const qc = useQueryClient()
  const can = useCan()
  const me = useUser()
  const canEdit = !portal && can('bookings:manage')
  const named = can('requests:activity') || can('calendar:view') // may see coordinator names
  const { rooms, buildings } = useLookups()
  const staffCoordinators = useUsers('coordinator', canEdit).data ?? []
  const prefBuilding = usePrefs((s) => s.buildingId)
  const [panel, setPanel] = useState(true)
  const [dialog, setDialog] = useState<{ booking: Booking | null; draft: Draft | null } | null>(null)

  // ── URL state (shareable) ──
  const view = (params.get('view') as View) || 'week'
  const anchor = params.get('date') ? parseISO(params.get('date')!) : startOfDay(new Date())
  const list = (k: string) => params.get(k)?.split(',').filter(Boolean) ?? []
  const buildingId = params.get('building') ?? prefBuilding ?? ''
  const roomIds = list('rooms')
  const coordinatorId = params.get('coordinator') ?? ''
  const types = list('types') as BookingType[]
  const statuses = (params.has('statuses') ? list('statuses') : ['confirmed', 'pending']) as BookingStatus[]
  const lens: Lens = portal ? 'coordinator' : (params.get('lens') as Lens) || 'admin'
  const focusId = params.get('booking')

  const update = (patch: Record<string, string | string[] | null>) => {
    const next = new URLSearchParams(params)
    for (const [k, v] of Object.entries(patch)) {
      const s = Array.isArray(v) ? v.join(',') : v
      if (s === null || s === '') next.delete(k)
      else next.set(k, s)
    }
    setParams(next, { replace: true })
  }
  const go = (d: Date, v: View = view) => update({ date: fmt(d, 'yyyy-MM-dd'), view: v, booking: null })

  const { from, to } = visibleRange(view, anchor)
  const visibleRooms = useMemo(
    () => rooms.filter((r) => (!buildingId || r.buildingId === buildingId) && (roomIds.length === 0 || roomIds.includes(r.id))),
    [rooms, buildingId, roomIds],
  )

  const queryKey = ['bookings', from.toISOString(), to.toISOString(), buildingId, roomIds.join(), coordinatorId, types.join(), statuses.join()]
  const adminQuery = useQuery({
    enabled: !portal,
    queryKey,
    queryFn: () =>
      api.get<Booking[]>('/bookings', {
        from: from.toISOString(),
        to: to.toISOString(),
        buildingId: buildingId || undefined,
        roomIds,
        coordinatorId: coordinatorId || undefined,
        types,
        statuses,
      }),
    placeholderData: (prev) => prev,
  })

  // Portal: anonymised blocks mapped onto the Booking shape the views expect.
  const blocksQuery = useQuery({
    enabled: portal,
    queryKey: ['blocks', from.toISOString(), to.toISOString(), buildingId, roomIds.join()],
    queryFn: () => api.get<Block[]>('/availability/calendar', { from: from.toISOString(), to: to.toISOString(), buildingId: buildingId || undefined, roomIds }),
    placeholderData: (prev) => prev,
  })
  const blockById = useMemo(() => new Map((blocksQuery.data ?? []).map((b) => [b.id, b])), [blocksQuery.data])
  const portalBookings = useMemo(
    () =>
      (blocksQuery.data ?? [])
        .map(
          (b) =>
            ({
              ...b,
              coordinatorId: b.mine ? (me?.id ?? null) : b.coordinatorName ? `name:${b.coordinatorName}` : null,
              customerName: null,
              price: 0,
              paidAmount: 0,
            }) as unknown as Booking,
        )
        .filter((b) => (types.length === 0 || types.includes(b.type)) && (!coordinatorId || b.coordinatorId === coordinatorId)),
    [blocksQuery.data, me?.id, types, coordinatorId],
  )
  const bookings = portal ? portalBookings : (adminQuery.data ?? [])
  const isFetching = portal ? blocksQuery.isFetching : adminQuery.isFetching

  // Coordinator filter: real users for managers, names seen in the calendar for coordinators.
  const coordinators = useMemo(() => {
    if (!portal) return staffCoordinators.map((c) => ({ id: c.id, fullName: c.fullName }))
    const m = new Map<string, string>()
    if (me) m.set(me.id, `${me.fullName} (אני)`)
    for (const b of blocksQuery.data ?? []) if (!b.mine && b.coordinatorName) m.set(`name:${b.coordinatorName}`, b.coordinatorName)
    return [...m.entries()].map(([id, fullName]) => ({ id, fullName }))
  }, [portal, staffCoordinators, blocksQuery.data, me])

  // Open booking from deep link (?booking=id)
  useEffect(() => {
    if (!focusId || dialog) return
    const b = bookings.find((x) => x.id === focusId)
    if (b) setDialog({ booking: b, draft: null })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [focusId, bookings])

  const move = useMutation({
    mutationFn: ({ b, start, end, roomId }: { b: Booking; start: Date; end: Date; roomId?: string }) =>
      api.post<Booking>(`/bookings/${b.id}/move`, { roomId: roomId ?? b.roomId, startsAt: start.toISOString(), endsAt: end.toISOString() }),
    onMutate: async ({ b, start, end, roomId }) => {
      await qc.cancelQueries({ queryKey })
      const prev = qc.getQueryData<Booking[]>(queryKey)
      const room = rooms.find((r) => r.id === (roomId ?? b.roomId))
      qc.setQueryData<Booking[]>(queryKey, (old = []) =>
        old.map((x) => (x.id === b.id ? { ...x, startsAt: start.toISOString(), endsAt: end.toISOString(), roomId: roomId ?? x.roomId, roomName: room?.name ?? x.roomName } : x)),
      )
      return { prev }
    },
    onError: (e, _v, ctx) => {
      if (ctx?.prev) qc.setQueryData(queryKey, ctx.prev)
      if (e instanceof ApiError && e.conflicts.length) toast.error(`לא ניתן להזיז – מתנגש עם "${e.conflicts[0].title}" (${timeRange(e.conflicts[0].startsAt, e.conflicts[0].endsAt)})`)
      else toast.error(errorMessage(e))
    },
    onSuccess: (b) => toast.success(`"${b.title}" הוזז ל-${timeRange(b.startsAt, b.endsAt)}`),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: ['bookings'] })
      qc.invalidateQueries({ queryKey: ['dashboard'] })
    },
  })

  // ── Columns for time-grid views ──
  const columns: Column[] = useMemo(() => {
    const dayHeader = (d: Date) => (
      <div className="flex flex-col items-center leading-tight">
        <span className="text-[11px] font-medium text-muted">{weekdayShort[d.getDay()]}</span>
        <span className={clsx('mt-0.5 text-lg font-bold tabular-nums', isToday(d) && 'text-brand')}>{fmt(d, 'dd.MM')}</span>
      </div>
    )
    if (view === 'week')
      return Array.from({ length: 7 }, (_, i) => {
        const d = addDays(from, i)
        return { key: d.toISOString(), day: d, header: dayHeader(d), highlight: isToday(d), match: (b: Booking) => isSameDay(new Date(b.startsAt), d) }
      })
    if (view === 'day') return [{ key: 'd', day: from, header: dayHeader(from), highlight: isToday(from), match: () => true }]
    if (view === 'rooms')
      return visibleRooms.map((r) => ({
        key: r.id,
        day: from,
        roomId: r.id,
        header: (
          <div className="flex flex-col items-center gap-0.5 leading-tight">
            <span className="flex items-center gap-1.5 text-xs font-semibold">
              <span className="h-2 w-2 rounded-full" style={{ background: r.color }} />
              <span className="truncate">{r.name}</span>
            </span>
            <span className="text-[10px] text-muted">עד {r.capacity} איש</span>
          </div>
        ),
        match: (b: Booking) => b.roomId === r.id,
      }))
    if (view === 'coordinators') {
      const cols: Column[] = coordinators
        .filter((c) => !coordinatorId || c.id === coordinatorId)
        .map((c) => ({
          key: c.id,
          day: from,
          header: <span className="text-xs font-semibold">{c.fullName}</span>,
          match: (b: Booking) => b.coordinatorId === c.id,
        }))
      if (!coordinatorId) cols.push({ key: 'none', day: from, header: <span className="text-xs font-semibold text-muted">ללא רכז</span>, match: (b: Booking) => !b.coordinatorId })
      return cols
    }
    return []
  }, [view, from, visibleRooms, coordinators, coordinatorId])

  const busyDays = useMemo(() => new Set(byDay(bookings).keys()), [bookings])
  const filtersActive = roomIds.length > 0 || !!coordinatorId || types.length > 0 || params.has('statuses') || lens !== 'admin'

  const title = (() => {
    switch (view) {
      case 'year':
        return fmt(anchor, 'yyyy')
      case 'month':
        return fmt(anchor, 'MMMM yyyy')
      case 'week': {
        const end = addDays(from, 6)
        return from.getMonth() === end.getMonth() ? `${fmt(from, 'd')}–${fmt(end, 'd בMMMM yyyy')}` : `${fmt(from, 'd בMMM')} – ${fmt(end, 'd בMMM yyyy')}`
      }
      default:
        return fmt(anchor, 'EEEE, d בMMMM yyyy')
    }
  })()

  const requestSlot = (d: Draft) => {
    const roomId = d.roomId ?? visibleRooms[0]?.id
    if (d.start < new Date()) return toast.error('לא ניתן לבקש מועד שעבר')
    const taken = bookings.some(
      (b) => !blockById.get(b.id)?.request && b.roomId === roomId && new Date(b.startsAt) < d.end && new Date(b.endsAt) > d.start,
    )
    if (roomId && taken) return toast.error('המועד תפוס בחדר זה – בחרו משבצת פנויה')
    const q = new URLSearchParams({ start: d.start.toISOString(), end: d.end.toISOString() })
    if (roomId) q.set('room', roomId)
    nav(`/portal/new?${q}`)
  }
  const openCreate = (draft: Draft) => (portal ? requestSlot(draft) : canEdit && setDialog({ booking: null, draft }))
  const openBooking = (b: Booking) => {
    if (!portal) return setDialog({ booking: b, draft: null })
    const blk = blockById.get(b.id)
    if (blk?.request) nav(`/portal/requests?id=${blk.id.replace('request:', '')}`)
    else if (blk?.mine) nav(`/portal/bookings?id=${blk.id}`)
    else toast.info(`${b.title} · ${b.roomName} · ${timeRange(b.startsAt, b.endsAt)}`)
  }
  const views = portal && !named ? VIEWS.filter((v) => v.value !== 'coordinators') : VIEWS

  return (
    <div className="flex flex-col gap-4">
      {/* Header */}
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-bold tracking-tight">{portal ? 'יומן זמינות' : 'יומן הזמנות'}</h1>
        <div className="flex items-center gap-1">
          <IconButton label="הקודם" onClick={() => go(step(view, anchor, -1))}>
            <ChevronRight className="h-5 w-5" />
          </IconButton>
          <Button size="sm" variant="secondary" onClick={() => go(startOfDay(new Date()))}>
            היום
          </Button>
          <IconButton label="הבא" onClick={() => go(step(view, anchor, 1))}>
            <ChevronLeft className="h-5 w-5" />
          </IconButton>
        </div>
        <h2 className="text-lg font-semibold">{title}</h2>
        {isFetching && <Spinner className="h-4 w-4" />}
        <div className="ms-auto flex flex-wrap items-center gap-2">
          <Tabs value={view} onChange={(v) => update({ view: v })} items={views} />
          <IconButton label="הדפסה" onClick={() => window.print()} className="hidden sm:inline-flex">
            <Printer className="h-4 w-4" />
          </IconButton>
          {(canEdit || portal) && (
            <Button
              icon={<Plus className="h-4 w-4" />}
              onClick={() => {
                if (portal) return nav('/portal/new')
                const s = new Date(anchor)
                s.setHours(Math.max(8, new Date().getHours() + 1), 0, 0, 0)
                openCreate({ start: s, end: new Date(s.getTime() + 3600_000), roomId: visibleRooms[0]?.id })
              }}
            >
              {portal ? 'בקשה חדשה' : 'הזמנה חדשה'}
            </Button>
          )}
        </div>
      </div>

      {/* Filters (mirrors the mockup) */}
      <div className={clsx('card grid grid-cols-2 gap-3 p-4 md:grid-cols-3', portal ? 'xl:grid-cols-4' : 'xl:grid-cols-6')}>
        <label className="min-w-0">
          <span className="label">מבנה</span>
          <Select value={buildingId} onChange={(e) => update({ building: e.target.value, rooms: null })}>
            <option value="">כל המבנים</option>
            {buildings.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </Select>
        </label>
        <MultiSelect
          label="חדר"
          allLabel="כל החדרים"
          value={roomIds}
          onChange={(v) => update({ rooms: v })}
          options={rooms
            .filter((r) => !buildingId || r.buildingId === buildingId)
            .map((r) => ({ value: r.id, label: r.name, color: r.color, group: buildings.find((b) => b.id === r.buildingId)?.name }))}
        />
        {named && (
        <label className="min-w-0">
          <span className="label">רכז</span>
          <Select value={coordinatorId} onChange={(e) => update({ coordinator: e.target.value })}>
            <option value="">כל הרכזים</option>
            {coordinators.map((c) => (
              <option key={c.id} value={c.id}>
                {c.fullName}
              </option>
            ))}
          </Select>
        </label>
        )}
        {named && <MultiSelect label="סוג הזמנה" allLabel="כל הסוגים" value={types} onChange={(v) => update({ types: v })} options={TYPES.map((t) => ({ value: t, label: bookingTypeLabel[t] }))} />}
        {!portal && (
        <MultiSelect
          label="סטטוס"
          allLabel="כל הסטטוסים"
          value={statuses}
          onChange={(v) => update({ statuses: v.length ? v : STATUSES })}
          options={STATUSES.map((s) => ({ value: s, label: bookingStatusLabel[s] }))}
        />
        )}
        {!portal && (
        <label className="min-w-0">
          <span className="label">צפייה כ (הדגמה)</span>
          <Select value={lens} onChange={(e) => update({ lens: e.target.value === 'admin' ? null : e.target.value })}>
            <option value="admin">מנהל מערכת</option>
            <option value="coordinator">רכז ({me?.fullName ?? 'אני'}) – הדגשת שלי</option>
            <option value="public">קהל רחב – ללא פרטים פנימיים</option>
          </Select>
        </label>
        )}
      </div>

      {/* Legend */}
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-xs text-muted">
        {portal && (
          <span className="flex items-center gap-1.5 font-medium text-ink">
            <span className="h-3 w-3 rounded border-2 border-brand bg-brand-soft" /> שלי
          </span>
        )}
        {(named ? (['activity', 'rental', 'internal'] as BookingType[]) : (['internal'] as BookingType[])).map((t) => (
          <span key={t} className="flex items-center gap-1.5">
            <span className={clsx('h-3 w-3 rounded border-s-[3px]', typeStyles[t].bg, typeStyles[t].border)} />
            {named ? bookingTypeLabel[t] : 'תפוס'}
          </span>
        ))}
        <span className="flex items-center gap-1.5">
          <span className="h-3 w-3 rounded border border-dashed border-ink/50" /> {portal ? 'הבקשה שלי ממתינה' : 'ממתין לאישור'}
        </span>
        <span className="flex items-center gap-1.5">
          <span className="hatch h-3 w-3 rounded border border-line" /> סגור / לא זמין
        </span>
        {filtersActive && (
          <button className="flex items-center gap-1 text-brand hover:underline" onClick={() => update({ rooms: null, coordinator: null, types: null, statuses: null, lens: null })}>
            <FilterX className="h-3.5 w-3.5" /> ניקוי סינון
          </button>
        )}
        {canEdit && (view === 'week' || view === 'day' || view === 'rooms' || view === 'coordinators') && (
          <span className="ms-auto hidden md:inline">גררו על משבצת ריקה ליצירה · גררו הזמנה להזזה · ידית תחתונה לשינוי משך</span>
        )}
        {portal && (
          <span className="ms-auto hidden font-medium text-brand md:inline">גררו על משבצת פנויה כדי לבקש מועד · לחיצה על פעילות שלכם פותחת את פרטיה</span>
        )}
      </div>

      {/* Body */}
      <div className="flex items-start gap-4">
        {panel && (
          <aside className="card hidden w-64 shrink-0 space-y-5 p-4 xl:block">
            <MiniMonth value={anchor} onPick={(d) => go(d, view === 'year' || view === 'month' ? 'day' : view)} busyDays={busyDays} />
            <div>
              <p className="mb-2 text-xs font-semibold text-muted">חדרים</p>
              <div className="scroll-thin max-h-[40vh] space-y-0.5 overflow-y-auto">
                {rooms
                  .filter((r) => !buildingId || r.buildingId === buildingId)
                  .map((r) => {
                    const on = roomIds.length === 0 || roomIds.includes(r.id)
                    return (
                      <button
                        key={r.id}
                        onClick={() => {
                          const all = rooms.filter((x) => !buildingId || x.buildingId === buildingId).map((x) => x.id)
                          const cur = roomIds.length ? roomIds : all
                          const next = on ? cur.filter((x) => x !== r.id) : [...cur, r.id]
                          update({ rooms: next.length === all.length ? null : next })
                        }}
                        className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-start text-sm hover:bg-subtle"
                      >
                        <span className="flex h-4 w-4 items-center justify-center rounded border-2" style={{ borderColor: r.color, background: on ? r.color : 'transparent' }} />
                        <span className={clsx('truncate', !on && 'text-muted')}>{r.name}</span>
                      </button>
                    )
                  })}
              </div>
            </div>
          </aside>
        )}

        <div className="flex min-w-0 flex-1 flex-col">
          <div className="mb-2 hidden justify-end xl:flex">
            <button onClick={() => setPanel((p) => !p)} className="flex items-center gap-1 text-xs text-muted hover:text-ink">
              {panel ? <PanelRightClose className="h-4 w-4" /> : <PanelRightOpen className="h-4 w-4" />}
              {panel ? 'הסתרת לוח צד' : 'הצגת לוח צד'}
            </button>
          </div>

          {view === 'year' && <YearView year={anchor.getFullYear()} bookings={bookings} onDay={(d) => go(d, 'day')} onMonth={(d) => go(d, 'month')} />}
          {view === 'month' && (
            <MonthView
              from={from}
              anchor={anchor}
              bookings={bookings}
              onDay={(d) => go(d, 'day')}
              onOpen={openBooking}
              onCreateDay={(d) => {
                const s = new Date(d)
                s.setHours(10, 0, 0, 0)
                openCreate({ start: s, end: new Date(s.getTime() + 3600_000), roomId: visibleRooms[0]?.id })
              }}
            />
          )}
          {view !== 'year' && view !== 'month' && (
            <div className="overflow-x-auto">
              <div style={{ minWidth: Math.max(600, columns.length * (view === 'week' ? 110 : 150)) }}>
                <TimeGrid
                  columns={columns}
                  bookings={bookings}
                  startHour={7}
                  endHour={23}
                  lens={lens}
                  myId={me?.id}
                  canEdit={canEdit || portal}
                  canMove={canEdit}
                  focusId={focusId}
                  onCreate={(r) => openCreate({ ...r, roomId: r.roomId ?? visibleRooms[0]?.id })}
                  onOpen={openBooking}
                  onMove={(b, r) => move.mutate({ b, ...r })}
                />
              </div>
              {columns.length === 0 && <p className="p-6 text-center text-sm text-muted">אין עמודות להצגה – בדקו את הסינון</p>}
            </div>
          )}
        </div>
      </div>

      {!portal && (
      <BookingDialog
        open={!!dialog}
        booking={dialog?.booking ?? null}
        draft={dialog?.draft ?? null}
        onClose={() => {
          setDialog(null)
          if (focusId) update({ booking: null })
        }}
      />
      )}
    </div>
  )
}
