import clsx from 'clsx'
import { useQuery } from '@tanstack/react-query'
import { addDays, startOfDay } from 'date-fns'
import { CalendarDays, Clock, Repeat, Sparkles, XCircle } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { CancelDialog } from '@/components/CancelDialog'
import { Badge, Button, Empty, PageHeader, PageLoader, Tabs } from '@/components/ui'
import { api } from '@/lib/api'
import { useCan } from '@/lib/auth'
import { dateLong, money, timeRange } from '@/lib/format'
import { bookingTypeShort, typeStyles } from '@/lib/labels'
import type { Booking } from '@/lib/types'

/** The signed-in user's confirmed bookings / activities (approved requests and series). */
export function BookingsPage() {
  const can = useCan()
  const coordinator = can('requests:activity')
  const [params] = useSearchParams()
  const focus = params.get('id')
  const [range, setRange] = useState<'upcoming' | 'past'>('upcoming')
  const [cancel, setCancel] = useState<Booking | null>(null)
  const today = startOfDay(new Date())
  const q = range === 'upcoming' ? { from: today.toISOString(), to: addDays(today, 120).toISOString() } : { from: addDays(today, -120).toISOString(), to: today.toISOString() }
  const { data = [], isLoading } = useQuery({
    queryKey: ['bookings', 'mine', range],
    queryFn: () => api.get<Booking[]>('/bookings', { ...q, statuses: ['confirmed', 'pending', 'cancelled'] }),
  })
  const rows = range === 'past' ? [...data].reverse() : data

  useEffect(() => {
    if (focus) document.querySelector(`[data-row="${focus}"]`)?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  }, [focus, rows.length])

  const groups = new Map<string, Booking[]>()
  for (const b of rows) {
    const k = dateLong(b.startsAt)
    groups.set(k, [...(groups.get(k) ?? []), b])
  }

  return (
    <div>
      <PageHeader
        title={coordinator ? 'הפעילויות שלי' : 'ההזמנות שלי'}
        subtitle="הזמנות שאושרו על ידי המתנ״ס, כולל מפגשים קבועים. ביטול מתבצע באמצעות בקשה למנהל."
        actions={
          <>
            <Tabs
              value={range}
              onChange={setRange}
              items={[
                { value: 'upcoming', label: 'קרובות' },
                { value: 'past', label: 'עבר' },
              ]}
            />
            <Link to="/portal/calendar">
              <Button variant="secondary" icon={<CalendarDays className="h-4 w-4" />}>
                ביומן
              </Button>
            </Link>
          </>
        }
      />
      {isLoading ? (
        <PageLoader />
      ) : rows.length === 0 ? (
        <div className="card">
          <Empty icon={<Sparkles className="h-6 w-6" />} title={range === 'upcoming' ? 'אין פעילויות קרובות' : 'אין פעילויות בתקופה זו'} hint="פעילות מופיעה כאן לאחר שהבקשה אושרה." />
        </div>
      ) : (
        <div className="space-y-5">
          {[...groups.entries()].map(([day, list]) => (
            <section key={day}>
              <h3 className="mb-2 text-sm font-semibold text-muted">{day}</h3>
              <div className="card divide-y divide-line overflow-hidden">
                {list.map((b) => {
                  const cancelled = b.status === 'cancelled'
                  return (
                    <article key={b.id} data-row={b.id} className={clsx('flex flex-wrap items-center gap-4 px-5 py-3', focus === b.id && 'bg-brand-soft/50', cancelled && 'opacity-60')}>
                      <span className={clsx('h-10 w-1 rounded-full', typeStyles[b.type].dot)} />
                      <div className="min-w-0 flex-1">
                        <p className={clsx('flex items-center gap-1.5 font-medium', cancelled && 'line-through')}>
                          {b.seriesId && <Repeat className="h-3.5 w-3.5 text-muted" />}
                          {b.title}
                        </p>
                        <p className="flex items-center gap-1 text-xs text-muted">
                          <Clock className="h-3 w-3" /> {timeRange(b.startsAt, b.endsAt)} · {b.roomName} · {bookingTypeShort[b.type]}
                        </p>
                      </div>
                      {b.price > 0 && (
                        <span className="text-end text-xs">
                          <span className="block font-semibold">{money(b.price)}</span>
                          <span className={b.paidAmount >= b.price ? 'text-ok' : 'text-danger'}>{b.paidAmount >= b.price ? 'שולם' : `יתרה ${money(b.price - b.paidAmount)}`}</span>
                        </span>
                      )}
                      {cancelled ? (
                        <Badge tone="gray">בוטל</Badge>
                      ) : b.cancelRequestedAt ? (
                        <Badge tone="red">ביטול ממתין לאישור</Badge>
                      ) : (
                        <>
                          {b.status === 'pending' && <Badge tone="amber">שיבוץ זמני</Badge>}
                          {range === 'upcoming' && (
                            <Button size="sm" variant="ghost" icon={<XCircle className="h-4 w-4" />} onClick={() => setCancel(b)}>
                              בקשת ביטול
                            </Button>
                          )}
                        </>
                      )}
                    </article>
                  )
                })}
              </div>
            </section>
          ))}
        </div>
      )}
      <CancelDialog
        block={cancel && { id: cancel.id, title: cancel.title, startsAt: cancel.startsAt, endsAt: cancel.endsAt, roomName: cancel.roomName, cancelRequested: !!cancel.cancelRequestedAt }}
        onClose={() => setCancel(null)}
      />
    </div>
  )
}
