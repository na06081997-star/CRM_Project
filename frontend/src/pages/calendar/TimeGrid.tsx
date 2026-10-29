import clsx from 'clsx'
import { isToday } from 'date-fns'
import { Repeat } from 'lucide-react'
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { time, timeRange } from '@/lib/format'
import { typeStyles } from '@/lib/labels'
import type { Booking } from '@/lib/types'
import { layoutDay, minutesToDate, snap, SNAP } from './utils'

export interface Column {
  key: string
  header: ReactNode
  day: Date // local midnight of the column's day
  roomId?: string
  highlight?: boolean
  match: (b: Booking) => boolean
}

export type Lens = 'admin' | 'coordinator' | 'public'

interface Props {
  columns: Column[]
  bookings: Booking[]
  startHour: number
  endHour: number
  lens: Lens
  myId?: string
  canEdit: boolean // drag on empty cells to create
  canMove?: boolean // drag / resize existing items (defaults to canEdit)
  focusId?: string | null
  onCreate: (r: { start: Date; end: Date; roomId?: string }) => void
  onOpen: (b: Booking) => void
  onMove: (b: Booking, r: { start: Date; end: Date; roomId?: string }) => void
}

const HOUR_PX = 52
const PX_PER_MIN = HOUR_PX / 60

type Drag =
  | { mode: 'create'; col: number; from: number; to: number }
  | { mode: 'move'; booking: Booking; col: number; origCol: number; offset: number; start: number; duration: number; moved: boolean; x0: number; y0: number }
  | { mode: 'resize'; booking: Booking; col: number; start: number; end: number }

export function TimeGrid({ columns, bookings, startHour, endHour, lens, myId, canEdit, canMove = canEdit, focusId, onCreate, onOpen, onMove }: Props) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const bodyRef = useRef<HTMLDivElement>(null)
  const [drag, setDrag] = useState<Drag | null>(null)
  const dragRef = useRef<Drag | null>(null)
  dragRef.current = drag
  const gridStart = startHour * 60
  const gridEnd = endHour * 60
  const hours = useMemo(() => Array.from({ length: endHour - startHour }, (_, i) => startHour + i), [startHour, endHour])

  const placed = useMemo(
    () => columns.map((c) => layoutDay(bookings.filter(c.match), c.day, gridStart, gridEnd)),
    [columns, bookings, gridStart, gridEnd],
  )

  // Scroll to "now" (or 08:00) once.
  useLayoutEffect(() => {
    const el = scrollRef.current
    if (!el) return
    const now = new Date()
    const target = columns.some((c) => isToday(c.day)) ? now.getHours() * 60 + now.getMinutes() - 90 : 8 * 60
    el.scrollTop = Math.max(0, (target - gridStart) * PX_PER_MIN)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [columns.length, gridStart])

  // Focus a booking opened from search.
  useEffect(() => {
    if (!focusId) return
    const el = document.querySelector(`[data-booking="${focusId}"]`)
    el?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  }, [focusId, placed])

  /** Converts a pointer position to (column index, minutes since midnight). */
  const locate = (x: number, y: number) => {
    const body = bodyRef.current!
    const rect = body.getBoundingClientRect()
    const colW = rect.width / columns.length
    // RTL: first column is on the right.
    const col = Math.min(columns.length - 1, Math.max(0, Math.floor((rect.right - x) / colW)))
    const min = gridStart + (y - rect.top) / PX_PER_MIN
    return { col, min: Math.min(gridEnd, Math.max(gridStart, min)) }
  }

  // Layout effect: listeners must be attached before a fast click releases.
  useLayoutEffect(() => {
    if (!drag) return
    const move = (e: PointerEvent) => {
      const d = dragRef.current
      if (!d) return
      const { col, min } = locate(e.clientX, e.clientY)
      if (d.mode === 'create') setDrag({ ...d, to: snap(min) })
      else if (d.mode === 'resize') setDrag({ ...d, end: Math.max(d.start + SNAP, snap(min)) })
      else {
        const moved = d.moved || Math.abs(e.clientX - d.x0) + Math.abs(e.clientY - d.y0) > 5
        const start = Math.min(gridEnd - d.duration, Math.max(gridStart, snap(min - d.offset)))
        setDrag({ ...d, col, start, moved })
      }
    }
    const up = () => {
      const d = dragRef.current
      setDrag(null)
      if (!d) return
      if (d.mode === 'create') {
        const a = Math.min(d.from, d.to)
        let b = Math.max(d.from, d.to)
        if (b - a < SNAP) b = a + 60 // simple click → one hour
        const c = columns[d.col]
        onCreate({ start: minutesToDate(c.day, a), end: minutesToDate(c.day, b), roomId: c.roomId })
      } else if (d.mode === 'resize') {
        const c = columns[d.col]
        const start = new Date(d.booking.startsAt)
        const end = minutesToDate(c.day, d.end)
        if (end.getTime() !== new Date(d.booking.endsAt).getTime()) onMove(d.booking, { start, end, roomId: d.booking.roomId })
      } else if (!d.moved) {
        onOpen(d.booking)
      } else {
        const c = columns[d.col]
        const start = minutesToDate(c.day, d.start)
        const end = new Date(start.getTime() + d.duration * 60000)
        const roomId = c.roomId ?? d.booking.roomId
        if (start.getTime() !== new Date(d.booking.startsAt).getTime() || roomId !== d.booking.roomId) onMove(d.booking, { start, end, roomId })
      }
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up, { once: true })
    return () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [drag?.mode])

  const startCreate = (e: React.PointerEvent, col: number) => {
    if (!canEdit || e.button !== 0) return
    const { min } = locate(e.clientX, e.clientY)
    const m = Math.floor(min / SNAP) * SNAP
    setDrag({ mode: 'create', col, from: m, to: m + SNAP })
  }

  const startMove = (e: React.PointerEvent, b: Booking, col: number) => {
    e.stopPropagation()
    if (e.button !== 0) return
    const { min } = locate(e.clientX, e.clientY)
    const day = columns[col].day
    const s = (new Date(b.startsAt).getTime() - day.getTime()) / 60000
    const dur = (new Date(b.endsAt).getTime() - new Date(b.startsAt).getTime()) / 60000
    const editable = canMove && b.status !== 'cancelled'
    setDrag({ mode: 'move', booking: b, col, origCol: col, offset: min - s, start: s, duration: dur, moved: false, x0: e.clientX, y0: editable ? e.clientY : Number.NaN })
  }

  const startResize = (e: React.PointerEvent, b: Booking, col: number) => {
    e.stopPropagation()
    const day = columns[col].day
    setDrag({
      mode: 'resize',
      booking: b,
      col,
      start: (new Date(b.startsAt).getTime() - day.getTime()) / 60000,
      end: (new Date(b.endsAt).getTime() - day.getTime()) / 60000,
    })
  }

  const now = new Date()
  const nowMin = now.getHours() * 60 + now.getMinutes()
  const isMoving = drag?.mode === 'move' && drag.moved && !Number.isNaN(drag.y0)

  return (
    <div className="card flex min-h-0 flex-1 flex-col overflow-hidden">
      {/* Column headers */}
      <div className="flex border-b border-line bg-surface">
        <div className="w-14 shrink-0" />
        <div className="grid flex-1" style={{ gridTemplateColumns: `repeat(${columns.length}, minmax(0, 1fr))` }}>
          {columns.map((c) => (
            <div key={c.key} className={clsx('border-s border-line px-2 py-2 text-center', c.highlight && 'bg-brand-soft/60')}>
              {c.header}
            </div>
          ))}
        </div>
      </div>

      <div ref={scrollRef} className="scroll-thin relative flex-1 overflow-y-auto" style={{ maxHeight: 'calc(100vh - 300px)', minHeight: 420 }}>
        <div className="flex" style={{ height: (gridEnd - gridStart) * PX_PER_MIN }}>
          {/* Hour gutter */}
          <div className="relative w-14 shrink-0 select-none">
            {hours.map((h, i) => (
              <span key={h} className="absolute end-2 -translate-y-1/2 text-[11px] tabular-nums text-muted" style={{ top: i * HOUR_PX }}>
                {i === 0 ? '' : `${String(h).padStart(2, '0')}:00`}
              </span>
            ))}
          </div>

          <div ref={bodyRef} className={clsx('relative grid flex-1', isMoving && 'cursor-grabbing')} style={{ gridTemplateColumns: `repeat(${columns.length}, minmax(0, 1fr))` }}>
            {/* Hour lines */}
            <div className="pointer-events-none absolute inset-0">
              {hours.map((h, i) => (
                <div key={h} className="absolute inset-x-0 border-t border-line/70" style={{ top: i * HOUR_PX }}>
                  <div className="absolute inset-x-0 top-1/2 border-t border-dashed border-line/40" style={{ top: HOUR_PX / 2 }} />
                </div>
              ))}
            </div>

            {columns.map((c, ci) => (
              <div
                key={c.key}
                className={clsx('relative border-s border-line', c.day.getDay() === 6 && 'hatch opacity-[.97]', canEdit && 'cursor-cell')}
                onPointerDown={(e) => startCreate(e, ci)}
              >
                {placed[ci].map((p) => {
                  const b = p.booking
                  const hidden = isMoving && drag.mode === 'move' && drag.booking.id === b.id
                  return (
                    <EventBlock
                      key={b.id}
                      p={p}
                      lens={lens}
                      mine={!!myId && b.coordinatorId === myId}
                      dimmed={hidden}
                      focused={focusId === b.id}
                      canResize={canMove && b.status !== 'cancelled'}
                      onPointerDown={(e) => startMove(e, b, ci)}
                      onResize={(e) => startResize(e, b, ci)}
                      resizingEnd={drag?.mode === 'resize' && drag.booking.id === b.id ? drag.end : undefined}
                    />
                  )
                })}

                {/* Ghosts */}
                {drag?.mode === 'create' && drag.col === ci && (
                  <Ghost top={Math.min(drag.from, drag.to) - gridStart} height={Math.max(SNAP, Math.abs(drag.to - drag.from))} label={`${fmtMin(Math.min(drag.from, drag.to))}–${fmtMin(Math.max(drag.from, drag.to) || drag.from + SNAP)}`} />
                )}
                {isMoving && drag.mode === 'move' && drag.col === ci && (
                  <Ghost top={drag.start - gridStart} height={drag.duration} label={`${drag.booking.title} · ${fmtMin(drag.start)}–${fmtMin(drag.start + drag.duration)}`} solid />
                )}

                {/* Now line */}
                {isToday(c.day) && nowMin >= gridStart && nowMin <= gridEnd && (
                  <div className="pointer-events-none absolute inset-x-0 z-20" style={{ top: (nowMin - gridStart) * PX_PER_MIN }}>
                    <div className="relative border-t-2 border-danger">
                      <span className="absolute -top-[5px] start-[-5px] h-2 w-2 rounded-full bg-danger" />
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

function fmtMin(m: number) {
  return `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(Math.round(m % 60)).padStart(2, '0')}`
}

function Ghost({ top, height, label, solid }: { top: number; height: number; label: string; solid?: boolean }) {
  return (
    <div
      className={clsx(
        'pointer-events-none absolute inset-x-1 z-30 rounded-lg border-2 px-2 py-1 text-[11px] font-medium',
        solid ? 'border-brand bg-brand/15 text-brand-strong shadow-pop' : 'border-dashed border-brand bg-brand/10 text-brand-strong',
      )}
      style={{ top: top * PX_PER_MIN, height: Math.max(height * PX_PER_MIN, 18) }}
    >
      {label}
    </div>
  )
}

function EventBlock({
  p,
  lens,
  mine,
  dimmed,
  focused,
  canResize,
  onPointerDown,
  onResize,
  resizingEnd,
}: {
  p: ReturnType<typeof layoutDay>[number]
  lens: Lens
  mine: boolean
  dimmed: boolean
  focused: boolean
  canResize: boolean
  onPointerDown: (e: React.PointerEvent) => void
  onResize: (e: React.PointerEvent) => void
  resizingEnd?: number
}) {
  const b = p.booking
  const st = typeStyles[b.type]
  const width = 100 / p.cols
  const blocked = b.type === 'maintenance' || b.type === 'closed'
  const title = lens === 'public' && blocked ? 'סגור' : b.title
  const host = lens === 'public' ? null : b.coordinatorName || b.customerName
  let height = p.height
  if (resizingEnd !== undefined) {
    const startMin = new Date(b.startsAt).getHours() * 60 + new Date(b.startsAt).getMinutes()
    height = resizingEnd - startMin
  }
  const px = Math.max(height * PX_PER_MIN - 2, 18)
  const compact = px < 40

  return (
    <div
      data-booking={b.id}
      onPointerDown={onPointerDown}
      className={clsx(
        'group absolute z-10 cursor-pointer overflow-hidden rounded-lg border-s-[3px] px-1.5 py-1 text-[11px] leading-tight shadow-sm transition-[box-shadow,opacity] hover:z-20 hover:shadow-pop',
        st.bg,
        st.border,
        b.status === 'pending' && 'border border-dashed',
        b.status === 'cancelled' && 'opacity-50 line-through',
        lens === 'coordinator' && !mine && 'opacity-40',
        dimmed && 'opacity-30',
        focused && 'ring-2 ring-brand ring-offset-1',
      )}
      style={{
        top: p.top * PX_PER_MIN + 1,
        height: px,
        insetInlineStart: `calc(${p.col * width}% + 2px)`,
        width: `calc(${width}% - 4px)`,
      }}
      title={`${b.title}\n${timeRange(b.startsAt, b.endsAt)} · ${b.roomName}`}
    >
      <div className={clsx('flex items-center gap-1 font-semibold', st.text)}>
        {b.seriesId && <Repeat className="h-3 w-3 shrink-0 opacity-70" />}
        <span className="truncate text-ink">{title}</span>
      </div>
      {!compact && (
        <>
          <p className="mt-0.5 tabular-nums text-muted">{compact ? time(b.startsAt) : timeRange(b.startsAt, b.endsAt)}</p>
          <p className="truncate text-muted">
            {b.roomName}
            {host ? ` · ${host}` : ''}
          </p>
        </>
      )}
      {compact && <span className="text-muted"> {time(b.startsAt)}</span>}
      {canResize && (
        <div
          onPointerDown={onResize}
          className="absolute inset-x-0 bottom-0 h-2 cursor-ns-resize opacity-0 group-hover:opacity-100"
        >
          <div className="mx-auto mt-0.5 h-1 w-6 rounded-full bg-ink/30" />
        </div>
      )}
    </div>
  )
}
