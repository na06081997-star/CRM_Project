import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { addDays, addMonths, format } from 'date-fns'
import { AlertTriangle, Check, CheckCircle2, Repeat, Send } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { NeedsChecklist, needsSummary, type Needs } from '@/components/NeedsChecklist'
import { SlotAlternatives } from '@/components/SlotAlternatives'
import { Badge, Button, Field, Input, Select, Spinner, Textarea, Toggle } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { useCan, useUser } from '@/lib/auth'
import { dateLong, ltr, minutesLabel } from '@/lib/format'
import { requestRouteLabel, roomKindLabel } from '@/lib/labels'
import { useActivities, useLookups } from '@/lib/queries'
import type { BookingRequest, SlotCheck } from '@/lib/types'

type Step = 1 | 2 | 3

const EVENT_KINDS = ['מפגש משפחתי', 'אירוע פרטי', 'פעילות', 'חוג', 'סדנה', 'הרצאה', 'טורניר', 'אירוע קהילתי', 'ישיבה', 'חזרה', 'אחר']
const ACTIVITY_IDEAS = ['חוג', 'סדנה', 'הרצאה', 'מפגש קהילתי', 'חזרה', 'טורניר', 'ישיבת צוות', 'הפעלה לילדים']

/** Portal / coordinator request: room and time, details with the requirements checklist, review. */
export function NewRequestPage() {
  const [params] = useSearchParams()
  const nav = useNavigate()
  const qc = useQueryClient()
  const me = useUser()
  const can = useCan()
  const coordinator = can('requests:activity')
  const canRent = can('requests:rental')
  const { rooms, equipment, buildingById } = useLookups()
  const activities = useActivities(coordinator).data ?? []
  const myActivities = activities.filter((a) => a.coordinatorId === me?.id)

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
    // Coordinators run activities and never request paid rentals.
    route: (coordinator ? 'activity' : 'rental') as 'activity' | 'rental',
    title: '',
    eventKind: coordinator ? 'פעילות' : 'מפגש משפחתי',
    attendees: 0,
    notes: '',
    repeat: false,
    until: format(addMonths(initStart ?? addDays(new Date(), 1), 3), 'yyyy-MM-dd'),
  })
  const [needs, setNeeds] = useState<Needs>({ chairs: 0, tables: 0, equipment: [] })
  const [ack, setAck] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [taken, setTaken] = useState(false)
  const [done, setDone] = useState<BookingRequest | null>(null)
  const set = <K extends keyof typeof f>(k: K, v: (typeof f)[K]) => setF((p) => ({ ...p, [k]: v }))

  const room = rooms.find((r) => r.id === f.roomId)
  const start = new Date(`${f.date}T${f.from}`)
  const end = new Date(`${f.date}T${f.to}`)
  const minutes = Math.round((end.getTime() - start.getTime()) / 60000)
  const recurring = f.route === 'activity' && f.repeat
  const validWindow = !Number.isNaN(start.getTime()) && end > start

  // Server-side check: bookings + cleaning buffers + opening hours, with alternatives.
  const [debounced, setDebounced] = useState('')
  const checkKey = JSON.stringify({ r: f.roomId, s: f.date + f.from, e: f.to, u: recurring ? f.until : '', a: f.attendees })
  useEffect(() => {
    const t = setTimeout(() => setDebounced(checkKey), 300)
    return () => clearTimeout(t)
  }, [checkKey])
  const check = useQuery({
    queryKey: ['availability', 'check', debounced],
    enabled: !!f.roomId && validWindow && debounced === checkKey,
    queryFn: () =>
      api.post<SlotCheck>('/availability/check', {
        roomId: f.roomId,
        startsAt: start.toISOString(),
        endsAt: end.toISOString(),
        attendees: Number(f.attendees) || 0,
        repeatUntil: recurring ? f.until : '',
      }),
    placeholderData: (prev) => prev,
  })
  const slot = check.data
  const occurrences = recurring ? (slot?.occurrences ?? []) : []
  const takenDates = occurrences.filter((o) => !o.free)
  const stepOneError = !room
    ? 'יש לבחור חלל'
    : !validWindow
      ? 'שעת הסיום חייבת להיות אחרי שעת ההתחלה'
      : start < new Date()
        ? 'לא ניתן לבחור מועד שעבר'
        : slot && !slot.free
          ? 'busy'
          : ''

  const send = useMutation({
    mutationFn: () =>
      api.post<BookingRequest>('/requests', {
        roomId: f.roomId,
        route: f.route,
        title: f.title.trim() || f.eventKind,
        eventKind: f.route === 'activity' ? (recurring ? 'חוג' : 'פעילות') : f.eventKind,
        name: f.name,
        phone: f.phone,
        startsAt: start.toISOString(),
        endsAt: end.toISOString(),
        attendees: Number(f.attendees) || 0,
        tablesCount: needs.tables,
        chairsCount: needs.chairs,
        equipment: needs.equipment,
        notes: f.notes,
        repeatUntil: recurring ? f.until : '',
      }),
    onSuccess: (r) => {
      setDone(r)
      for (const k of ['requests', 'blocks', 'availability']) qc.invalidateQueries({ queryKey: [k] })
    },
    onError: (e) => {
      if (e instanceof ApiError) {
        setErrors(e.fields)
        if ((e.fields.startsAt ?? '').includes('תפוס') || e.conflicts.length) setTaken(true)
        else if (Object.keys(e.fields).some((k) => ['title', 'phone', 'name', 'attendees', 'route'].includes(k))) setStep(2)
        else if (e.fields.repeatUntil) setStep(1)
      }
    },
  })

  // The end of a weekly series follows the first date (3 months later) when it would fall before it.
  const withDate = <T extends { date: string; until: string }>(p: T, date: string): T => ({
    ...p,
    date,
    until: p.until > date ? p.until : format(addMonths(new Date(`${date}T12:00`), 3), 'yyyy-MM-dd'),
  })
  const pickTime = (s: Date, e: Date) => setF((p) => ({ ...withDate(p, format(s, 'yyyy-MM-dd')), from: format(s, 'HH:mm'), to: format(e, 'HH:mm') }))

  // ── Confirmation ──
  if (done)
    return (
      <div className="mx-auto max-w-lg py-6">
        <div className="card p-6 text-center sm:p-8">
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
            {ltr(`${f.from}–${f.to}`)}
            {done.repeatUntil && (
              <>
                <br />
                <span className="inline-flex items-center gap-1 font-medium">
                  <Repeat className="h-3.5 w-3.5" /> כל שבוע עד {format(new Date(done.repeatUntil), 'dd.MM.yyyy')}
                </span>
              </>
            )}
          </p>
          <p className="mt-4 rounded-xl border-s-4 border-brand bg-brand-soft/40 px-4 py-3 text-start text-sm">הבקשה נשלחה לבדיקה. עדיין אין אישור הזמנה ואין חיוב. ניתן לעקוב באזור ״הבקשות שלי״.</p>
          <div className="mt-6 grid gap-2">
            <Button onClick={() => nav(`/portal/requests?id=${done.id}`)}>מעבר לבקשות שלי</Button>
            <Link to="/portal/new" reloadDocument className="text-sm font-medium text-brand hover:underline">
              בקשה נוספת
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
          <Row k="שעות" v={ltr(`${f.from}–${f.to}`)} />
          {minutes > 0 && <Row k="משך" v={minutesLabel(minutes)} />}
          {recurring && <Row k="חזרה" v={`כל שבוע עד ${format(new Date(f.until), 'dd.MM.yy')}`} />}
          <Row k="מחיר" v={f.route === 'activity' ? 'ללא חיוב' : 'לאחר בדיקה'} />
        </dl>
        {step > 1 && (
          <button className="mt-3 text-sm font-medium text-brand hover:underline" onClick={() => setStep(1)}>
            שינוי חלל או מועד
          </button>
        )}
        {f.route === 'rental' && <p className="mt-4 border-t border-brand/20 pt-3 text-xs text-muted">אין גבייה בטופס. אמצעי התשלום יתואם לאחר אישור המתנ״ס.</p>}
      </div>
    </aside>
  )

  return (
    <div className="space-y-5">
      {/* Stepper */}
      <ol className="flex items-center gap-1 overflow-x-auto text-sm sm:gap-2">
        {(['חלל וזמן', 'פרטים', 'בדיקה ושליחה'] as const).map((label, i) => {
          const n = (i + 1) as Step
          const state = n < step ? 'done' : n === step ? 'current' : 'next'
          return (
            <li key={label} className="flex shrink-0 items-center gap-1 sm:gap-2">
              {i > 0 && <span className="h-px w-4 bg-line sm:w-8" />}
              <button
                disabled={n > step}
                onClick={() => setStep(n)}
                className={clsx('flex items-center gap-2 rounded-full px-2.5 py-1.5', state === 'current' && 'bg-brand-soft font-semibold text-brand-strong', state === 'next' && 'text-muted')}
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
        <div className="min-w-0 space-y-5">
          {step === 1 && (
            <section className="card p-4 sm:p-6">
              <h1 className="text-xl font-bold">איזה חלל ומתי?</h1>
              <p className="mt-1 text-sm text-muted">בחרו חלל, תאריך ושעות. ניתן גם לבחור ישירות מתוך היומן.</p>
              <div className="mt-5 grid grid-cols-2 gap-3 sm:gap-4">
                <Field label="חלל *" className="col-span-2">
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
                <Field label="תאריך *" className="col-span-2 sm:col-span-1">
                  <Input type="date" min={format(new Date(), 'yyyy-MM-dd')} value={f.date} onChange={(e) => setF((p) => withDate(p, e.target.value))} />
                </Field>
                <div className="col-span-2 grid grid-cols-2 gap-3 sm:col-span-1">
                  <Field label="משעה *">
                    <Input type="time" step={900} value={f.from} onChange={(e) => set('from', e.target.value)} />
                  </Field>
                  <Field label="עד שעה *">
                    <Input type="time" step={900} value={f.to} onChange={(e) => set('to', e.target.value)} />
                  </Field>
                </div>
              </div>

              {f.route === 'activity' && (
                <div className="mt-4 rounded-xl border border-line p-3">
                  <Toggle checked={f.repeat} onChange={(v) => set('repeat', v)} label="חוג קבוע – חוזר כל שבוע באותו יום ושעה" />
                  {f.repeat && (
                    <div className="mt-3 grid gap-3 sm:grid-cols-2">
                      <Field label="עד תאריך" error={errors.repeatUntil} hint="עד שנה קדימה">
                        <Input type="date" min={f.date} max={format(addMonths(new Date(`${f.date}T12:00`), 12), 'yyyy-MM-dd')} value={f.until} onChange={(e) => set('until', e.target.value)} />
                      </Field>
                      {occurrences.length > 0 && (
                        <div className="text-sm">
                          <p className="font-semibold">{occurrences.length} מפגשים</p>
                          {takenDates.length > 0 ? (
                            <p className="mt-1 text-xs text-amber-700 dark:text-amber-300">
                              {takenDates.length} תאריכים תפוסים ולא ייכללו: {takenDates.map((o) => format(new Date(o.startsAt), 'dd.MM')).join(', ')}
                            </p>
                          ) : (
                            <p className="mt-1 text-xs text-ok">כל המפגשים פנויים כרגע</p>
                          )}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )}

              {f.roomId && validWindow && (
                <div className="mt-4">
                  {check.isError ? (
                    <p className="flex items-center gap-2 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-800 dark:bg-red-950/30 dark:text-red-200">
                      <AlertTriangle className="h-4 w-4 shrink-0" /> {errorMessage(check.error)}
                    </p>
                  ) : check.isFetching && !slot ? (
                    <div className="flex items-center gap-2 text-sm text-muted">
                      <Spinner /> בודק זמינות…
                    </div>
                  ) : stepOneError === 'busy' && slot ? (
                    <SlotAlternatives {...slot} onPickTime={pickTime} onPickRoom={(id) => set('roomId', id)} />
                  ) : (
                    <div className={clsx('flex items-center gap-2 rounded-xl px-4 py-3 text-sm', stepOneError ? 'bg-red-50 text-red-800 dark:bg-red-950/30 dark:text-red-200' : 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/30 dark:text-emerald-200')}>
                      {stepOneError ? <AlertTriangle className="h-4 w-4 shrink-0" /> : <CheckCircle2 className="h-4 w-4 shrink-0" />}
                      {stepOneError || 'פנוי לבקשה'}
                      <Link to={`/portal/rooms/${f.roomId}?date=${f.date}`} className="ms-auto shrink-0 text-xs font-medium underline">
                        יומן החלל
                      </Link>
                    </div>
                  )}
                </div>
              )}
              <div className="mt-6 flex justify-end">
                <Button className="max-sm:w-full" disabled={!!stepOneError || !slot || check.isError} onClick={() => setStep(2)}>
                  המשך לפרטים
                </Button>
              </div>
            </section>
          )}

          {step === 2 && room && (
            <>
              <section className="card p-4 sm:p-6">
                <h1 className="text-xl font-bold">{f.route === 'activity' ? 'מה הפעילות?' : 'מי מזמין ומה מתוכנן?'}</h1>
                <p className="mt-1 text-sm text-muted">כל השדות המסומנים ב־* נדרשים.</p>
                <div className="mt-5 grid gap-4 sm:grid-cols-2">
                  {f.route === 'activity' ? (
                    <Field label="שם הפעילות *" error={errors.title} className="sm:col-span-2" hint="כתבו בחופשיות – לדוגמה: חוג ציור לילדים, הרצאה על בריאות">
                      <Input value={f.title} onChange={(e) => set('title', e.target.value)} list="activity-ideas" placeholder="מה מתקיים בחלל?" autoFocus />
                      <datalist id="activity-ideas">
                        {[...new Set([...myActivities.map((a) => a.name), ...ACTIVITY_IDEAS])].map((n) => (
                          <option key={n} value={n} />
                        ))}
                      </datalist>
                    </Field>
                  ) : (
                    <>
                      <Field label="סוג אירוע *">
                        <Select value={f.eventKind} onChange={(e) => set('eventKind', e.target.value)}>
                          {EVENT_KINDS.map((k) => (
                            <option key={k}>{k}</option>
                          ))}
                        </Select>
                      </Field>
                      <Field label="שם האירוע" error={errors.title}>
                        <Input value={f.title} onChange={(e) => set('title', e.target.value)} placeholder={f.eventKind} />
                      </Field>
                    </>
                  )}
                  {coordinator && canRent && (
                    <Field label="מסלול" error={errors.route}>
                      <Select value={f.route} onChange={(e) => set('route', e.target.value as 'activity' | 'rental')}>
                        <option value="activity">{requestRouteLabel.activity}</option>
                        <option value="rental">{requestRouteLabel.rental}</option>
                      </Select>
                    </Field>
                  )}
                  <Field label="איש קשר *" error={errors.name}>
                    <Input value={f.name} onChange={(e) => set('name', e.target.value)} />
                  </Field>
                  <Field label="טלפון *" error={errors.phone}>
                    <Input type="tel" value={f.phone} onChange={(e) => set('phone', e.target.value)} placeholder="050-000-0000" />
                  </Field>
                  <Field label="מספר משתתפים" error={errors.attendees} hint={room.capacity ? `קיבולת: ${room.capacity}` : undefined}>
                    <Input type="number" inputMode="numeric" min={0} value={f.attendees} onChange={(e) => set('attendees', Number(e.target.value))} />
                  </Field>
                </div>

                <div className="mt-6 border-t border-line pt-5">
                  <h2 className="font-semibold">מה צריך בחלל?</h2>
                  <p className="mb-3 mt-0.5 text-xs text-muted">הדרישות נבדקות עם הבקשה; סימון ״כן״ אינו אישור לאספקה.</p>
                  <NeedsChecklist value={needs} onChange={setNeeds} roomEquipment={room.equipment} catalog={equipment} />
                </div>
                <Field label="בקשות מיוחדות / הערות" className="mt-5">
                  <Textarea value={f.notes} onChange={(e) => set('notes', e.target.value)} />
                </Field>
              </section>
              <div className="flex justify-end gap-2 max-sm:[&>*]:flex-1">
                <Button variant="secondary" onClick={() => setStep(1)}>
                  חזרה
                </Button>
                <Button onClick={() => setStep(3)} disabled={!f.name.trim() || !f.phone.trim() || (f.route === 'activity' && !f.title.trim())}>
                  המשך לבדיקה
                </Button>
              </div>
            </>
          )}

          {step === 3 && room && (
            <section className="card p-4 sm:p-6">
              <Badge tone="gray">לפני השליחה</Badge>
              <h1 className="mt-3 text-xl font-bold">הפרטים נכונים?</h1>
              <dl className="mt-4 divide-y divide-line text-sm">
                <Line k="חלל" v={room.name} />
                <Line k="מועד" v={`${format(start, 'dd.MM.yyyy')} · ${ltr(`${f.from}–${f.to}`)}`} />
                {recurring && <Line k="חזרה" v={`כל שבוע עד ${format(new Date(f.until), 'dd.MM.yyyy')}${occurrences.length ? ` · ${occurrences.length - takenDates.length} מפגשים פנויים` : ''}`} />}
                <Line k="איש קשר" v={`${f.name} · ${ltr(f.phone)}`} />
                <Line k={f.route === 'activity' ? 'פעילות' : 'אירוע'} v={`${f.title || f.eventKind} · ${requestRouteLabel[f.route]}`} />
                {f.attendees > 0 && <Line k="משתתפים" v={String(f.attendees)} />}
                <Line k="דרישות" v={needsSummary(needs, equipment)} />
                {f.notes && <Line k="הערות" v={f.notes} />}
              </dl>
              <p className="mt-4 rounded-xl bg-subtle px-4 py-3 text-sm">
                <b>לתשומת לבך:</b> זו בקשה בלבד. החלל אינו שמור לפני אישור המתנ״ס.
              </p>
              <label className="mt-4 flex items-center gap-2 text-sm">
                <input type="checkbox" checked={ack} onChange={(e) => setAck(e.target.checked)} className="h-5 w-5 accent-[rgb(var(--brand))]" />
                קראתי שהשליחה אינה אישור הזמנה.
              </label>
              {taken && (
                <p className="mt-4 flex items-start gap-2 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-800 dark:bg-red-950/30 dark:text-red-200">
                  <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                  החלל כבר אינו פנוי בשעות שבחרת. הבקשה לא נשלחה – חזרו לבחירת מועד לקבלת הצעות חלופיות.
                </p>
              )}
              {send.isError && !taken && <p className="mt-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-800 dark:bg-red-950/30 dark:text-red-200">{errorMessage(send.error)} – הפרטים נשמרו, ניתן לנסות שוב.</p>}
              <div className="mt-6 flex justify-end gap-2 max-sm:[&>*]:flex-1">
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
    <div className="grid grid-cols-[84px_1fr] gap-3 py-2.5 sm:grid-cols-[100px_1fr]">
      <dt className="text-muted">{k}</dt>
      <dd className="min-w-0 break-words font-medium">{v}</dd>
    </div>
  )
}
