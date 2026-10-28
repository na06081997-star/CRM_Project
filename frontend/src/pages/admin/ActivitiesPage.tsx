import clsx from 'clsx'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { CalendarDays, Pencil, Plus, Sparkles } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { toast } from '@/components/toast'
import { Badge, Button, Empty, Field, Input, Modal, PageHeader, PageLoader, Select, Textarea, Toggle } from '@/components/ui'
import { api, errorMessage } from '@/lib/api'
import { useActivities, useLookups, useUsers } from '@/lib/queries'
import type { Activity } from '@/lib/types'

const COLORS = ['#0f766e', '#db2777', '#9333ea', '#16a34a', '#ea580c', '#65a30d', '#ca8a04', '#0369a1']

export function ActivitiesPage() {
  const { data, isLoading } = useActivities()
  const coordinators = useUsers('coordinator').data ?? []
  const { roomById } = useLookups()
  const [edit, setEdit] = useState<Partial<Activity> | null>(null)
  if (isLoading) return <PageLoader />
  const list = data ?? []

  return (
    <div>
      <PageHeader
        title="פעילויות וחוגים"
        subtitle="תוכניות קבועות המופעלות ע״י רכזים – כל מפגש מופיע ביומן"
        actions={
          <Button icon={<Plus className="h-4 w-4" />} onClick={() => setEdit({ color: COLORS[0], isActive: true })}>
            פעילות חדשה
          </Button>
        }
      />
      {list.length === 0 ? (
        <div className="card">
          <Empty icon={<Sparkles className="h-6 w-6" />} title="עדיין אין פעילויות" />
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {list.map((a) => {
            const coord = coordinators.find((c) => c.id === a.coordinatorId)
            return (
              <article key={a.id} className={clsx('card p-4', !a.isActive && 'opacity-60')}>
                <div className="flex items-start gap-3">
                  <span className="mt-1 h-10 w-1.5 rounded-full" style={{ background: a.color }} />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center justify-between gap-2">
                      <h3 className="truncate font-semibold">{a.name}</h3>
                      {!a.isActive && <Badge>לא פעיל</Badge>}
                    </div>
                    <p className="text-xs text-muted">
                      {coord?.fullName ?? 'ללא רכז'} · {a.defaultRoomId ? roomById.get(a.defaultRoomId)?.name : 'ללא חדר קבוע'}
                    </p>
                    {a.description && <p className="mt-2 line-clamp-2 text-sm text-muted">{a.description}</p>}
                    <div className="mt-3 flex items-center justify-between">
                      <span className="text-xs text-muted">{a.sessionsCount} מפגשים ביומן</span>
                      <div className="flex gap-1">
                        <Link to={`/admin/calendar?view=month${a.defaultRoomId ? `&rooms=${a.defaultRoomId}` : ''}`}>
                          <Button size="sm" variant="ghost" icon={<CalendarDays className="h-3.5 w-3.5" />}>
                            יומן
                          </Button>
                        </Link>
                        <Button size="sm" variant="ghost" icon={<Pencil className="h-3.5 w-3.5" />} onClick={() => setEdit(a)}>
                          עריכה
                        </Button>
                      </div>
                    </div>
                  </div>
                </div>
              </article>
            )
          })}
        </div>
      )}
      <ActivityDialog activity={edit} onClose={() => setEdit(null)} />
    </div>
  )
}

function ActivityDialog({ activity, onClose }: { activity: Partial<Activity> | null; onClose: () => void }) {
  const qc = useQueryClient()
  const coordinators = useUsers('coordinator').data ?? []
  const { rooms } = useLookups()
  const [f, setF] = useState<Partial<Activity>>({})
  useEffect(() => setF(activity ?? {}), [activity])
  const isNew = !activity?.id
  const save = useMutation({
    mutationFn: () => (isNew ? api.post('/activities', f) : api.put(`/activities/${activity!.id}`, f)),
    onSuccess: () => {
      toast.success('הפעילות נשמרה')
      qc.invalidateQueries({ queryKey: ['activities'] })
      onClose()
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  return (
    <Modal
      open={!!activity}
      onClose={onClose}
      title={isNew ? 'פעילות חדשה' : 'עריכת פעילות'}
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
        <Field label="שם הפעילות" className="sm:col-span-2">
          <Input value={f.name ?? ''} onChange={(e) => setF({ ...f, name: e.target.value })} />
        </Field>
        <Field label="רכז/ת">
          <Select value={f.coordinatorId ?? ''} onChange={(e) => setF({ ...f, coordinatorId: e.target.value || null })}>
            <option value="">ללא</option>
            {coordinators.map((c) => (
              <option key={c.id} value={c.id}>
                {c.fullName}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="חדר קבוע">
          <Select value={f.defaultRoomId ?? ''} onChange={(e) => setF({ ...f, defaultRoomId: e.target.value || null })}>
            <option value="">ללא</option>
            {rooms.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="קהל יעד">
          <Input value={f.audience ?? ''} onChange={(e) => setF({ ...f, audience: e.target.value })} placeholder="ילדים 6–9, נוער, גיל הזהב…" />
        </Field>
        <div>
          <span className="label">צבע</span>
          <div className="flex gap-2">
            {COLORS.map((c) => (
              <button key={c} type="button" onClick={() => setF({ ...f, color: c })} className={clsx('h-7 w-7 rounded-full ring-offset-2 ring-offset-surface', f.color === c && 'ring-2 ring-ink')} style={{ background: c }} aria-label={c} />
            ))}
          </div>
        </div>
        <Field label="תיאור" className="sm:col-span-2">
          <Textarea value={f.description ?? ''} onChange={(e) => setF({ ...f, description: e.target.value })} />
        </Field>
        <Toggle checked={f.isActive ?? true} onChange={(v) => setF({ ...f, isActive: v })} label="פעילות פעילה" />
      </div>
      <p className="mt-4 text-xs text-muted">ליצירת מפגשים קבועים: פתחו הזמנה חדשה ביומן, בחרו את הפעילות והגדירו “חזרה שבועית”.</p>
    </Modal>
  )
}
