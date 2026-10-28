import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { addDays, startOfDay } from 'date-fns'
import { CalendarDays, Clock, Pencil, Plus, Timer, Users } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Attachments } from '@/components/Attachments'
import { toast } from '@/components/toast'
import { Badge, Button, Field, Input, Modal, PageHeader, PageLoader, Select, Textarea, Toggle } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { useCan } from '@/lib/auth'
import { money, timeRange } from '@/lib/format'
import { roomKindLabel } from '@/lib/labels'
import { usePrefs } from '@/lib/prefs'
import { useLookups } from '@/lib/queries'
import type { Booking, Room } from '@/lib/types'

const COLORS = ['#7c3aed', '#0f766e', '#0369a1', '#0891b2', '#db2777', '#ca8a04', '#ea580c', '#65a30d', '#be123c', '#475569']

export function RoomsPage() {
  const can = useCan()
  const { rooms, buildings, equipmentLabel } = useLookups()
  const prefBuilding = usePrefs((s) => s.buildingId)
  const [edit, setEdit] = useState<Partial<Room> | null>(null)
  const today = startOfDay(new Date())
  const { data: todays = [] } = useQuery({
    queryKey: ['bookings', 'today-rooms'],
    queryFn: () => api.get<Booking[]>('/bookings', { from: today.toISOString(), to: addDays(today, 1).toISOString(), statuses: ['confirmed'] }),
    refetchInterval: 60_000,
  })
  const shown = rooms.filter((r) => !prefBuilding || r.buildingId === prefBuilding)
  const now = Date.now()

  if (!rooms.length && !buildings.length) return <PageLoader />

  return (
    <div>
      <PageHeader
        title="חדרים ואולמות"
        subtitle={`${shown.length} חללים · מחירון, כללים, ציוד וזמני חציצה`}
        actions={
          can('rooms:manage') && (
            <Button icon={<Plus className="h-4 w-4" />} onClick={() => setEdit({ buildingId: prefBuilding ?? buildings[0]?.id, color: COLORS[0], isActive: true, isBookable: true, bufferMinutes: 15, minHours: 1, equipment: [], kind: 'other' })}>
              חדר חדש
            </Button>
          )
        }
      />
      {buildings
        .filter((b) => !prefBuilding || b.id === prefBuilding)
        .map((b) => {
          const list = shown.filter((r) => r.buildingId === b.id)
          if (!list.length) return null
          return (
            <section key={b.id} className="mb-8">
              <h2 className="mb-3 flex items-baseline gap-2 text-sm font-semibold text-muted">
                {b.name} <span className="text-xs font-normal">· {b.address} · פתוח {b.opensAt}–{b.closesAt}</span>
              </h2>
              <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                {list.map((r) => {
                  const mine = todays.filter((x) => x.roomId === r.id)
                  const current = mine.find((x) => new Date(x.startsAt).getTime() <= now && new Date(x.endsAt).getTime() > now)
                  const next = mine.find((x) => new Date(x.startsAt).getTime() > now)
                  return (
                    <article key={r.id} className={clsx('card overflow-hidden', !r.isActive && 'opacity-60')}>
                      <div className="h-1.5" style={{ background: r.color }} />
                      <div className="p-4">
                        <div className="flex items-start justify-between gap-2">
                          <div>
                            <h3 className="font-semibold">{r.name}</h3>
                            <p className="text-xs text-muted">{roomKindLabel[r.kind] ?? r.kind}</p>
                          </div>
                          {current ? <Badge tone="red" dot>תפוס</Badge> : <Badge tone="green" dot>פנוי</Badge>}
                        </div>
                        <div className="mt-3 grid grid-cols-3 gap-2 text-xs">
                          <span className="flex items-center gap-1 text-muted">
                            <Users className="h-3.5 w-3.5" /> {r.capacity}
                          </span>
                          <span className="flex items-center gap-1 text-muted">
                            <Clock className="h-3.5 w-3.5" /> {money(r.hourlyRate)}/ש׳
                          </span>
                          <span className="flex items-center gap-1 text-muted">
                            <Timer className="h-3.5 w-3.5" /> {r.bufferMinutes} דק׳
                          </span>
                        </div>
                        {r.equipment.length > 0 && (
                          <div className="mt-3 flex flex-wrap gap-1">
                            {r.equipment.map((e) => (
                              <span key={e} className="chip bg-subtle text-muted">
                                {equipmentLabel(e)}
                              </span>
                            ))}
                          </div>
                        )}
                        <div className="mt-3 rounded-xl bg-subtle px-3 py-2 text-xs">
                          {current ? (
                            <p>
                              <span className="font-medium">עכשיו:</span> {current.title} · עד {timeRange(current.startsAt, current.endsAt).split('–')[1]}
                            </p>
                          ) : next ? (
                            <p>
                              <span className="font-medium">הבא:</span> {next.title} · {timeRange(next.startsAt, next.endsAt)}
                            </p>
                          ) : (
                            <p className="text-muted">אין הזמנות נוספות היום</p>
                          )}
                        </div>
                        <div className="mt-3 flex gap-2">
                          <Link to={`/admin/calendar?view=week&rooms=${r.id}`} className="flex-1">
                            <Button variant="secondary" size="sm" className="w-full" icon={<CalendarDays className="h-3.5 w-3.5" />}>
                              יומן החדר
                            </Button>
                          </Link>
                          {can('rooms:manage') && (
                            <Button variant="ghost" size="sm" icon={<Pencil className="h-3.5 w-3.5" />} onClick={() => setEdit(r)}>
                              עריכה
                            </Button>
                          )}
                        </div>
                      </div>
                    </article>
                  )
                })}
              </div>
            </section>
          )
        })}
      <RoomDialog room={edit} onClose={() => setEdit(null)} />
    </div>
  )
}

function RoomDialog({ room, onClose }: { room: Partial<Room> | null; onClose: () => void }) {
  const qc = useQueryClient()
  const { buildings, equipment } = useLookups()
  const [f, setF] = useState<Partial<Room>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  useEffect(() => {
    setF(room ?? {})
    setErrors({})
  }, [room])
  const set = <K extends keyof Room>(k: K, v: Room[K]) => setF((p) => ({ ...p, [k]: v }))
  const isNew = !room?.id
  const eq = useMemo(() => new Set(f.equipment ?? []), [f.equipment])

  const save = useMutation({
    mutationFn: () => (isNew ? api.post<Room>('/rooms', f) : api.put<Room>(`/rooms/${room!.id}`, f)),
    onSuccess: () => {
      toast.success('החדר נשמר')
      qc.invalidateQueries({ queryKey: ['rooms'] })
      onClose()
    },
    onError: (e) => {
      if (e instanceof ApiError) setErrors(e.fields)
      toast.error(errorMessage(e))
    },
  })

  return (
    <Modal
      open={!!room}
      onClose={onClose}
      size="lg"
      title={isNew ? 'חדר חדש' : `עריכת ${room?.name}`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            ביטול
          </Button>
          <Button onClick={() => save.mutate()} loading={save.isPending}>
            שמירה
          </Button>
        </>
      }
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="שם החדר" error={errors.name}>
          <Input value={f.name ?? ''} onChange={(e) => set('name', e.target.value)} />
        </Field>
        <Field label="מבנה" error={errors.buildingId}>
          <Select value={f.buildingId ?? ''} onChange={(e) => set('buildingId', e.target.value)}>
            {buildings.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="סוג חלל">
          <Select value={f.kind ?? 'other'} onChange={(e) => set('kind', e.target.value)}>
            {Object.entries(roomKindLabel).map(([k, v]) => (
              <option key={k} value={k}>
                {v}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="קיבולת (משתתפים)" error={errors.capacity}>
          <Input type="number" min={0} value={f.capacity ?? 0} onChange={(e) => set('capacity', Number(e.target.value))} />
        </Field>
        <Field label="מחיר לשעה (₪)">
          <Input type="number" min={0} value={f.hourlyRate ?? 0} onChange={(e) => set('hourlyRate', Number(e.target.value))} />
        </Field>
        <Field label="מינימום שעות לשכירות">
          <Input type="number" min={0} step={0.5} value={f.minHours ?? 1} onChange={(e) => set('minHours', Number(e.target.value))} />
        </Field>
        <Field label="זמן חציצה אחרי הזמנה (דקות)" hint="ניקיון והתארגנות בין אירועים" error={errors.bufferMinutes}>
          <Input type="number" min={0} max={240} value={f.bufferMinutes ?? 15} onChange={(e) => set('bufferMinutes', Number(e.target.value))} />
        </Field>
        <Field label="סדר תצוגה">
          <Input type="number" value={f.sortOrder ?? 0} onChange={(e) => set('sortOrder', Number(e.target.value))} />
        </Field>
        <div className="sm:col-span-2">
          <span className="label">צבע ביומן</span>
          <div className="flex flex-wrap gap-2">
            {COLORS.map((c) => (
              <button key={c} type="button" onClick={() => set('color', c)} className={clsx('h-8 w-8 rounded-full ring-offset-2 ring-offset-surface transition', f.color === c && 'ring-2 ring-ink')} style={{ background: c }} aria-label={c} />
            ))}
          </div>
        </div>
        <div className="sm:col-span-2">
          <span className="label">ציוד זמין בחדר</span>
          <div className="flex flex-wrap gap-1.5">
            {equipment.map((e) => (
              <button
                key={e.code}
                type="button"
                onClick={() => set('equipment', eq.has(e.code) ? [...eq].filter((x) => x !== e.code) : [...eq, e.code])}
                className={clsx('rounded-xl border px-3 py-1.5 text-xs', eq.has(e.code) ? 'border-brand bg-brand-soft text-brand-strong' : 'border-line hover:bg-subtle')}
              >
                {e.label}
              </button>
            ))}
          </div>
        </div>
        <Field label="תיאור" className="sm:col-span-2">
          <Textarea value={f.description ?? ''} onChange={(e) => set('description', e.target.value)} rows={2} />
        </Field>
        <Field label="כללי שימוש" className="sm:col-span-2">
          <Textarea value={f.rules ?? ''} onChange={(e) => set('rules', e.target.value)} rows={2} />
        </Field>
        <div className="flex gap-6 sm:col-span-2">
          <Toggle checked={f.isActive ?? true} onChange={(v) => set('isActive', v)} label="פעיל" />
          <Toggle checked={f.isBookable ?? true} onChange={(v) => set('isBookable', v)} label="זמין להזמנה בפורטל" />
        </div>
        {!isNew && room?.id && (
          <div className="sm:col-span-2">
            <Attachments entity="room" entityId={room.id} />
          </div>
        )}
      </div>
    </Modal>
  )
}
