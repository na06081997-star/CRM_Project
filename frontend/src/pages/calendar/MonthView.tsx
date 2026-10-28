import clsx from 'clsx'
import { addDays, isSameMonth, isToday } from 'date-fns'
import { useMemo } from 'react'
import { time } from '@/lib/format'
import { typeStyles, weekdayLong } from '@/lib/labels'
import type { Booking } from '@/lib/types'
import { byDay, dayKey } from './utils'

const MAX_CHIPS = 3

export function MonthView({ from, anchor, bookings, onDay, onOpen, onCreateDay }: { from: Date; anchor: Date; bookings: Booking[]; onDay: (d: Date) => void; onOpen: (b: Booking) => void; onCreateDay: (d: Date) => void }) {
  const days = useMemo(() => Array.from({ length: 42 }, (_, i) => addDays(from, i)), [from])
  const weeks = days.slice(35).every((d) => !isSameMonth(d, anchor)) ? 5 : 6
  const map = useMemo(() => byDay(bookings), [bookings])

  return (
    <div className="card overflow-hidden">
      <div className="grid grid-cols-7 border-b border-line bg-subtle/60">
        {weekdayLong.map((d) => (
          <div key={d} className="px-2 py-2 text-center text-xs font-semibold text-muted">
            {d}
          </div>
        ))}
      </div>
      <div className="grid grid-cols-7" style={{ gridTemplateRows: `repeat(${weeks}, minmax(112px, 1fr))` }}>
        {days.slice(0, weeks * 7).map((d) => {
          const list = map.get(dayKey(d)) ?? []
          const extra = list.length - MAX_CHIPS
          return (
            <div
              key={d.toISOString()}
              onDoubleClick={() => onCreateDay(d)}
              className={clsx('group min-w-0 border-b border-s border-line p-1.5', !isSameMonth(d, anchor) && 'bg-subtle/50 text-muted', d.getDay() === 6 && 'hatch')}
            >
              <button
                onClick={() => onDay(d)}
                className={clsx(
                  'mb-1 flex h-7 w-7 items-center justify-center rounded-full text-xs font-semibold transition hover:bg-subtle',
                  isToday(d) && 'bg-brand text-white hover:bg-brand dark:text-slate-900',
                )}
              >
                {d.getDate()}
              </button>
              <div className="flex flex-col gap-0.5">
                {list.slice(0, MAX_CHIPS).map((b) => {
                  const st = typeStyles[b.type]
                  return (
                    <button
                      key={b.id}
                      onClick={() => onOpen(b)}
                      className={clsx(
                        'flex w-full items-center gap-1 truncate rounded-md px-1.5 py-0.5 text-start text-[11px] hover:brightness-95',
                        st.bg,
                        b.status === 'pending' && 'border border-dashed border-current',
                        b.status === 'cancelled' && 'line-through opacity-50',
                      )}
                    >
                      <span className={clsx('h-1.5 w-1.5 shrink-0 rounded-full', st.dot)} />
                      <span className="tabular-nums text-muted">{time(b.startsAt)}</span>
                      <span className="truncate font-medium">{b.title}</span>
                    </button>
                  )
                })}
                {extra > 0 && (
                  <button onClick={() => onDay(d)} className="rounded-md px-1.5 text-start text-[11px] font-medium text-muted hover:bg-subtle">
                    +{extra} נוספים
                  </button>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
