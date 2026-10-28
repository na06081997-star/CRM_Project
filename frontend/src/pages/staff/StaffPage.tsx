import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { addDays, isToday, isTomorrow, startOfDay } from 'date-fns'
import { BellRing, Check, CheckCircle2, Clock, DoorOpen, LayoutDashboard, PartyPopper, Play, RotateCcw } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { SimpleHeader } from '@/components/layout/SimpleHeader'
import { toast } from '@/components/toast'
import { Badge, Button, Empty, Modal, PageLoader, Tabs, Textarea } from '@/components/ui'
import { api, errorMessage } from '@/lib/api'
import { useCan, useUser } from '@/lib/auth'
import { fmt, relative, time } from '@/lib/format'
import { taskKindLabel } from '@/lib/labels'
import { useSocket } from '@/lib/realtime'
import type { Task, TaskStatus } from '@/lib/types'

/** Mobile-first app for cleaners / technicians: today's tasks and "בוצע". */
export function StaffPage() {
  const qc = useQueryClient()
  const user = useUser()
  const can = useCan()
  const [range, setRange] = useState<'today' | 'week' | 'done'>('today')
  const [finishing, setFinishing] = useState<Task | null>(null)
  const [note, setNote] = useState('')
  useSocket('/ws', (ev) => {
    if (ev.type === 'task.changed') qc.invalidateQueries({ queryKey: ['tasks'] })
    if (ev.type === 'notification') {
      qc.invalidateQueries({ queryKey: ['notifications'] })
      const d = ev.data as { subject?: string }
      if (d?.subject) toast.info(d.subject)
    }
  })

  const today = startOfDay(new Date())
  const query =
    range === 'done'
      ? { statuses: ['done'], from: addDays(today, -7).toISOString() }
      : { statuses: ['open', 'in_progress'], to: addDays(today, range === 'today' ? 1 : 7).toISOString() }
  const { data = [], isLoading } = useQuery({
    queryKey: ['tasks', 'mine', range],
    queryFn: () => api.get<Task[]>('/tasks', { ...query, assigneeId: can('tasks:manage') ? user?.id : undefined }),
    refetchInterval: 60_000,
  })

  const setStatus = useMutation({
    mutationFn: ({ id, status, note }: { id: string; status: TaskStatus; note?: string }) => api.post<Task>(`/tasks/${id}/status`, { status, note }),
    onSuccess: (t) => {
      if (t.status === 'done') toast.success('כל הכבוד! המשימה סומנה כבוצעה ✓')
      qc.invalidateQueries({ queryKey: ['tasks'] })
      setFinishing(null)
      setNote('')
    },
    onError: (e) => toast.error(errorMessage(e)),
  })

  const groups = new Map<string, Task[]>()
  for (const t of data) {
    const d = new Date(t.dueAt)
    const key = d < today ? 'באיחור' : isToday(d) ? 'היום' : isTomorrow(d) ? 'מחר' : fmt(d, 'EEEE d.M')
    groups.set(key, [...(groups.get(key) ?? []), t])
  }

  return (
    <div className="min-h-full pb-16">
      <SimpleHeader subtitle="צוות תפעול">
        {can('dashboard:view') && (
          <Link to="/admin">
            <Button size="sm" variant="secondary" icon={<LayoutDashboard className="h-4 w-4" />}>
              ניהול
            </Button>
          </Link>
        )}
      </SimpleHeader>
      <div className="mx-auto max-w-xl px-4 pt-6">
        <h1 className="text-2xl font-bold">שלום {user?.fullName.split(' ')[0]}</h1>
        <p className="text-sm text-muted">{fmt(new Date(), 'EEEE, d בMMMM')}</p>
        <Tabs
          className="mt-4 w-full [&>button]:flex-1"
          value={range}
          onChange={setRange}
          items={[
            { value: 'today', label: 'היום' },
            { value: 'week', label: 'השבוע' },
            { value: 'done', label: 'בוצעו' },
          ]}
        />

        {isLoading ? (
          <PageLoader />
        ) : data.length === 0 ? (
          <div className="card mt-6">
            <Empty icon={<PartyPopper className="h-6 w-6" />} title={range === 'done' ? 'עוד לא סומנו משימות' : 'אין משימות פתוחות 🎉'} hint="משימות חדשות יופיעו כאן אוטומטית, ותקבלו תזכורת לפני כל משימה." />
          </div>
        ) : (
          [...groups.entries()].map(([label, list]) => (
            <section key={label} className="mt-6">
              <h2 className={clsx('mb-2 text-sm font-semibold', label === 'באיחור' ? 'text-danger' : 'text-muted')}>{label}</h2>
              <div className="space-y-3">
                {list.map((t) => (
                  <article key={t.id} className={clsx('card p-4', t.status === 'in_progress' && 'border-brand/50 ring-1 ring-brand/20')}>
                    <div className="flex items-start gap-3">
                      <div className="flex w-14 shrink-0 flex-col items-center rounded-xl bg-subtle py-2">
                        <Clock className="h-4 w-4 text-muted" />
                        <span className="mt-0.5 text-sm font-bold tabular-nums">{time(t.dueAt)}</span>
                      </div>
                      <div className="min-w-0 flex-1">
                        <p className={clsx('font-semibold', t.status === 'done' && 'text-muted line-through')}>{t.title}</p>
                        <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted">
                          <Badge>{taskKindLabel[t.kind] ?? t.kind}</Badge>
                          {t.roomName && (
                            <span className="flex items-center gap-1">
                              <DoorOpen className="h-3.5 w-3.5" /> {t.roomName}
                            </span>
                          )}
                          {t.reminderSentAt && (
                            <span className="flex items-center gap-1">
                              <BellRing className="h-3.5 w-3.5" /> תזכורת נשלחה
                            </span>
                          )}
                        </div>
                        {t.description && <p className="mt-2 text-sm text-muted">{t.description}</p>}
                        {t.status === 'done' && t.completedAt && <p className="mt-2 text-xs text-ok">בוצע {relative(t.completedAt)}</p>}
                      </div>
                    </div>
                    <div className="mt-4 flex gap-2">
                      {t.status === 'open' && (
                        <Button variant="secondary" className="flex-1" icon={<Play className="h-4 w-4" />} onClick={() => setStatus.mutate({ id: t.id, status: 'in_progress' })}>
                          התחלתי
                        </Button>
                      )}
                      {t.status !== 'done' ? (
                        <Button size="lg" className="flex-[2]" icon={<Check className="h-5 w-5" />} onClick={() => setFinishing(t)}>
                          בוצע
                        </Button>
                      ) : (
                        <Button variant="ghost" size="sm" icon={<RotateCcw className="h-4 w-4" />} onClick={() => setStatus.mutate({ id: t.id, status: 'open' })}>
                          החזרה לפתוחות
                        </Button>
                      )}
                    </div>
                  </article>
                ))}
              </div>
            </section>
          ))
        )}
      </div>

      <Modal
        open={!!finishing}
        onClose={() => setFinishing(null)}
        title="סימון משימה כבוצעה"
        size="sm"
        footer={
          <Button size="lg" className="w-full" icon={<CheckCircle2 className="h-5 w-5" />} loading={setStatus.isPending} onClick={() => finishing && setStatus.mutate({ id: finishing.id, status: 'done', note })}>
            אישור – בוצע
          </Button>
        }
      >
        <p className="mb-3 font-medium">{finishing?.title}</p>
        <Textarea value={note} onChange={(e) => setNote(e.target.value)} placeholder="הערה למנהל (לא חובה) – לדוגמה: חסר נייר טואלט, נורה נוספת דורשת החלפה…" rows={3} />
      </Modal>
    </div>
  )
}
