import clsx from 'clsx'
import { useQuery } from '@tanstack/react-query'
import { addDays, startOfDay } from 'date-fns'
import { AlertTriangle, ArrowLeft, CalendarCheck, CalendarDays, CircleDollarSign, ClipboardList, Radio, Search } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Badge, Button, Card, Empty, PageHeader, Stat } from '@/components/ui'
import { api } from '@/lib/api'
import { useCan, useUser } from '@/lib/auth'
import { dateShort, fmt, money, relative, timeRange } from '@/lib/format'
import { requestStatusLabel, typeStyles } from '@/lib/labels'
import type { Booking } from '@/lib/types'
import { useMyBalances, useMyRequests } from './PortalLayout'
import { requestTone } from './RequestsPage'

export function HomePage() {
  const user = useUser()
  const can = useCan()
  const coordinator = can('requests:activity')
  const today = startOfDay(new Date())
  const { data: upcoming = [] } = useQuery({
    queryKey: ['bookings', 'mine', 'upcoming'],
    queryFn: () => api.get<Booking[]>('/bookings', { from: today.toISOString(), to: addDays(today, 14).toISOString(), statuses: ['confirmed', 'pending'] }),
  })
  const requests = useMyRequests().data ?? []
  const balances = useMyBalances().data ?? []

  const now = Date.now()
  const week = upcoming.filter((b) => new Date(b.startsAt) < addDays(today, 7))
  const pending = requests.filter((r) => r.status === 'pending')
  const due = balances.reduce((s, b) => s + b.price - b.paidAmount, 0)
  const hour = new Date().getHours()
  const greet = hour < 12 ? 'בוקר טוב' : hour < 17 ? 'צהריים טובים' : 'ערב טוב'

  return (
    <div className="space-y-6">
      <PageHeader
        title={`${greet}, ${user?.fullName.split(' ')[0] ?? ''}`}
        subtitle={fmt(new Date(), 'EEEE, d בMMMM yyyy')}
        actions={
          <>
            <Link to="/portal/search">
              <Button variant="secondary" icon={<Search className="h-4 w-4" />}>
                חיפוש חלל פנוי
              </Button>
            </Link>
            {can('availability:view') && (
              <Link to="/portal/calendar">
                <Button icon={<CalendarDays className="h-4 w-4" />}>יומן זמינות</Button>
              </Link>
            )}
          </>
        }
      />

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Stat label={coordinator ? 'הפעילויות שלי השבוע' : 'ההזמנות שלי השבוע'} value={week.length} icon={<CalendarCheck className="h-5 w-5" />} hint={upcoming.length > week.length ? `${upcoming.length - week.length} נוספות בשבוע שאחריו` : undefined} />
        <Link to="/portal/requests">
          <Stat label="בקשות ממתינות לאישור" value={pending.length} tone="amber" icon={<ClipboardList className="h-5 w-5" />} hint="לחצו למעקב" />
        </Link>
        <Stat label="בקשות שאושרו" value={requests.filter((r) => r.status === 'approved').length} tone="green" icon={<CalendarCheck className="h-5 w-5" />} hint={`מתוך ${requests.length} בקשות`} />
        <Link to="/portal/payments">
          <Stat label="יתרה לתשלום" value={money(due)} tone={due > 0 ? 'red' : 'teal'} icon={due > 0 ? <AlertTriangle className="h-5 w-5" /> : <CircleDollarSign className="h-5 w-5" />} hint={due > 0 ? `${balances.length} הזמנות פתוחות` : 'אין חובות פתוחים'} />
        </Link>
      </div>

      <div className="grid gap-6 xl:grid-cols-3">
        <Card
          className="xl:col-span-2"
          padded={false}
          title={coordinator ? 'הפעילויות הקרובות שלי' : 'ההזמנות הקרובות שלי'}
          actions={
            <Link to="/portal/bookings" className="flex items-center gap-1 text-xs font-medium text-brand hover:underline">
              לכל הרשימה <ArrowLeft className="h-3.5 w-3.5" />
            </Link>
          }
        >
          {upcoming.length === 0 ? (
            <Empty
              title="אין פעילויות קרובות"
              hint="מצאו מועד פנוי ושלחו בקשה – לאחר אישור המתנ״ס היא תופיע כאן."
              action={
                <Link to="/portal/new">
                  <Button size="sm">בקשה חדשה</Button>
                </Link>
              }
            />
          ) : (
            <ul className="scroll-thin max-h-[420px] divide-y divide-line overflow-y-auto">
              {upcoming.slice(0, 12).map((b) => {
                const live = now >= new Date(b.startsAt).getTime() && now < new Date(b.endsAt).getTime()
                return (
                  <li key={b.id}>
                    <Link to={`/portal/bookings?id=${b.id}`} className="flex items-center gap-4 px-5 py-3 transition hover:bg-subtle">
                      <span className="w-16 shrink-0 text-xs text-muted">{dateShort(b.startsAt)}</span>
                      <span className="w-24 shrink-0 text-sm font-semibold tabular-nums">{timeRange(b.startsAt, b.endsAt)}</span>
                      <span className={clsx('h-9 w-1 rounded-full', typeStyles[b.type].dot)} />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium">{b.title}</span>
                        <span className="block truncate text-xs text-muted">{b.roomName}</span>
                      </span>
                      {live ? (
                        <Badge tone="green">
                          <Radio className="h-3 w-3 animate-pulse2" /> מתקיים כעת
                        </Badge>
                      ) : b.cancelRequestedAt ? (
                        <Badge tone="red">ביטול ממתין</Badge>
                      ) : b.status === 'pending' ? (
                        <Badge tone="amber">שיבוץ זמני</Badge>
                      ) : null}
                    </Link>
                  </li>
                )
              })}
            </ul>
          )}
        </Card>

        <Card
          padded={false}
          title="הבקשות האחרונות שלי"
          actions={
            <Link to="/portal/requests" className="text-xs font-medium text-brand hover:underline">
              לכל הבקשות
            </Link>
          }
        >
          {requests.length === 0 ? (
            <Empty title="עדיין לא נשלחו בקשות" />
          ) : (
            <ul className="divide-y divide-line">
              {requests.slice(0, 6).map((r) => (
                <li key={r.id}>
                  <Link to={`/portal/requests?id=${r.id}`} className="block px-5 py-3 hover:bg-subtle">
                    <div className="flex items-center justify-between gap-2">
                      <span className="truncate text-sm font-medium">
                        <span className="text-muted">#{r.number}</span> {r.title}
                      </span>
                      <Badge tone={requestTone(r)}>{r.bookingStatus === 'cancelled' ? 'בוטלה' : requestStatusLabel[r.status]}</Badge>
                    </div>
                    <p className="mt-0.5 text-xs text-muted">
                      {r.roomName} · {dateShort(r.startsAt)} {timeRange(r.startsAt, r.endsAt)}
                    </p>
                    <p className="mt-0.5 text-[11px] text-muted/80">נשלחה {relative(r.createdAt)}</p>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>
    </div>
  )
}
