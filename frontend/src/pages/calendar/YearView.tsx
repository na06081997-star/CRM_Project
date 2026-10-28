import clsx from 'clsx'
import { addDays, isSameMonth, isToday, startOfMonth, startOfWeek } from 'date-fns'
import { useMemo } from 'react'
import { fmt } from '@/lib/format'
import { weekdayShort } from '@/lib/labels'
import type { Booking } from '@/lib/types'
import { byDay, dayKey, WEEK } from './utils'

/** Twelve mini-months with a heat-map of daily booking volume. */
export function YearView({ year, bookings, onDay, onMonth }: { year: number; bookings: Booking[]; onDay: (d: Date) => void; onMonth: (d: Date) => void }) {
  const map = useMemo(() => byDay(bookings), [bookings])
  const max = useMemo(() => Math.max(1, ...[...map.values()].map((l) => l.length)), [map])

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
      {Array.from({ length: 12 }, (_, m) => {
        const first = new Date(year, m, 1)
        const start = startOfWeek(startOfMonth(first), WEEK)
        const total = [...map.entries()].filter(([k]) => k.startsWith(`${year}-${String(m + 1).padStart(2, '0')}`)).reduce((s, [, l]) => s + l.length, 0)
        return (
          <div key={m} className="card p-4">
            <button onClick={() => onMonth(first)} className="mb-3 flex w-full items-baseline justify-between">
              <span className="font-semibold hover:text-brand">{fmt(first, 'MMMM')}</span>
              <span className="text-xs text-muted">{total} הזמנות</span>
            </button>
            <div className="grid grid-cols-7 gap-0.5 text-center text-[10px] text-muted">
              {weekdayShort.map((d) => (
                <span key={d}>{d}</span>
              ))}
            </div>
            <div className="mt-1 grid grid-cols-7 gap-0.5">
              {Array.from({ length: 42 }, (_, i) => {
                const d = addDays(start, i)
                if (!isSameMonth(d, first)) return <span key={i} className="h-7" />
                const n = map.get(dayKey(d))?.length ?? 0
                const level = n === 0 ? 0 : Math.ceil((n / max) * 4)
                return (
                  <button
                    key={i}
                    onClick={() => onDay(d)}
                    title={`${fmt(d, 'd בMMMM')} · ${n} הזמנות`}
                    className={clsx(
                      'flex h-7 items-center justify-center rounded-md text-[11px] tabular-nums transition hover:ring-2 hover:ring-brand/40',
                      level === 0 && 'text-ink/70',
                      level === 1 && 'bg-brand/10',
                      level === 2 && 'bg-brand/25',
                      level === 3 && 'bg-brand/45 text-white',
                      level === 4 && 'bg-brand/70 text-white',
                      isToday(d) && 'font-bold ring-2 ring-brand',
                    )}
                  >
                    {d.getDate()}
                  </button>
                )
              })}
            </div>
          </div>
        )
      })}
    </div>
  )
}
