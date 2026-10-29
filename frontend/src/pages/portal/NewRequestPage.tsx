import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { addDays, format, startOfDay } from 'date-fns'
import { AlertTriangle, Check, CheckCircle2, Send } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { Badge, Button, Field, Input, Select, Textarea } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { useCan, useUser } from '@/lib/auth'
import { dateLong, minutesLabel } from '@/lib/format'
import { requestRouteLabel, roomKindLabel } from '@/lib/labels'
import { useActivities, useLookups } from '@/lib/queries'
import type { Block, BookingRequest } from '@/lib/types'

type Step = 1 | 2 | 3

const EVENT_KINDS = ['מפגש משפחתי', 'אירוע פרטי', 'פעילות', 'חוג', 'סדנה', 'הרצאה', 'טורניר', 'אירוע קהילתי', 'ישיבה', 'חזרה', 'אחר']

/** Mockup portal screens 3–4: short request form adapted to the room, review, then confirmation. */
export function NewRequestPage() {
  const [params] = useSearchParams()
  const nav = useNavigate()
  const qc = useQueryClient()
  const me = useUser()
  const can = useCan()
  const coordinator = can('requests:activity')
  const { rooms, equipment, buildingById } = useLookups()
  const activities = useActivities(coordinator).data ?? []

  const initStart = params.get('start') ? new Date(params.get('start')!) : null
  const initEnd = params.get('end') ? new Date(params.get('end')!) : null
  const [step, setStep] = useState<Step>(params.get('room') && initStart ? 2 : 1)
  const [f, setF] = useState({
    roomId: params.get('room') ?? '',
    date: format(initStart ?? addDays(new Date(), 1), 'yyyy-MM-dd'),
    from: initStart ? format(initStart, 'HH:mm') : '18:00',
    to: initEnd ? format(initEnd, 'HH:mm') : '20:00',
    name: me?.fullName ?? '',
    phone: me?.phone ?? '',
    route: (coordinator ? 'activity' : 'rental') as 'activity' | 'rental',
    activityId: '',
    title: '',
    eventKind: coordinator ? 'פעילות' : 'מפגש משפחתי',
    attendees: 0,
    tables: 0,
    chairs: 0,
    eq: [] as string[],
    notes: '',
  })
  const [ack, setAck] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [taken, setTaken] = useState(false)
  const [done, setDone] = useState<BookingRequest | null>(null)
  const set = <K extends keyof typeof f>(k: K, v: (typeof f)[K]) => setF((p) => ({ ...p, [k]: v }))

  const room = rooms.find((r) => r.id === f.roomId)
  const start = new Date(`${f.date}T${f.from}`)
  const end = new Date(`${f.date}T${f.to}`)
  const minutes = Math.round((end.getTime() - start.getTime()) / 60000)
  const isHall = room?.kind === 'event_hall'

  // Availability of the chosen room that day (anonymised blocks; pending requests don't block).
  const day = startOfDay(start)
  const { data: blocks = [] } = useQuery({
    queryKey: ['blocks', 'day', f.roomId, f.date],
    enabled: !!f.roomId && !Number.isNaN(day.getTime()),
    queryFn: () => api.get<Block[]>('/availability/calendar', { from: day.toISOString(), to: addDays(day, 1).toISOString(), roomIds: [f.roomId] }),
  })
  const conflict = useMemo(() => blocks.some((b) => !b.request && new Date(b.startsAt) < end && new Date(b.endsAt) > start), [blocks, start, end])
  const stepOneError = !room ? 'יש לבחור חלל' : !(end > start) ? 'שעת הסיום חייבת להיות אחרי שעת ההתחלה' : start < new Date() ? 'לא ניתן לבחור מועד שעבר' : conflict ? 'החלל תפוס בשעות שבחרת' : ''

  const send = useMutation({
    mutationFn: () =>
      api.post<BookingRequest>('/requests', {
        roomId: f.roomId,
        route: f.route,
        activityId: f.route === 'activity' ? f.activityId || null : null,
        title: f.title || f.eventKind,
        eventKind: f.eventKind,
        name: f.name,
        phone: f.phone,
        startsAt: start.toISOString(),
        endsAt: end.toISOString(),
        attendees: Number(f.attendees) || 0,
        tablesCount: isHall ? f.tables : 0,
        chairsCount: isHall ? f.chairs : 0,
        equipment: f.eq,
        notes: f.notes,
      }),
    onSuccess: (r) => {
      setDone(r)
      for (const k of ['requests', 'blocks', 'availability']) qc.invalidateQueries({ queryKey: [k] })
    },
    onError: (e) => {
      if (e instanceof ApiError) {
        setErrors(e.fields)
        if ((e.fields.startsAt ?? '').includes('תפוס') || e.conflicts.length) setTaken(true)
        else if (Object.keys(e.fields).some((k) => ['title', 'phone', 'name', 'attendees', 'activityId'].includes(k))) setStep(2)
      }
    },
  })

  const reqLabel = (code: string) => equipment.find((e) => e.code === code)?.label ?? code

  // ── Confirmation (screen 4, right side) ──
  if (done)
    return (
      <div className="mx-auto max-w-lg py-6">
        <div className="card p-8 text-center">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-emerald-50 text-ok dark:bg-emerald-950">
            <Check className="h-7 w-7" strokeWidth={3} />
          </div>
          <h1 className="mt-4 text-2xl font-bold">הבקשה נקלטה</h1>
          <p className="mt-1 font-semibold">מספר הבקשה שלך: #{done.number}</p>
          <Badge tone="amber" className="mt-3">
            ממתינה לאישור המתנ״ס
          </Badge>
          <p className="mt-4 text-sm">
            {done.roomName} · {dateLong(done.startsAt)}
            <br />
            {f.from}–{f.to}
          </p>
          <p className="mt-4 rounded-xl border-s-4 border-brand bg-brand-soft/40 px-4 py-3 text-start text-sm">הבקשה נשלחה לבדיקה. עדיין אין אישור הזמנה ואין חיוב. ניתן לעקוב באזור ״הבקשות שלי״.</p>
          <div className="mt-6 grid gap-2">
            <Button onClick={() => nav(`/portal/requests?id=${done.id}`)}>מעבר לבקשות שלי</Button>
            <Link to="/portal/search" className="text-sm font-medium text-brand hover:underline">
              חיפוש חלל נוסף
            </Link>
          </div>
        </div>
      </div>
    )

  const Summary = (
    <aside className="space-y-4">
      <div className="rounded-2xl border border-brand/25 bg-brand-soft/40 p-5">
        <h3 className="text-sm font-semibold">סיכום הבחירה</h3>
        <p className="mt-2 text-xl font-bold">{room?.name ?? '—'}</p>
        <dl className="mt-3 space-y-2 text-sm">
          <Row k="תאריך" v={Number.isNaN(start.getTime()) ? '—' : format(start, 'dd.MM.yyyy')} />
          <Row k="שעות" v={`${f.from}–${f.to}`} />
          {minutes > 0 && <Row k="משך" v={minutesLabel(minutes)} />}
          <Row k="מחיר" v={f.route === 'activity' ? 'ללא חיוב אוטומטי' : 'לאחר בדיקה'} />
        </dl>
        {step > 1 && (
          <button className="mt-3 text-sm font-medium text-brand hover:underline" onClick={() => setStep(1)}>
            שינוי חלל או מועד
          </button>
        )}
        <p className="mt-4 border-t border-brand/20 pt-3 text-xs text-muted">אין גבייה בטופס. אמצעי התשלום יתואם לאחר אישור המתנ״ס.</p>
      </div>
      {!coordinator && <p className="rounded-xl border-s-4 border-brand bg-subtle px-4 py-3 text-xs text-muted">משתמש רגיל אינו בוחר רכז ואינו עורך מחיר, סטטוס אישור או ״שולם״.</p>}
    </aside>
  )

  return (
    <div className="space-y-5">
      {/* Stepper */}
      <ol className="flex flex-wrap items-center gap-2 text-sm">
        {(['בחירת חלל וזמן', 'פרטי הבקשה', 'בדיקה ושליחה'] as const).map((label, i) => {
          const n = (i + 1) as Step
          const state = n < step ? 'done' : n === step ? 'current' : 'next'
          return (
            <li key={label} className="flex items-center gap-2">
              {i > 0 && <span className="h-px w-8 bg-line" />}
              <button
                disabled={n > step}
                onClick={() => setStep(n)}
                className={clsx('flex items-center gap-2 rounded-full px-3 py-1.5', state === 'current' && 'bg-brand-soft font-semibold text-brand-strong', state === 'next' && 'text-muted')}
              >
                <span className={clsx('flex h-6 w-6 items-center justify-center rounded-full text-xs font-bold', state === 'done' ? 'bg-ok text-white' : state === 'current' ? 'bg-brand text-white dark:text-slate-900' : 'bg-subtle')}>
                  {state === 'done' ? <Check className="h-3.5 w-3.5" /> : n}
                </span>
                {label}
              </button>
            </li>
          )
        })}
      </ol>

      <div className="grid gap-5 xl:grid-cols-[1fr_320px]">
        <div className="space-y-5">
          {step === 1 && (
            <section className="card p-6">
              <h1 className="text-xl font-bold">איזה חלל ומתי?</h1>
              <p className="mt-1 text-sm text-muted">בחרו חלל, תאריך ושעות. ניתן גם לבחור ישירות מתוך היומן.</p>
              <div className="mt-5 grid gap-4 sm:grid-cols-2">
                <Field label="חלל *">
                  <Select value={f.roomId} onChange={(e) => set('roomId', e.target.value)}>
                    <option value="">בחרו חלל</option>
                    {rooms
                      .filter((r) => r.isBookable)
                      .map((r) => (
                        <option key={r.id} value={r.id}>
                          {r.name} · {buildingById.get(r.buildingId)?.name} · {roomKindLabel[r.kind]}
                        </option>
                      ))}
                  </Select>
                </Field>
                <Field label="תאריך הפעילות *">
                  <Input type="date" min={format(new Date(), 'yyyy-MM-dd')} value={f.date} onChange={(e) => set('date', e.target.value)} />
                </Field>
                <Field label="שעת התחלה *">
                  <Input type="time" step={900} value={f.from} onChange={(e) => set('from', e.target.value)} />
                </Field>
                <Field label="שעת סיום *">
                  <Input type="time" step={900} value={f.to} onChange={(e) => set('to', e.target.value)} />
                </Field>
              </div>
              {f.roomId && (
                <div className={clsx('mt-4 flex items-center gap-2 rounded-xl px-4 py-3 text-sm', stepOneError ? 'bg-red-50 text-red-800 dark:bg-red-950/30 dark:text-red-200' : 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/30 dark:text-emerald-200')}>
                  {stepOneError ? <AlertTriangle className="h-4 w-4" /> : <CheckCircle2 className="h-4 w-4" />}
                  {stepOneError || 'פנוי לבקשה'}
                  {f.roomId && (
                    <Link to={`/portal/rooms/${f.roomId}?date=${f.date}`} className="ms-auto text-xs font-medium underline">
                      יומן החלל
                    </Link>
                  )}
                </div>
              )}
              <div className="mt-6 flex justify-end">
                <Button disabled={!!stepOneError} onClick={() => setStep(2)}>
                  המשך לפרטי הבקשה
                </Button>
              </div>
            </section>
          )}

          {step === 2 && room && (
            <>
              <section className="card p-6">
                <h1 className="text-xl font-bold">מי מזמין ומה מתוכנן?</h1>
                <p className="mt-1 text-sm text-muted">כל השדות המסומנים ב־* נדרשים. הפרטים ניתנים לתיקון לפני השליחה.</p>
                <div className="mt-5 grid gap-4 sm:grid-cols-2">
                  <Field label="שם המזמין *" error={errors.name}>
                    <Input value={f.name} onChange={(e) => set('name', e.target.value)} />
                  </Field>
                  <Field label="מספר טלפון *" error={errors.phone}>
                    <Input dir="ltr" value={f.phone} onChange={(e) => set('phone', e.target.value)} placeholder="050-000-0000" />
                  </Field>
                  <Field label="מסלול" error={errors.route}>
                    {coordinator ? (
                      <Select value={f.route} onChange={(e) => set('route', e.target.value as 'activity' | 'rental')}>
                        <option value="activity">{requestRouteLabel.activity}</option>
                        <option value="rental">{requestRouteLabel.rental}</option>
                      </Select>
                    ) : (
                      <Input value={requestRouteLabel.rental} disabled />
                    )}
                  </Field>
                  <Field label={f.route === 'activity' ? 'סוג פעילות *' : 'סוג אירוע *'}>
                    <Select value={f.eventKind} onChange={(e) => set('eventKind', e.target.value)}>
                      {EVENT_KINDS.map((k) => (
                        <option key={k}>{k}</option>
                      ))}
                    </Select>
                  </Field>
                  {f.route === 'activity' && (
                    <Field label="פעילות" error={errors.activityId}>
                      <Select
                        value={f.activityId}
                        onChange={(e) => {
                          const a = activities.find((x) => x.id === e.target.value)
                          setF((p) => ({ ...p, activityId: e.target.value, title: p.title || a?.name || '' }))
                        }}
                      >
                        <option value="">פעילות חדשה / חד־פעמית</option>
                        {activities.map((a) => (
                          <option key={a.id} value={a.id}>
                            {a.name}
                          </option>
                        ))}
                      </Select>
                    </Field>
                  )}
                  <Field label="שם האירוע / הפעילות" error={errors.title} className={f.route === 'activity' ? '' : 'sm:col-span-2'}>
                    <Input value={f.title} onChange={(e) => set('title', e.target.value)} placeholder={f.eventKind} />
                  </Field>
                  <Field label="מספר משתתפים" error={errors.attendees} hint={room.capacity ? `קיבולת: ${room.capacity}` : undefined}>
                    <Input type="number" min={0} value={f.attendees} onChange={(e) => set('attendees', Number(e.target.value))} />
                  </Field>
                  {isHall && f.route === 'activity' && (
                    <>
                      <Field label="כמות שולחנות">
                        <Input type="number" min={0} value={f.tables} onChange={(e) => set('tables', Number(e.target.value))} />
                      </Field>
                      <Field label="כמות כיסאות">
                        <Input type="number" min={0} value={f.chairs} onChange={(e) => set('chairs', Number(e.target.value))} />
                      </Field>
                    </>
                  )}
                </div>

                {room.equipment.length > 0 && (
                  <div className="mt-6 border-t border-line pt-5">
                    <h2 className="font-semibold">מה צריך ב{room.name}?</h2>
                    <div className="mt-3 grid gap-4 sm:grid-cols-3">
                      {room.equipment.map((code) => {
                        const on = f.eq.includes(code)
                        return (
                          <div key={code}>
                            <span className="label">{reqLabel(code)}</span>
                            <div className="inline-flex rounded-xl border border-line p-1">
                              {[true, false].map((v) => (
                                <button
                                  key={String(v)}
                                  type="button"
                                  onClick={() => set('eq', v ? [...new Set([...f.eq, code])] : f.eq.filter((x) => x !== code))}
                                  className={clsx('rounded-lg px-4 py-1 text-sm', on === v ? 'bg-brand-soft font-semibold text-brand-strong ring-1 ring-brand/40' : 'text-muted')}
                                >
                                  {v ? 'כן' : 'לא'}
                                </button>
                              ))}
                            </div>
                          </div>
                        )
                      })}
                    </div>
                    <p className="mt-3 text-xs text-muted">ציוד ודרישות נבדקים עם הבקשה; סימון ״כן״ אינו אישור לאספקה.</p>
                  </div>
                )}
                <Field label="בקשות מיוחדות / הערות" className="mt-5">
                  <Textarea value={f.notes} onChange={(e) => set('notes', e.target.value)} />
                </Field>
              </section>
              <div className="flex justify-end gap-2">
                <Button variant="secondary" onClick={() => setStep(1)}>
                  חזרה לבחירת מועד
                </Button>
                <Button onClick={() => setStep(3)} disabled={!f.name.trim() || !f.phone.trim()}>
                  המשך לבדיקה
                </Button>
              </div>
            </>
          )}

          {step === 3 && room && (
            <section className="card p-6">
              <Badge tone="gray">לפני השליחה</Badge>
              <h1 className="mt-3 text-xl font-bold">הפרטים נכונים?</h1>
              <dl className="mt-4 divide-y divide-line text-sm">
                <Line k="חלל" v={room.name} />
                <Line k="מועד" v={`${format(start, 'dd.MM.yyyy')} · ${f.from}–${f.to}`} />
                <Line k="מזמין" v={`${f.name} · ${f.phone}`} />
                <Line k="פעילות" v={`${f.title || f.eventKind} · ${requestRouteLabel[f.route]}`} />
                {f.attendees > 0 && <Line k="משתתפים" v={String(f.attendees)} />}
                {isHall && f.route === 'activity' && (f.tables > 0 || f.chairs > 0) && <Line k="שולחנות / כיסאות" v={`${f.tables} / ${f.chairs}`} />}
                {room.equipment.length > 0 && <Line k="דרישות" v={room.equipment.map((c) => `${reqLabel(c)}: ${f.eq.includes(c) ? 'כן' : 'לא'}`).join(' · ')} />}
                {f.notes && <Line k="הערות" v={f.notes} />}
              </dl>
              <p className="mt-4 rounded-xl bg-subtle px-4 py-3 text-sm">
                <b>לתשומת לבך:</b> זו בקשה בלבד. החלל אינו שמור לפני אישור המתנ״ס. מחיר ותנאי שימוש יימסרו לאחר בדיקה.
              </p>
              <label className="mt-4 flex items-center gap-2 text-sm">
                <input type="checkbox" checked={ack} onChange={(e) => setAck(e.target.checked)} className="h-4 w-4 accent-[rgb(var(--brand))]" />
                קראתי שהשליחה אינה אישור הזמנה.
              </label>
              {taken && (
                <p className="mt-4 flex items-start gap-2 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-800 dark:bg-red-950/30 dark:text-red-200">
                  <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                  ״החלל כבר אינו פנוי בשעות שבחרת. הבקשה לא נשלחה. בחרו מועד אחר או חלל חלופי.״
                </p>
              )}
              {send.isError && !taken && <p className="mt-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-800 dark:bg-red-950/30 dark:text-red-200">{errorMessage(send.error)} – הפרטים נשמרו, ניתן לנסות שוב.</p>}
              <div className="mt-6 flex justify-end gap-2">
                <Button variant="secondary" onClick={() => (taken ? setStep(1) : setStep(2))}>
                  {taken ? 'בחירת מועד אחר' : 'חזרה לעריכה'}
                </Button>
                <Button icon={<Send className="h-4 w-4" />} disabled={!ack || taken} loading={send.isPending} onClick={() => send.mutate()}>
                  שליחת בקשה
                </Button>
              </div>
            </section>
          )}
        </div>
        {Summary}
      </div>
    </div>
  )
}

function Row({ k, v }: { k: string; v: string }) {
  return (
    <div className="flex justify-between gap-2">
      <dt className="text-muted">{k}</dt>
      <dd className="font-semibold">{v}</dd>
    </div>
  )
}

function Line({ k, v }: { k: string; v: string }) {
  return (
    <div className="grid grid-cols-[100px_1fr] gap-3 py-2.5">
      <dt className="text-muted">{k}</dt>
      <dd className="font-medium">{v}</dd>
    </div>
  )
}
