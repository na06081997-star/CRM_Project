import clsx from 'clsx'
import { addDays, addMonths, isSameDay, isSameMonth, isToday, startOfMonth, startOfWeek } from 'date-fns'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useEffect, useState } from 'react'
import { fmt } from '@/lib/format'
import { weekdayShort } from '@/lib/labels'
import { WEEK } from './utils'

/** Small month navigator, as in Google Calendar's side panel. */
export function MiniMonth({ value, onPick, busyDays }: { value: Date; onPick: (d: Date) => void; busyDays?: Set<string> }) {
  const [cursor, setCursor] = useState(startOfMonth(value))
  useEffect(() => setCursor(startOfMonth(value)), [value])
  const start = startOfWeek(cursor, WEEK)

  return (
    <div className="select-none">
      <div className="mb-2 flex items-center justify-between">
        <span className="text-sm font-semibold">{fmt(cursor, 'MMMM yyyy')}</span>
        <div className="flex">
          <button className="rounded-lg p-1 hover:bg-subtle" onClick={() => setCursor(addMonths(cursor, -1))} aria-label="חודש קודם">
            <ChevronRight className="h-4 w-4" />
          </button>
          <button className="rounded-lg p-1 hover:bg-subtle" onClick={() => setCursor(addMonths(cursor, 1))} aria-label="חודש הבא">
            <ChevronLeft className="h-4 w-4" />
          </button>
        </div>
      </div>
      <div className="grid grid-cols-7 text-center text-[10px] text-muted">
        {weekdayShort.map((d) => (
          <span key={d} className="py-1">
            {d}
          </span>
        ))}
      </div>
      <div className="grid grid-cols-7 text-center">
        {Array.from({ length: 42 }, (_, i) => {
          const d = addDays(start, i)
          const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
          return (
            <button
              key={i}
              onClick={() => onPick(d)}
              className={clsx(
                'relative mx-auto my-0.5 flex h-7 w-7 items-center justify-center rounded-full text-xs tabular-nums transition hover:bg-subtle',
                !isSameMonth(d, cursor) && 'text-muted/50',
                isSameDay(d, value) && 'bg-brand-soft font-semibold text-brand-strong',
                isToday(d) && 'bg-brand text-white hover:bg-brand dark:text-slate-900',
              )}
            >
              {d.getDate()}
              {busyDays?.has(key) && !isToday(d) && <span className="absolute bottom-0.5 h-1 w-1 rounded-full bg-brand/70" />}
            </button>
          )
        })}
      </div>
    </div>
  )
}
