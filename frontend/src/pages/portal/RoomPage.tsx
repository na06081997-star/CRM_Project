import clsx from 'clsx'
import { useQuery } from '@tanstack/react-query'
import { addDays, isSameDay, isToday, parseISO, startOfWeek } from 'date-fns'
import { ArrowRight, CheckCircle2, ChevronLeft, ChevronRight } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { toast } from '@/components/toast'
import { Badge, Button, PageLoader } from '@/components/ui'
import { api } from '@/lib/api'
import { useUser } from '@/lib/auth'
import { dateShort, fmt, minutesLabel, time } from '@/lib/format'
import { weekdayShort } from '@/lib/labels'
import { useLookups } from '@/lib/queries'
import type { Block, Booking } from '@/lib/types'
import { TimeGrid, type Column } from '../calendar/TimeGrid'
import { WEEK } from '../calendar/utils'

/** Mockup portal screen 2: a room's week, pick a free range, continue to the request. */
export function RoomPage() {
  const { id = '' } = useParams()
  const [params] = useSearchParams()
  const nav = useNavigate()
  const me = useUser()
  const { roomById, equipmentLabel } = useLookups()
  const room = roomById.get(id)
  const [weekStart, setWeekStart] = useState(() => startOfWeek(params.get('date') ? parseISO(params.get('date')!) : new Date(), WEEK))
  const [sel, setSel] = useState<{ start: Date; end: Date } | null>(null)
  const to = addDays(weekStart, 7)

  const { data: blocks = [], isLoading } = useQuery({
    queryKey: ['blocks', 'room', id, weekStart.toISOString()],
    queryFn: () => api.get<Block[]>('/availability/calendar', { from: weekStart.toISOString(), to: to.toISOString(), roomIds: [id] }),
    placeholderData: (p) => p,
  })

  const items = useMemo(() => {
    const list = blocks.map((b) => ({ ...b, coordinatorId: b.mine ? me?.id : null, customerName: null }) as unknown as Booking)
    if (sel)
      list.push({
        id: 'selection',
        roomId: id,
        roomName: room?.name ?? '',
        title: 'הבחירה שלך',
        type: 'activity',
        status: 'confirmed',
        startsAt: sel.start.toISOString(),
        endsAt: sel.end.toISOString(),
        coordinatorId: me?.id ?? null,
      } as unknown as Booking)
    return list
  }, [blocks, sel, id, room?.name, me?.id])

  const columns: Column[] = useMemo(
    () =>
      Array.from({ length: 7 }, (_, i) => {
        const d = addDays(weekStart, i)
        return {
          key: d.toISOString(),
          day: d,
          roomId: id,
          highlight: isToday(d),
          header: (
            <div className="flex flex-col items-center leading-tight">
              <span className="text-[11px] font-medium text-muted">{weekdayShort[d.getDay()]}</span>
              <span className={clsx('mt-0.5 text-lg font-bold tabular-nums', isToday(d) && 'text-brand')}>{fmt(d, 'dd.MM')}</span>
            </div>
          ),
          match: (b: Booking) => isSameDay(new Date(b.startsAt), d),
        }
      }),
    [weekStart, id],
  )

  const busy = (s: Date, e: Date) => blocks.some((b) => !b.request && new Date(b.startsAt) < e && new Date(b.endsAt) > s)
  const minutes = sel ? Math.round((sel.end.getTime() - sel.start.getTime()) / 60000) : 0

  if (!room) return <PageLoader />

  return (
    <div className="space-y-5">
      <Link to="/portal/search" className="flex items-center gap-1 text-sm text-brand hover:underline">
        <ArrowRight className="h-4 w-4" /> חזרה לתוצאות
      </Link>
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold">מתי {room.name} פנוי?</h1>
          <p className="mt-1 text-sm text-muted">
            {room.equipment.map(equipmentLabel).join(' · ')}
            {room.equipment.length > 0 && ' · '}עד {room.capacity} משתתפים · בקשה בכפוף לאישור מנהל
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="sm" icon={<ChevronRight className="h-4 w-4" />} onClick={() => setWeekStart(addDays(weekStart, -7))} disabled={weekStart <= startOfWeek(new Date(), WEEK)}>
            שבוע קודם
          </Button>
          <span className="rounded-xl border border-line bg-surface px-3 py-1.5 text-sm font-semibold">
            {fmt(weekStart, 'd')}–{fmt(addDays(weekStart, 6), 'd בMMMM')}
          </span>
          <Button variant="secondary" size="sm" onClick={() => setWeekStart(addDays(weekStart, 7))}>
            שבוע הבא <ChevronLeft className="h-4 w-4" />
          </Button>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-xs text-muted">
        <span className="flex items-center gap-1.5">
          <span className="h-3 w-3 rounded border border-line bg-surface" /> פנוי
        </span>
        <span className="flex items-center gap-1.5">
          <span className="h-3 w-3 rounded border-s-[3px] border-internal bg-internal-soft" /> תפוס
        </span>
        <span className="flex items-center gap-1.5">
          <span className="h-3 w-3 rounded border-2 border-brand bg-brand-soft" /> הבחירה שלך
        </span>
        <span className="flex items-center gap-1.5">
          <span className="hatch h-3 w-3 rounded border border-line" /> לא זמין
        </span>
        <span className="ms-auto font-medium text-brand">גררו על השעות הפנויות כדי לבחור התחלה וסיום</span>
      </div>

      <div className="grid gap-5 xl:grid-cols-[1fr_300px]">
        <div className="overflow-x-auto">
          <div style={{ minWidth: 720 }}>
            {isLoading ? (
              <PageLoader />
            ) : (
              <TimeGrid
                columns={columns}
                bookings={items}
                startHour={7}
                endHour={23}
                lens="coordinator"
                myId={me?.id}
                canEdit
                canMove={false}
                onCreate={(r) => {
                  if (r.start < new Date()) return toast.error('לא ניתן לבחור מועד שעבר')
                  if (busy(r.start, r.end)) return toast.error('לא ניתן לבחור רצף שחוצה אירוע קיים או חסימה')
                  setSel({ start: r.start, end: r.end })
                }}
                onOpen={(b) => (b.id === 'selection' ? setSel(null) : toast.info(`${b.title} · ${time(b.startsAt)}–${time(b.endsAt)}`))}
                onMove={() => undefined}
              />
            )}
          </div>
        </div>

        <aside className="space-y-4">
          <div className="rounded-2xl border border-brand/25 bg-brand-soft/40 p-5">
            <h3 className="font-semibold">הזמן שבחרת</h3>
            {sel ? (
              <>
                <dl className="mt-3 space-y-2 text-sm">
                  <Row k="חלל" v={room.name} />
                  <Row k="תאריך" v={dateShort(sel.start) + '.' + sel.start.getFullYear()} />
                  <Row k="שעות" v={`${time(sel.start)}–${time(sel.end)}`} />
                  <Row k="משך" v={minutesLabel(minutes)} />
                </dl>
                <Badge tone="green" className="mt-3">
                  <CheckCircle2 className="h-3 w-3" /> פנוי לבקשה
                </Badge>
                <Button
                  className="mt-4 w-full"
                  onClick={() => nav(`/portal/new?room=${id}&start=${encodeURIComponent(sel.start.toISOString())}&end=${encodeURIComponent(sel.end.toISOString())}`)}
                >
                  המשך לפרטי הבקשה
                </Button>
                <p className="mt-2 text-xs text-muted">המחיר הסופי יימסר על ידי המתנ״ס לאחר בדיקה.</p>
              </>
            ) : (
              <p className="mt-2 text-sm text-muted">גררו ביומן על רצף שעות פנוי. אפשר ללחוץ על הבחירה כדי לבטל אותה.</p>
            )}
          </div>
          <div className="card p-5">
            <h3 className="font-semibold">הזמן המבוקש תפוס?</h3>
            <p className="mt-1 text-sm text-muted">בחרו תא פנוי אחר ביומן או חזרו לחיפוש כדי למצוא חלל אחר באותן שעות.</p>
            <Link to="/portal/search" className="mt-2 inline-block text-sm font-medium text-brand hover:underline">
              חיפוש חללים אחרים
            </Link>
          </div>
          <p className="text-xs text-muted">בקשות שטרם אושרו אינן מוצגות כ״תפוס״ לאחרים. המועד מובטח רק לאחר אישור המתנ״ס.</p>
        </aside>
      </div>
    </div>
  )
}

function Row({ k, v }: { k: string; v: string }) {
  return (
    <div className="flex justify-between gap-2">
      <dt className="text-muted">{k}</dt>
      <dd className="font-semibold">{v}</dd>
    </div>
  )
}

