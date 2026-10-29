import clsx from 'clsx'
import { useQuery } from '@tanstack/react-query'
import { addDays, format } from 'date-fns'
import { CalendarSearch, CheckCircle2, Search, Users, XCircle } from 'lucide-react'
import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Badge, Button, Empty, Field, Input, PageLoader, Select, Toggle } from '@/components/ui'
import { api } from '@/lib/api'
import { dateLong, time, timeRange } from '@/lib/format'
import { roomKindLabel } from '@/lib/labels'
import { useLookups } from '@/lib/queries'
import type { RoomAvailability } from '@/lib/types'

/** Mockup portal screen 1: "מתי פנוי, ואיזה חלל מתאים לי?" */
export function SearchPage() {
  const [params, setParams] = useSearchParams()
  const [date, setDate] = useState(params.get('date') ?? format(addDays(new Date(), 1), 'yyyy-MM-dd'))
  const [from, setFrom] = useState(params.get('from') ?? '18:00')
  const [to, setTo] = useState(params.get('to') ?? '20:00')
  const [kind, setKind] = useState(params.get('kind') ?? '')
  const [onlyFree, setOnlyFree] = useState(false)
  const { equipmentLabel, buildingById } = useLookups()
  // The search runs on submitted values (URL), not on every keystroke.
  const [initial] = useState({ date, from, to })
  const query = { date: params.get('date') ?? initial.date, from: params.get('from') ?? initial.from, to: params.get('to') ?? initial.to, kind: params.get('kind') ?? '' }

  const { data, isFetching } = useQuery({
    queryKey: ['availability', query],
    queryFn: () => api.get<RoomAvailability[]>('/availability', { ...query, kind: query.kind || undefined }),
  })
  const all = data ?? []
  const free = all.filter((r) => r.windowFree)
  const shown = (onlyFree ? free : all).slice().sort((a, b) => Number(b.windowFree) - Number(a.windowFree))
  const startISO = new Date(`${query.date}T${query.from}`).toISOString()
  const endISO = new Date(`${query.date}T${query.to}`).toISOString()

  return (
    <div className="space-y-6">
      <section className="rounded-2xl border border-brand/20 bg-brand-soft/50 p-5">
        <h1 className="text-2xl font-bold">מוצאים חלל. בוחרים זמן. שולחים בקשה.</h1>
        <p className="mt-1 text-sm text-muted">בחרו תאריך ושעות כדי לראות אילו חדרים ואולמות פנויים עבורכם.</p>
        <form
          className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-[1.2fr_1fr_1fr_1.2fr_auto]"
          onSubmit={(e) => {
            e.preventDefault()
            setParams({ date, from, to, ...(kind ? { kind } : {}) })
          }}
        >
          <Field label="תאריך">
            <Input type="date" value={date} min={format(new Date(), 'yyyy-MM-dd')} onChange={(e) => setDate(e.target.value)} />
          </Field>
          <Field label="שעת התחלה">
            <Input type="time" step={900} value={from} onChange={(e) => setFrom(e.target.value)} />
          </Field>
          <Field label="שעת סיום">
            <Input type="time" step={900} value={to} onChange={(e) => setTo(e.target.value)} />
          </Field>
          <Field label="סוג חלל">
            <Select value={kind} onChange={(e) => setKind(e.target.value)}>
              <option value="">כל החללים</option>
              {Object.entries(roomKindLabel).map(([k, v]) => (
                <option key={k} value={k}>
                  {v}
                </option>
              ))}
            </Select>
          </Field>
          <Button type="submit" className="self-end" icon={<Search className="h-4 w-4" />} loading={isFetching}>
            חיפוש זמינות
          </Button>
        </form>
      </section>

      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold">
            {free.length} חללים פנויים מתוך {all.length} בתאריך ובשעות שבחרת
          </h2>
          <p className="text-xs text-muted">
            {dateLong(`${query.date}T12:00:00`)} · {query.from}–{query.to} · הזמינות עשויה להשתנות עד לאישור
          </p>
        </div>
        <Toggle checked={onlyFree} onChange={setOnlyFree} label="הצגת פנויים בלבד" />
      </div>

      {!data ? (
        <PageLoader />
      ) : shown.length === 0 ? (
        <div className="card">
          <Empty icon={<CalendarSearch className="h-6 w-6" />} title="לא נמצאו חללים מתאימים" hint="נסו תאריך, שעות או סוג חלל אחרים" />
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {shown.map((r) => {
            const roomLink = `/portal/rooms/${r.room.id}?date=${query.date}`
            return (
              <article key={r.room.id} className={clsx('card flex flex-col overflow-hidden transition', r.windowFree ? 'hover:shadow-pop' : 'opacity-80')}>
                <div className="h-1.5" style={{ background: r.room.color }} />
                <div className="flex flex-1 flex-col p-4">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <h3 className="truncate font-semibold">{r.room.name}</h3>
                      <p className="text-xs text-muted">
                        {buildingById.get(r.room.buildingId)?.name} · {roomKindLabel[r.room.kind]}
                      </p>
                    </div>
                    {r.windowFree ? (
                      <Badge tone="green">
                        <CheckCircle2 className="h-3 w-3" /> פנוי לבקשה
                      </Badge>
                    ) : (
                      <Badge tone="gray">
                        <XCircle className="h-3 w-3" /> תפוס בשעות שבחרת
                      </Badge>
                    )}
                  </div>
                  <p className="mt-3 flex items-center gap-1 text-xs text-muted">
                    <Users className="h-3.5 w-3.5" /> עד {r.room.capacity} משתתפים
                  </p>
                  {r.room.equipment.length > 0 && <p className="mt-1 text-xs text-muted">{r.room.equipment.map(equipmentLabel).join(' · ')}</p>}
                  <div className="mt-3">
                    <p className="mb-1.5 text-[11px] font-medium text-muted">{r.windowFree ? 'זמנים פנויים נוספים ביום זה' : 'הצגת מועדים חלופיים'}</p>
                    <div className="flex flex-wrap gap-1">
                      {(r.freeSlots ?? []).slice(0, 4).map((s) => (
                        <Link
                          key={s.start}
                          to={`/portal/new?room=${r.room.id}&start=${encodeURIComponent(s.start)}&end=${encodeURIComponent(new Date(Math.min(new Date(s.end).getTime(), new Date(s.start).getTime() + 2 * 3600_000)).toISOString())}`}
                          className="rounded-lg bg-emerald-50 px-2 py-0.5 text-[11px] tabular-nums text-emerald-700 hover:bg-emerald-100 dark:bg-emerald-950 dark:text-emerald-300"
                          title={`בקשה מ־${time(s.start)}`}
                        >
                          {timeRange(s.start, s.end)}
                        </Link>
                      ))}
                      {(r.freeSlots ?? []).length === 0 && <span className="text-[11px] text-muted">אין זמנים פנויים ביום זה</span>}
                    </div>
                  </div>
                  <div className="mt-auto grid gap-2 pt-4">
                    {r.windowFree && (
                      <Link to={`/portal/new?room=${r.room.id}&start=${encodeURIComponent(startISO)}&end=${encodeURIComponent(endISO)}`}>
                        <Button size="sm" className="w-full">
                          בקשה למועד זה
                        </Button>
                      </Link>
                    )}
                    <Link to={roomLink}>
                      <Button size="sm" variant={r.windowFree ? 'secondary' : 'soft'} className="w-full">
                        {r.windowFree ? 'פרטים ויומן זמינות' : 'מתי החלל פנוי?'}
                      </Button>
                    </Link>
                  </div>
                </div>
              </article>
            )
          })}
        </div>
      )}
      <p className="text-center text-xs text-muted">כל המחירים מתקבלים לאחר בדיקת הבקשה. ״פנוי לבקשה״ אינו אישור הזמנה ואין תשלום בשלב החיפוש.</p>
    </div>
  )
}
