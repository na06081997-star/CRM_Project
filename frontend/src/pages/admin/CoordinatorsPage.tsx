import { useQuery } from '@tanstack/react-query'
import { addDays, startOfWeek } from 'date-fns'
import { CalendarDays, Mail, Phone } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Avatar, Button, Empty, PageHeader, PageLoader } from '@/components/ui'
import { api } from '@/lib/api'
import { dateShort, timeRange } from '@/lib/format'
import { useActivities, useUsers } from '@/lib/queries'
import type { Booking } from '@/lib/types'

export function CoordinatorsPage() {
  const { data: coordinators, isLoading } = useUsers('coordinator')
  const activities = useActivities().data ?? []
  const from = startOfWeek(new Date(), { weekStartsOn: 0 })
  const { data: week = [] } = useQuery({
    queryKey: ['bookings', 'coord-week', from.toISOString()],
    queryFn: () => api.get<Booking[]>('/bookings', { from: from.toISOString(), to: addDays(from, 7).toISOString(), statuses: ['confirmed', 'pending'] }),
  })
  if (isLoading) return <PageLoader />
  const list = coordinators ?? []
  const now = Date.now()

  return (
    <div>
      <PageHeader title="רכזים" subtitle="עומס שבועי, פעילויות ומפגשים קרובים לכל רכז/ת" />
      {list.length === 0 ? (
        <div className="card">
          <Empty title="אין רכזים במערכת" hint="ניתן להוסיף רכזים במסך משתמשים והרשאות" />
        </div>
      ) : (
        <div className="grid gap-4 lg:grid-cols-2 2xl:grid-cols-3">
          {list.map((c) => {
            const mine = week.filter((b) => b.coordinatorId === c.id)
            const hours = mine.reduce((s, b) => s + (new Date(b.endsAt).getTime() - new Date(b.startsAt).getTime()) / 3600000, 0)
            const acts = activities.filter((a) => a.coordinatorId === c.id)
            const upcoming = mine.filter((b) => new Date(b.endsAt).getTime() > now).slice(0, 4)
            return (
              <article key={c.id} className="card p-5">
                <div className="flex items-center gap-3">
                  <Avatar name={c.fullName} color={c.avatarColor} size={44} />
                  <div className="min-w-0 flex-1">
                    <h3 className="font-semibold">{c.fullName}</h3>
                    <div className="flex flex-wrap gap-x-3 text-xs text-muted">
                      {c.phone && (
                        <a href={`tel:${c.phone}`} className="flex items-center gap-1 hover:text-brand">
                          <Phone className="h-3 w-3" /> {c.phone}
                        </a>
                      )}
                      <a href={`mailto:${c.email}`} className="flex items-center gap-1 hover:text-brand">
                        <Mail className="h-3 w-3" /> {c.email}
                      </a>
                    </div>
                  </div>
                </div>
                <div className="mt-4 grid grid-cols-3 gap-2 text-center">
                  <div className="rounded-xl bg-subtle p-2">
                    <p className="text-lg font-bold">{mine.length}</p>
                    <p className="text-[11px] text-muted">מפגשים השבוע</p>
                  </div>
                  <div className="rounded-xl bg-subtle p-2">
                    <p className="text-lg font-bold">{hours.toFixed(1)}</p>
                    <p className="text-[11px] text-muted">שעות</p>
                  </div>
                  <div className="rounded-xl bg-subtle p-2">
                    <p className="text-lg font-bold">{acts.length}</p>
                    <p className="text-[11px] text-muted">פעילויות</p>
                  </div>
                </div>
                {acts.length > 0 && (
                  <div className="mt-3 flex flex-wrap gap-1">
                    {acts.map((a) => (
                      <span key={a.id} className="chip bg-subtle">
                        <span className="h-1.5 w-1.5 rounded-full" style={{ background: a.color }} />
                        {a.name}
                      </span>
                    ))}
                  </div>
                )}
                <ul className="mt-4 space-y-1.5 text-sm">
                  {upcoming.length === 0 && <li className="text-xs text-muted">אין מפגשים נוספים השבוע</li>}
                  {upcoming.map((b) => (
                    <li key={b.id} className="flex justify-between gap-2 text-xs">
                      <span className="truncate">{b.title}</span>
                      <span className="shrink-0 tabular-nums text-muted">
                        {dateShort(b.startsAt)} {timeRange(b.startsAt, b.endsAt)}
                      </span>
                    </li>
                  ))}
                </ul>
                <Link to={`/admin/calendar?view=week&coordinator=${c.id}`}>
                  <Button variant="secondary" size="sm" className="mt-4 w-full" icon={<CalendarDays className="h-3.5 w-3.5" />}>
                    היומן של {c.fullName.split(' ')[0]}
                  </Button>
                </Link>
              </article>
            )
          })}
        </div>
      )}
    </div>
  )
}
