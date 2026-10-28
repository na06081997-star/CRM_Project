import clsx from 'clsx'
import { useQuery } from '@tanstack/react-query'
import { AlertTriangle, ArrowLeft, CalendarCheck, CircleDollarSign, ClipboardList, Gauge, ListChecks, Radio } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Badge, Card, Empty, PageHeader, PageLoader, Stat } from '@/components/ui'
import { api } from '@/lib/api'
import { useCan, useUser } from '@/lib/auth'
import { dateShort, fmt, money, relative, timeRange } from '@/lib/format'
import { bookingTypeLabel, typeStyles, weekdayShort } from '@/lib/labels'
import { usePrefs } from '@/lib/prefs'
import type { BookingType, Dashboard } from '@/lib/types'

export function DashboardPage() {
  const user = useUser()
  const can = useCan()
  const buildingId = usePrefs((s) => s.buildingId)
  const { data, isLoading } = useQuery({
    queryKey: ['dashboard', buildingId],
    queryFn: () => api.get<Dashboard>('/dashboard', { buildingId: buildingId ?? undefined }),
    refetchInterval: 60_000,
  })
  if (isLoading || !data) return <PageLoader />

  const now = Date.now()
  const hour = new Date().getHours()
  const greet = hour < 12 ? 'בוקר טוב' : hour < 17 ? 'צהריים טובים' : 'ערב טוב'
  const maxDay = Math.max(1, ...data.week.map((d) => d.count))
  const typeTotal = Object.values(data.byType).reduce((s, n) => s + (n || 0), 0) || 1

  return (
    <div className="space-y-6">
      <PageHeader title={`${greet}, ${user?.fullName.split(' ')[0] ?? ''}`} subtitle={fmt(new Date(), 'EEEE, d בMMMM yyyy')} />

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-6">
        <Stat label="הזמנות היום" value={data.todayBookings} icon={<CalendarCheck className="h-5 w-5" />} hint={data.ongoingNow ? `${data.ongoingNow} מתקיימות כעת` : 'אין פעילות כרגע'} />
        <Link to="/admin/bookings">
          <Stat label="בקשות ממתינות" value={data.pendingRequests} tone="amber" icon={<ClipboardList className="h-5 w-5" />} hint="לחצו לטיפול" />
        </Link>
        <Stat label="תפוסת חדרים היום" value={`${data.occupancyToday}%`} tone="blue" icon={<Gauge className="h-5 w-5" />} hint={`השבוע: ${data.occupancyWeek}%`} />
        {can('payments:view') && (
          <>
            <Stat label="נגבה החודש" value={money(data.payments.paidThisMonth)} tone="green" icon={<CircleDollarSign className="h-5 w-5" />} hint={`סה״כ חויב ${money(data.payments.totalBilled)}`} />
            <Link to="/admin/payments">
              <Stat label="יתרות פתוחות" value={money(data.payments.outstanding)} tone="red" icon={<AlertTriangle className="h-5 w-5" />} hint={`${data.payments.unpaidCount} הזמנות לא שולמו במלואן`} />
            </Link>
          </>
        )}
        <Link to="/admin/tasks">
          <Stat label="משימות תפעול פתוחות" value={data.openTasks} tone="violet" icon={<ListChecks className="h-5 w-5" />} hint="להיום ולפני" />
        </Link>
      </div>

      <div className="grid gap-6 xl:grid-cols-3">
        {/* Today timeline */}
        <Card
          className="xl:col-span-2"
          title={
            <span className="flex items-center gap-2">
              לו״ז היום <Badge tone="teal">{data.today.length}</Badge>
            </span>
          }
          actions={
            <Link to="/admin/calendar?view=rooms" className="flex items-center gap-1 text-xs font-medium text-brand hover:underline">
              יומן לפי חדר <ArrowLeft className="h-3.5 w-3.5" />
            </Link>
          }
          padded={false}
        >
          {data.today.length === 0 ? (
            <Empty title="אין הזמנות היום" />
          ) : (
            <ul className="scroll-thin max-h-[420px] divide-y divide-line overflow-y-auto">
              {data.today.map((b) => {
                const s = new Date(b.startsAt).getTime()
                const e = new Date(b.endsAt).getTime()
                const live = now >= s && now < e
                const past = now >= e
                const st = typeStyles[b.type]
                return (
                  <li key={b.id}>
                    <Link to={`/admin/calendar?view=day&date=${b.startsAt.slice(0, 10)}&booking=${b.id}`} className={clsx('flex items-center gap-4 px-5 py-3 transition hover:bg-subtle', past && 'opacity-55')}>
                      <span className="w-24 shrink-0 text-sm font-semibold tabular-nums">{timeRange(b.startsAt, b.endsAt)}</span>
                      <span className={clsx('h-9 w-1 rounded-full', st.dot)} />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium">{b.title}</span>
                        <span className="block truncate text-xs text-muted">
                          {b.roomName}
                          {b.coordinatorName || b.customerName ? ` · ${b.coordinatorName || b.customerName}` : ''}
                        </span>
                      </span>
                      {live ? (
                        <Badge tone="green">
                          <Radio className="h-3 w-3 animate-pulse2" /> מתקיים כעת
                        </Badge>
                      ) : b.status === 'pending' ? (
                        <Badge tone="amber">ממתין</Badge>
                      ) : (
                        <Badge tone="gray">{bookingTypeLabel[b.type].split(' ')[0]}</Badge>
                      )}
                    </Link>
                  </li>
                )
              })}
            </ul>
          )}
        </Card>

        {/* Pending requests */}
        <Card
          title="בקשות אחרונות לטיפול"
          actions={
            <Link to="/admin/bookings" className="text-xs font-medium text-brand hover:underline">
              לכל הבקשות
            </Link>
          }
          padded={false}
        >
          {data.latestRequests.length === 0 ? (
            <Empty title="אין בקשות ממתינות 🎉" />
          ) : (
            <ul className="divide-y divide-line">
              {data.latestRequests.map((r) => (
                <li key={r.id}>
                  <Link to={`/admin/bookings?request=${r.id}`} className="block px-5 py-3 hover:bg-subtle">
                    <div className="flex items-center justify-between gap-2">
                      <span className="truncate text-sm font-medium">{r.title}</span>
                      {r.hasConflict && <Badge tone="red">התנגשות</Badge>}
                    </div>
                    <p className="mt-0.5 text-xs text-muted">
                      {r.requesterName} · {r.roomName} · {dateShort(r.startsAt)} {timeRange(r.startsAt, r.endsAt)}
                    </p>
                    <p className="mt-0.5 text-[11px] text-muted/80">התקבלה {relative(r.createdAt)}</p>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>

      <div className="grid gap-6 lg:grid-cols-2 xl:grid-cols-3">
        {/* Week chart */}
        <Card title="הזמנות השבוע">
          <div className="flex h-44 gap-2">
            {data.week.map((d, i) => {
              const today = d.date === fmt(new Date(), 'yyyy-MM-dd')
              return (
                <div key={d.date} className="flex flex-1 flex-col items-center gap-1.5">
                  <span className="text-[11px] font-semibold tabular-nums text-muted">{d.count || ''}</span>
                  <div className="flex w-full flex-1 items-end">
                    <div
                      className={clsx('w-full rounded-t-lg transition-all', today ? 'bg-brand' : 'bg-brand/30')}
                      style={{ height: `${(d.count / maxDay) * 100}%`, minHeight: d.count ? 6 : 2 }}
                      title={`${d.count} הזמנות · ${money(d.revenue)}`}
                    />
                  </div>
                  <span className={clsx('text-xs', today ? 'font-bold text-brand' : 'text-muted')}>{weekdayShort[i]}</span>
                </div>
              )
            })}
          </div>
          <div className="mt-5 space-y-2">
            {(Object.entries(data.byType) as [BookingType, number][]).map(([t, n]) => (
              <div key={t} className="flex items-center gap-3 text-xs">
                <span className={clsx('h-2.5 w-2.5 rounded-full', typeStyles[t].dot)} />
                <span className="w-32 text-muted">{bookingTypeLabel[t]}</span>
                <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-subtle">
                  <div className={clsx('h-full rounded-full', typeStyles[t].dot)} style={{ width: `${(n / typeTotal) * 100}%` }} />
                </div>
                <span className="w-6 text-end tabular-nums">{n}</span>
              </div>
            ))}
          </div>
        </Card>

        {/* Room occupancy */}
        <Card title="תפוסת חדרים השבוע">
          <ul className="space-y-3">
            {(data.roomsWeek ?? []).map((r) => (
              <li key={r.roomId}>
                <div className="mb-1 flex justify-between text-xs">
                  <span className="flex items-center gap-1.5 font-medium">
                    <span className="h-2 w-2 rounded-full" style={{ background: r.color }} />
                    {r.name}
                  </span>
                  <span className="tabular-nums text-muted">
                    {r.hours.toFixed(1)} ש׳ · {r.percent}%
                  </span>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-subtle">
                  <div className="h-full rounded-full" style={{ width: `${Math.max(r.percent, 1)}%`, background: r.color }} />
                </div>
              </li>
            ))}
          </ul>
        </Card>

        {/* Unpaid */}
        {can('payments:view') && (
          <Card
            title="יתרות גבוהות לגבייה"
            actions={
              <Link to="/admin/payments" className="text-xs font-medium text-brand hover:underline">
                לתשלומים
              </Link>
            }
            padded={false}
          >
            {data.topUnpaid.length === 0 ? (
              <Empty title="אין יתרות פתוחות" />
            ) : (
              <ul className="divide-y divide-line">
                {data.topUnpaid.map((b) => (
                  <li key={b.id} className="flex items-center justify-between gap-3 px-5 py-3">
                    <span className="min-w-0">
                      <span className="block truncate text-sm font-medium">{b.customerName || b.title}</span>
                      <span className="block truncate text-xs text-muted">
                        {b.title} · {dateShort(b.startsAt)}
                      </span>
                    </span>
                    <span className="text-end">
                      <span className="block text-sm font-semibold text-danger">{money(b.price - b.paidAmount)}</span>
                      <span className="block text-[11px] text-muted">מתוך {money(b.price)}</span>
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        )}
      </div>
    </div>
  )
}
