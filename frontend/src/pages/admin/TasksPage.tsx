import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { addDays, startOfDay } from 'date-fns'
import { BellRing, CalendarClock, CheckCircle2, Circle, Clock, Plus, Repeat, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from '@/components/toast'
import { Avatar, Badge, Button, Card, Empty, Field, IconButton, Input, Modal, PageHeader, PageLoader, Select, Tabs, Textarea, Toggle } from '@/components/ui'
import { api, errorMessage } from '@/lib/api'
import { dateShort, fmt, fromLocalInput, time, toLocalInput } from '@/lib/format'
import { staffKindLabel, taskKindLabel, taskStatusLabel, weekdayShort } from '@/lib/labels'
import { useLookups, useUsers } from '@/lib/queries'
import type { Task, TaskSchedule, TaskStatus } from '@/lib/types'

const statusTone: Record<TaskStatus, 'amber' | 'blue' | 'green' | 'gray'> = { open: 'amber', in_progress: 'blue', done: 'green', cancelled: 'gray' }

export function TasksPage() {
  const [tab, setTab] = useState<'tasks' | 'schedules'>('tasks')
  return (
    <div>
      <PageHeader
        title="משימות תפעול"
        subtitle="ניקיון, הקמות ותיקונים · שיבוץ לצוות · תזכורות אוטומטיות לפי לו״ז קבוע"
        actions={
          <Tabs
            value={tab}
            onChange={setTab}
            items={[
              { value: 'tasks', label: 'משימות' },
              { value: 'schedules', label: 'לו״ז קבוע' },
            ]}
          />
        }
      />
      {tab === 'tasks' ? <TaskBoard /> : <Schedules />}
    </div>
  )
}

function TaskBoard() {
  const qc = useQueryClient()
  const [range, setRange] = useState<'today' | 'week' | 'open'>('week')
  const [edit, setEdit] = useState<Partial<Task> | null>(null)
  const today = startOfDay(new Date())
  const query =
    range === 'today'
      ? { from: today.toISOString(), to: addDays(today, 1).toISOString() }
      : range === 'week'
        ? { from: addDays(today, -1).toISOString(), to: addDays(today, 7).toISOString() }
        : { statuses: ['open', 'in_progress'] }
  const { data = [], isLoading } = useQuery({ queryKey: ['tasks', range], queryFn: () => api.get<Task[]>('/tasks', query) })
  const status = useMutation({
    mutationFn: ({ id, s }: { id: string; s: TaskStatus }) => api.post(`/tasks/${id}/status`, { status: s }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['tasks'] }),
    onError: (e) => toast.error(errorMessage(e)),
  })

  const cols: { key: TaskStatus; label: string }[] = [
    { key: 'open', label: 'פתוחות' },
    { key: 'in_progress', label: 'בביצוע' },
    { key: 'done', label: 'בוצעו' },
  ]

  return (
    <>
      <div className="mb-4 flex flex-wrap items-center gap-3">
        <Tabs
          value={range}
          onChange={setRange}
          items={[
            { value: 'today', label: 'היום' },
            { value: 'week', label: 'השבוע' },
            { value: 'open', label: 'כל הפתוחות' },
          ]}
        />
        <Button className="ms-auto" icon={<Plus className="h-4 w-4" />} onClick={() => setEdit({ kind: 'cleaning', dueAt: new Date(Date.now() + 3600_000).toISOString() })}>
          משימה חדשה
        </Button>
      </div>
      {isLoading ? (
        <PageLoader />
      ) : (
        <div className="grid gap-4 lg:grid-cols-3">
          {cols.map((c) => {
            const list = data.filter((t) => t.status === c.key)
            return (
              <div key={c.key} className="rounded-2xl bg-subtle/70 p-3">
                <p className="mb-3 flex items-center justify-between px-1 text-sm font-semibold">
                  {c.label} <Badge tone={statusTone[c.key]}>{list.length}</Badge>
                </p>
                <div className="space-y-2">
                  {list.length === 0 && <p className="px-1 py-6 text-center text-xs text-muted">אין משימות</p>}
                  {list.map((t) => (
                    <div key={t.id} className="card p-3">
                      <div className="flex items-start gap-2">
                        <button
                          className="mt-0.5 text-muted hover:text-ok"
                          onClick={() => status.mutate({ id: t.id, s: t.status === 'done' ? 'open' : 'done' })}
                          aria-label="סימון בוצע"
                        >
                          {t.status === 'done' ? <CheckCircle2 className="h-5 w-5 text-ok" /> : <Circle className="h-5 w-5" />}
                        </button>
                        <button className="min-w-0 flex-1 text-start" onClick={() => setEdit(t)}>
                          <p className={clsx('text-sm font-medium', t.status === 'done' && 'text-muted line-through')}>{t.title}</p>
                          <p className="mt-0.5 flex flex-wrap items-center gap-x-2 text-xs text-muted">
                            <span className="flex items-center gap-1">
                              <Clock className="h-3 w-3" /> {dateShort(t.dueAt)} {time(t.dueAt)}
                            </span>
                            {t.roomName && <span>· {t.roomName}</span>}
                            {t.scheduleId && <Repeat className="h-3 w-3" />}
                            {t.reminderSentAt && <BellRing className="h-3 w-3 text-brand" />}
                          </p>
                          {t.completionNote && <p className="mt-1 rounded-lg bg-subtle px-2 py-1 text-xs">“{t.completionNote}”</p>}
                        </button>
                        {t.assigneeName && <Avatar name={t.assigneeName} size={26} />}
                      </div>
                      <div className="mt-2 flex items-center justify-between">
                        <Badge>{taskKindLabel[t.kind] ?? t.kind}</Badge>
                        {t.status === 'open' && (
                          <button className="text-xs text-brand hover:underline" onClick={() => status.mutate({ id: t.id, s: 'in_progress' })}>
                            התחלה ←
                          </button>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )
          })}
        </div>
      )}
      <TaskDialog task={edit} onClose={() => setEdit(null)} />
    </>
  )
}

function TaskDialog({ task, onClose }: { task: Partial<Task> | null; onClose: () => void }) {
  const qc = useQueryClient()
  const staff = useUsers('staff').data ?? []
  const { rooms } = useLookups()
  const [f, setF] = useState<Partial<Task>>({})
  useEffect(() => setF(task ?? {}), [task])
  const isNew = !task?.id
  const save = useMutation({
    mutationFn: () => (isNew ? api.post('/tasks', f) : api.put(`/tasks/${task!.id}`, f)),
    onSuccess: () => {
      toast.success(isNew ? 'המשימה נוצרה והעובד/ת קיבלו התראה' : 'המשימה עודכנה')
      qc.invalidateQueries({ queryKey: ['tasks'] })
      onClose()
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  const del = useMutation({
    mutationFn: () => api.del(`/tasks/${task!.id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['tasks'] })
      onClose()
    },
  })
  return (
    <Modal
      open={!!task}
      onClose={onClose}
      title={isNew ? 'משימה חדשה' : 'עריכת משימה'}
      footer={
        <>
          {!isNew && (
            <Button variant="ghost" className="me-auto text-danger" icon={<Trash2 className="h-4 w-4" />} onClick={() => del.mutate()}>
              מחיקה
            </Button>
          )}
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
        <Field label="כותרת" className="sm:col-span-2">
          <Input value={f.title ?? ''} onChange={(e) => setF({ ...f, title: e.target.value })} />
        </Field>
        <Field label="סוג">
          <Select value={f.kind ?? 'cleaning'} onChange={(e) => setF({ ...f, kind: e.target.value })}>
            {Object.entries(taskKindLabel).map(([k, v]) => (
              <option key={k} value={k}>
                {v}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="מועד">
          <Input type="datetime-local" value={f.dueAt ? toLocalInput(f.dueAt) : ''} onChange={(e) => setF({ ...f, dueAt: fromLocalInput(e.target.value) })} />
        </Field>
        <Field label="שיבוץ לעובד/ת">
          <Select value={f.assigneeId ?? ''} onChange={(e) => setF({ ...f, assigneeId: e.target.value || null })}>
            <option value="">לא משובץ</option>
            {staff.map((s) => (
              <option key={s.id} value={s.id}>
                {s.fullName} {s.staffKind ? `(${staffKindLabel[s.staffKind]})` : ''}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="חדר">
          <Select value={f.roomId ?? ''} onChange={(e) => setF({ ...f, roomId: e.target.value || null })}>
            <option value="">ללא</option>
            {rooms.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
          </Select>
        </Field>
        {!isNew && (
          <Field label="סטטוס">
            <Select value={f.status ?? 'open'} onChange={(e) => setF({ ...f, status: e.target.value as TaskStatus })}>
              {Object.entries(taskStatusLabel).map(([k, v]) => (
                <option key={k} value={k}>
                  {v}
                </option>
              ))}
            </Select>
          </Field>
        )}
        <Field label="תיאור" className="sm:col-span-2">
          <Textarea value={f.description ?? ''} onChange={(e) => setF({ ...f, description: e.target.value })} />
        </Field>
      </div>
    </Modal>
  )
}

function Schedules() {
  const qc = useQueryClient()
  const staff = useUsers('staff').data ?? []
  const { rooms, roomById } = useLookups()
  const { data = [], isLoading } = useQuery({ queryKey: ['task-schedules'], queryFn: () => api.get<TaskSchedule[]>('/task-schedules') })
  const [edit, setEdit] = useState<Partial<TaskSchedule> | null>(null)
  const save = useMutation({
    mutationFn: (s: Partial<TaskSchedule>) => (s.id ? api.put(`/task-schedules/${s.id}`, s) : api.post('/task-schedules', s)),
    onSuccess: () => {
      toast.success('הלו״ז נשמר – משימות לשבוע הקרוב נוצרו אוטומטית')
      qc.invalidateQueries({ queryKey: ['task-schedules'] })
      qc.invalidateQueries({ queryKey: ['tasks'] })
      setEdit(null)
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  const del = useMutation({
    mutationFn: (id: string) => api.del(`/task-schedules/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['task-schedules'] }),
  })
  if (isLoading) return <PageLoader />

  return (
    <Card
      title={
        <span className="flex items-center gap-2">
          <CalendarClock className="h-4 w-4" /> משימות חוזרות
        </span>
      }
      actions={
        <Button size="sm" icon={<Plus className="h-3.5 w-3.5" />} onClick={() => setEdit({ kind: 'cleaning', weekdays: [0, 1, 2, 3, 4], timeOfDay: '07:00', durationMinutes: 60, isActive: true })}>
          לו״ז חדש
        </Button>
      }
      padded={false}
    >
      <p className="border-b border-line px-5 py-3 text-xs text-muted">המערכת יוצרת משימות לשבוע הקרוב ושולחת תזכורת לעובד/ת לפני המועד (SMS / Telegram / אימייל לפי ההגדרה).</p>
      {data.length === 0 ? (
        <Empty title="אין לו״ז קבוע" />
      ) : (
        <ul className="divide-y divide-line">
          {data.map((s) => (
            <li key={s.id} className={clsx('flex flex-wrap items-center gap-4 px-5 py-3', !s.isActive && 'opacity-50')}>
              <span className="w-14 text-lg font-bold tabular-nums">{s.timeOfDay}</span>
              <span className="min-w-0 flex-1">
                <button className="block font-medium hover:text-brand" onClick={() => setEdit(s)}>
                  {s.title}
                </button>
                <span className="text-xs text-muted">
                  {s.roomId ? roomById.get(s.roomId)?.name : 'כללי'} · {staff.find((u) => u.id === s.assigneeId)?.fullName ?? 'לא משובץ'} · {s.durationMinutes} דק׳
                </span>
              </span>
              <span className="flex gap-1">
                {weekdayShort.map((d, i) => (
                  <span key={d} className={clsx('flex h-7 w-7 items-center justify-center rounded-full text-[11px]', s.weekdays.includes(i) ? 'bg-brand text-white dark:text-slate-900' : 'bg-subtle text-muted')}>
                    {d}
                  </span>
                ))}
              </span>
              <IconButton label="מחיקה" onClick={() => del.mutate(s.id)}>
                <Trash2 className="h-4 w-4" />
              </IconButton>
            </li>
          ))}
        </ul>
      )}
      <Modal
        open={!!edit}
        onClose={() => setEdit(null)}
        title={edit?.id ? 'עריכת לו״ז' : 'לו״ז חדש'}
        footer={
          <>
            <Button variant="secondary" onClick={() => setEdit(null)}>
              ביטול
            </Button>
            <Button onClick={() => edit && save.mutate(edit)} loading={save.isPending}>
              שמירה
            </Button>
          </>
        }
      >
        {edit && (
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="כותרת" className="sm:col-span-2">
              <Input value={edit.title ?? ''} onChange={(e) => setEdit({ ...edit, title: e.target.value })} />
            </Field>
            <Field label="שעה">
              <Input type="time" value={edit.timeOfDay ?? ''} onChange={(e) => setEdit({ ...edit, timeOfDay: e.target.value })} />
            </Field>
            <Field label="משך (דקות)">
              <Input type="number" min={5} value={edit.durationMinutes ?? 60} onChange={(e) => setEdit({ ...edit, durationMinutes: Number(e.target.value) })} />
            </Field>
            <Field label="עובד/ת">
              <Select value={edit.assigneeId ?? ''} onChange={(e) => setEdit({ ...edit, assigneeId: e.target.value || null })}>
                <option value="">לא משובץ</option>
                {staff.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.fullName}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="חדר">
              <Select value={edit.roomId ?? ''} onChange={(e) => setEdit({ ...edit, roomId: e.target.value || null })}>
                <option value="">כללי</option>
                {rooms.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="סוג">
              <Select value={edit.kind ?? 'cleaning'} onChange={(e) => setEdit({ ...edit, kind: e.target.value })}>
                {Object.entries(taskKindLabel).map(([k, v]) => (
                  <option key={k} value={k}>
                    {v}
                  </option>
                ))}
              </Select>
            </Field>
            <div className="sm:col-span-2">
              <span className="label">ימים</span>
              <div className="flex gap-1.5">
                {weekdayShort.map((d, i) => {
                  const on = edit.weekdays?.includes(i)
                  return (
                    <button
                      key={d}
                      type="button"
                      onClick={() => setEdit({ ...edit, weekdays: on ? edit.weekdays!.filter((x) => x !== i) : [...(edit.weekdays ?? []), i].sort() })}
                      className={clsx('h-9 w-9 rounded-full text-sm font-medium', on ? 'bg-brand text-white dark:text-slate-900' : 'bg-subtle text-muted hover:bg-line')}
                    >
                      {d}
                    </button>
                  )
                })}
              </div>
            </div>
            <Toggle checked={edit.isActive ?? true} onChange={(v) => setEdit({ ...edit, isActive: v })} label="פעיל" />
            <p className="text-xs text-muted sm:col-span-2">יום הבא: {fmt(addDays(new Date(), 1), 'EEEE d.M')}</p>
          </div>
        )}
      </Modal>
    </Card>
  )
}
