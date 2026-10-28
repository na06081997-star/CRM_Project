import { useQuery } from '@tanstack/react-query'
import { CalendarDays, CornerDownLeft, DoorOpen, Search, User } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import { api } from '@/lib/api'
import { useCan } from '@/lib/auth'
import { dateShort, timeRange } from '@/lib/format'
import { useRooms } from '@/lib/queries'
import type { Booking, Customer } from '@/lib/types'

interface Hit {
  key: string
  icon: JSX.Element
  title: string
  sub: string
  go: () => void
}

/** Ctrl+K global search across bookings, customers and rooms. */
export function CommandPalette({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [q, setQ] = useState('')
  const [debounced, setDebounced] = useState('')
  const [active, setActive] = useState(0)
  const nav = useNavigate()
  const can = useCan()
  const rooms = useRooms().data ?? []

  useEffect(() => {
    const t = setTimeout(() => setDebounced(q.trim()), 200)
    return () => clearTimeout(t)
  }, [q])
  useEffect(() => {
    if (!open) {
      setQ('')
      setActive(0)
    }
  }, [open])

  const bookings = useQuery({
    queryKey: ['bookings', 'search', debounced],
    queryFn: () => api.get<Booking[]>('/bookings', { q: debounced, limit: 8 }),
    enabled: open && debounced.length > 1,
  })
  const customers = useQuery({
    queryKey: ['customers', debounced],
    queryFn: () => api.get<Customer[]>('/customers', { q: debounced }),
    enabled: open && debounced.length > 1 && can('customers:manage'),
  })

  const hits: Hit[] = useMemo(() => {
    const out: Hit[] = []
    const go = (to: string) => () => {
      nav(to)
      onClose()
    }
    for (const b of bookings.data ?? [])
      out.push({
        key: 'b' + b.id,
        icon: <CalendarDays className="h-4 w-4" />,
        title: b.title,
        sub: `${b.roomName} · ${dateShort(b.startsAt)} ${timeRange(b.startsAt, b.endsAt)}`,
        go: go(`/admin/calendar?date=${b.startsAt.slice(0, 10)}&booking=${b.id}`),
      })
    for (const c of (customers.data ?? []).slice(0, 5))
      out.push({ key: 'c' + c.id, icon: <User className="h-4 w-4" />, title: c.name, sub: c.phone || c.email || '', go: go(`/admin/customers?id=${c.id}`) })
    const lq = debounced.toLowerCase()
    if (lq)
      for (const r of rooms.filter((r) => r.name.toLowerCase().includes(lq)).slice(0, 4))
        out.push({ key: 'r' + r.id, icon: <DoorOpen className="h-4 w-4" />, title: r.name, sub: `קיבולת ${r.capacity}`, go: go(`/admin/calendar?view=rooms&rooms=${r.id}`) })
    return out
  }, [bookings.data, customers.data, rooms, debounced, nav, onClose])

  if (!open) return null
  return createPortal(
    <div className="fixed inset-0 z-50 flex items-start justify-center p-4 pt-[12vh]">
      <div className="absolute inset-0 animate-fade-in bg-slate-900/40" onClick={onClose} />
      <div className="relative w-full max-w-xl animate-slide-up overflow-hidden rounded-2xl border border-line bg-surface shadow-pop">
        <div className="flex items-center gap-3 border-b border-line px-4">
          <Search className="h-5 w-5 text-muted" />
          <input
            autoFocus
            value={q}
            onChange={(e) => {
              setQ(e.target.value)
              setActive(0)
            }}
            onKeyDown={(e) => {
              if (e.key === 'Escape') onClose()
              if (e.key === 'ArrowDown') setActive((a) => Math.min(a + 1, hits.length - 1))
              if (e.key === 'ArrowUp') setActive((a) => Math.max(a - 1, 0))
              if (e.key === 'Enter') hits[active]?.go()
            }}
            placeholder="חיפוש הזמנה, לקוח, חדר או טלפון…"
            className="h-14 flex-1 bg-transparent text-base outline-none placeholder:text-muted"
          />
        </div>
        <div className="scroll-thin max-h-[50vh] overflow-y-auto p-2">
          {debounced.length < 2 && <p className="p-6 text-center text-sm text-muted">הקלידו לפחות 2 תווים</p>}
          {debounced.length >= 2 && hits.length === 0 && !bookings.isFetching && <p className="p-6 text-center text-sm text-muted">לא נמצאו תוצאות</p>}
          {hits.map((h, i) => (
            <button
              key={h.key}
              onMouseEnter={() => setActive(i)}
              onClick={h.go}
              className={`flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-start ${i === active ? 'bg-brand-soft' : ''}`}
            >
              <span className="rounded-lg bg-subtle p-2 text-muted">{h.icon}</span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium">{h.title}</span>
                <span className="block truncate text-xs text-muted">{h.sub}</span>
              </span>
              {i === active && <CornerDownLeft className="h-4 w-4 text-muted" />}
            </button>
          ))}
        </div>
      </div>
    </div>,
    document.body,
  )
}
