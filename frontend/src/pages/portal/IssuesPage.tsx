import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Hammer, MessageSquareWarning, Package, Plus, Sparkles, Wrench } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { PhotoGallery, PhotoPicker, uploadPhotos } from '@/components/Photos'
import { toast } from '@/components/toast'
import { Badge, Button, Empty, Field, Input, Modal, PageHeader, PageLoader, Select, Textarea } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { relative } from '@/lib/format'
import { issueKinds, taskKindLabel, taskStatusLabel } from '@/lib/labels'
import { useLookups } from '@/lib/queries'
import type { Task, TaskStatus } from '@/lib/types'

const kindIcon: Record<string, ReactNode> = {
  repair: <Hammer className="h-5 w-5" />,
  supplies: <Package className="h-5 w-5" />,
  cleaning: <Sparkles className="h-5 w-5" />,
  other: <MessageSquareWarning className="h-5 w-5" />,
}
const statusTone: Record<TaskStatus, 'amber' | 'blue' | 'green' | 'gray'> = { open: 'amber', in_progress: 'blue', done: 'green', cancelled: 'gray' }

/** Coordinator tickets to the operations team: broken items, supplies, cleaning. */
export function IssuesPage() {
  const [open, setOpen] = useState(false)
  const { data = [], isLoading } = useQuery({ queryKey: ['tasks', 'issues'], queryFn: () => api.get<Task[]>('/tasks') })
  const tickets = [...data].sort((a, b) => b.createdAt.localeCompare(a.createdAt))

  return (
    <div>
      <PageHeader
        title="פניות לצוות התפעול"
        subtitle="תקלה, ציוד מתכלה או ניקיון – עם תמונה. תקבלו עדכון כשהפנייה טופלה."
        actions={
          <Button icon={<Plus className="h-4 w-4" />} onClick={() => setOpen(true)}>
            פנייה חדשה
          </Button>
        }
      />
      {isLoading ? (
        <PageLoader />
      ) : tickets.length === 0 ? (
        <div className="card">
          <Empty icon={<Wrench className="h-6 w-6" />} title="אין פניות עדיין" hint="משהו שבור? חסר טונר? צריך ניקיון? פתחו פנייה והצוות יטפל." />
        </div>
      ) : (
        <div className="grid gap-3 lg:grid-cols-2">
          {tickets.map((t) => (
            <article key={t.id} className="card p-4">
              <div className="flex items-start gap-3">
                <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-subtle text-muted">{kindIcon[t.kind] ?? kindIcon.other}</span>
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="font-semibold">{t.title}</p>
                    <Badge tone={statusTone[t.status]}>{t.status === 'open' && !t.assigneeId ? 'ממתינה לשיבוץ' : taskStatusLabel[t.status]}</Badge>
                  </div>
                  <p className="mt-0.5 text-xs text-muted">
                    {taskKindLabel[t.kind] ?? t.kind}
                    {t.roomName && ` · ${t.roomName}`} · נפתחה {relative(t.createdAt)}
                    {t.assigneeName && ` · מטפל/ת: ${t.assigneeName}`}
                  </p>
                  {t.description && <p className="mt-2 whitespace-pre-line text-sm">{t.description}</p>}
                  {t.status === 'done' && (
                    <p className="mt-2 rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-800 dark:bg-emerald-950/30 dark:text-emerald-200">
                      טופל {t.completedAt && relative(t.completedAt)}
                      {t.completionNote && ` · ${t.completionNote}`}
                    </p>
                  )}
                  <div className="mt-3 space-y-2">
                    <PhotoGallery entity="task" entityId={t.id} />
                    {t.status === 'done' && <PhotoGallery entity="task_done" entityId={t.id} title="תמונה לאחר הטיפול" />}
                  </div>
                </div>
              </div>
            </article>
          ))}
        </div>
      )}
      <IssueDialog open={open} onClose={() => setOpen(false)} />
    </div>
  )
}

function IssueDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const qc = useQueryClient()
  const { rooms } = useLookups()
  const blank = { kind: '', roomId: '', title: '', description: '' }
  const [f, setF] = useState(blank)
  const [photos, setPhotos] = useState<File[]>([])
  const [errors, setErrors] = useState<Record<string, string>>({})
  const set = <K extends keyof typeof f>(k: K, v: (typeof f)[K]) => setF((p) => ({ ...p, [k]: v }))
  const kind = issueKinds.find((k) => k.value === f.kind)

  const close = () => {
    setF(blank)
    setPhotos([])
    setErrors({})
    onClose()
  }
  const send = useMutation({
    mutationFn: async () => {
      const t = await api.post<Task>('/tasks/issues', { kind: f.kind, roomId: f.roomId || null, title: f.title, description: f.description })
      if (photos.length) {
        try {
          await uploadPhotos('task', t.id, photos)
        } catch (e) {
          toast.error(`הפנייה נשלחה, אך העלאת התמונה נכשלה: ${errorMessage(e)}`)
        }
      }
      return t
    },
    onSuccess: () => {
      toast.success('הפנייה נשלחה לצוות התפעול')
      qc.invalidateQueries({ queryKey: ['tasks'] })
      close()
    },
    onError: (e) => {
      if (e instanceof ApiError) setErrors(e.fields)
      toast.error(errorMessage(e))
    },
  })

  return (
    <Modal
      open={open}
      onClose={close}
      title="פנייה חדשה לצוות התפעול"
      footer={
        <>
          <Button variant="secondary" onClick={close}>
            ביטול
          </Button>
          <Button disabled={!f.kind} loading={send.isPending} onClick={() => send.mutate()}>
            שליחה
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div>
          <span className="label">מה צריך? *</span>
          <div className="grid grid-cols-2 gap-2">
            {issueKinds.map((k) => (
              <button
                key={k.value}
                type="button"
                onClick={() => set('kind', k.value)}
                className={clsx(
                  'flex min-h-16 flex-col items-start gap-1 rounded-xl border p-3 text-start transition',
                  f.kind === k.value ? 'border-brand bg-brand-soft ring-1 ring-brand/40' : 'border-line hover:bg-subtle',
                )}
              >
                <span className={clsx('flex items-center gap-2 text-sm font-semibold', f.kind === k.value && 'text-brand-strong')}>
                  {kindIcon[k.value]} {k.label}
                </span>
                <span className="text-[11px] leading-tight text-muted">{k.hint}</span>
              </button>
            ))}
          </div>
          {errors.kind && <p className="mt-1 text-xs text-danger">{errors.kind}</p>}
        </div>
        <Field label="איפה?" hint="לא חובה">
          <Select value={f.roomId} onChange={(e) => set('roomId', e.target.value)}>
            <option value="">— לא בחדר מסוים —</option>
            {rooms.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="כותרת קצרה" error={errors.title}>
          <Input value={f.title} maxLength={120} onChange={(e) => set('title', e.target.value)} placeholder={kind?.hint ?? 'לדוגמה: כיסא שבור בחדר החוגים'} />
        </Field>
        <Field label="פירוט" error={errors.description}>
          <Textarea value={f.description} onChange={(e) => set('description', e.target.value)} placeholder="מה בדיוק קרה / מה חסר? ככל שיותר מדויק – הטיפול מהיר יותר" />
        </Field>
        <div>
          <span className="label">תמונה</span>
          <PhotoPicker files={photos} onChange={setPhotos} />
        </div>
      </div>
    </Modal>
  )
}
