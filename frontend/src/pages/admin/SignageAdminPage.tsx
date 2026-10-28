import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ExternalLink, ImagePlus, Megaphone, Monitor, Pencil, Plus, Trash2 } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { toast } from '@/components/toast'
import { Badge, Button, Card, Empty, Field, IconButton, Input, Modal, PageHeader, PageLoader, Select, Textarea, Toggle } from '@/components/ui'
import { api, errorMessage, publicFileUrl } from '@/lib/api'
import { dateShort, fromLocalInput, toLocalInput } from '@/lib/format'
import { useLookups } from '@/lib/queries'
import type { Announcement, Attachment, SignageScreen } from '@/lib/types'

export function SignageAdminPage() {
  const qc = useQueryClient()
  const { buildings, rooms } = useLookups()
  const screens = useQuery({ queryKey: ['screens'], queryFn: () => api.get<SignageScreen[]>('/signage/screens') })
  const anns = useQuery({ queryKey: ['announcements'], queryFn: () => api.get<Announcement[]>('/signage/announcements') })
  const [screen, setScreen] = useState<Partial<SignageScreen> | null>(null)
  const [ann, setAnn] = useState<Partial<Announcement> | null>(null)

  const saveScreen = useMutation({
    mutationFn: (s: Partial<SignageScreen>) => (s.id ? api.put(`/signage/screens/${s.id}`, s) : api.post('/signage/screens', s)),
    onSuccess: () => {
      toast.success('המסך נשמר – התצוגות התעדכנו')
      qc.invalidateQueries({ queryKey: ['screens'] })
      setScreen(null)
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  const delAnn = useMutation({
    mutationFn: (id: string) => api.del(`/signage/announcements/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['announcements'] }),
  })
  if (screens.isLoading) return <PageLoader />
  const now = Date.now()

  return (
    <div className="space-y-6">
      <PageHeader title="שילוט דיגיטלי" subtitle="מסכי לובי המציגים בזמן אמת מה קורה עכשיו בכל חדר, אירועים קרובים והודעות" />

      <Card
        title={
          <span className="flex items-center gap-2">
            <Monitor className="h-4 w-4" /> מסכים
          </span>
        }
        actions={
          <Button size="sm" icon={<Plus className="h-3.5 w-3.5" />} onClick={() => setScreen({ theme: 'dark', rotateSeconds: 12, isActive: true, roomIds: [] })}>
            מסך חדש
          </Button>
        }
      >
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {(screens.data ?? []).map((s) => (
            <div key={s.id} className="overflow-hidden rounded-2xl border border-line">
              <div className={clsx('relative aspect-video overflow-hidden', s.theme === 'dark' ? 'bg-slate-950' : 'bg-slate-100')}>
                <iframe src={`/signage/${s.slug}?preview=1`} title={s.name} className="pointer-events-none absolute left-0 top-0 h-[400%] w-[400%] origin-top-left border-0" style={{ transform: 'scale(0.25)' }} />
              </div>
              <div className="flex items-center gap-2 p-3">
                <div className="min-w-0 flex-1">
                  <p className="font-medium">{s.name}</p>
                  <p className="text-xs text-muted" dir="ltr">
                    /signage/{s.slug}
                  </p>
                </div>
                {!s.isActive && <Badge tone="gray">כבוי</Badge>}
                <a href={`/signage/${s.slug}`} target="_blank" rel="noreferrer">
                  <IconButton label="פתיחה במסך מלא">
                    <ExternalLink className="h-4 w-4" />
                  </IconButton>
                </a>
                <IconButton label="עריכה" onClick={() => setScreen(s)}>
                  <Pencil className="h-4 w-4" />
                </IconButton>
              </div>
            </div>
          ))}
        </div>
      </Card>

      <Card
        title={
          <span className="flex items-center gap-2">
            <Megaphone className="h-4 w-4" /> הודעות ועדכונים
          </span>
        }
        actions={
          <Button size="sm" icon={<Plus className="h-3.5 w-3.5" />} onClick={() => setAnn({ priority: 0, startsAt: new Date().toISOString() })}>
            הודעה חדשה
          </Button>
        }
        padded={false}
      >
        {(anns.data ?? []).length === 0 ? (
          <Empty title="אין הודעות" />
        ) : (
          <ul className="divide-y divide-line">
            {(anns.data ?? []).map((a) => {
              const live = new Date(a.startsAt).getTime() <= now && (!a.endsAt || new Date(a.endsAt).getTime() > now)
              return (
                <li key={a.id} className="flex items-center gap-4 px-5 py-3">
                  {a.imageKey ? <img src={publicFileUrl(a.imageKey)} alt="" className="h-12 w-16 rounded-lg object-cover" /> : <div className="flex h-12 w-16 items-center justify-center rounded-lg bg-subtle text-muted"><Megaphone className="h-5 w-5" /></div>}
                  <div className="min-w-0 flex-1">
                    <p className="font-medium">{a.title}</p>
                    <p className="truncate text-xs text-muted">{a.body}</p>
                  </div>
                  <span className="text-xs text-muted">
                    {dateShort(a.startsAt)}
                    {a.endsAt ? ` – ${dateShort(a.endsAt)}` : ''}
                  </span>
                  <Badge tone={live ? 'green' : 'gray'}>{live ? 'מוצג' : 'לא פעיל'}</Badge>
                  <IconButton label="עריכה" onClick={() => setAnn(a)}>
                    <Pencil className="h-4 w-4" />
                  </IconButton>
                  <IconButton label="מחיקה" onClick={() => delAnn.mutate(a.id)}>
                    <Trash2 className="h-4 w-4" />
                  </IconButton>
                </li>
              )
            })}
          </ul>
        )}
      </Card>

      <Modal
        open={!!screen}
        onClose={() => setScreen(null)}
        title={screen?.id ? 'עריכת מסך' : 'מסך חדש'}
        footer={
          <>
            <Button variant="secondary" onClick={() => setScreen(null)}>
              ביטול
            </Button>
            <Button onClick={() => screen && saveScreen.mutate(screen)} loading={saveScreen.isPending}>
              שמירה
            </Button>
          </>
        }
      >
        {screen && (
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="שם המסך">
              <Input value={screen.name ?? ''} onChange={(e) => setScreen({ ...screen, name: e.target.value })} />
            </Field>
            <Field label="מזהה בכתובת (slug)" hint="אותיות לטיניות קטנות ומקפים">
              <Input dir="ltr" value={screen.slug ?? ''} onChange={(e) => setScreen({ ...screen, slug: e.target.value })} placeholder="lobby" />
            </Field>
            <Field label="מבנה">
              <Select value={screen.buildingId ?? ''} onChange={(e) => setScreen({ ...screen, buildingId: e.target.value || null, roomIds: [] })}>
                <option value="">כל המבנים</option>
                {buildings.map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="ערכת צבעים">
              <Select value={screen.theme ?? 'dark'} onChange={(e) => setScreen({ ...screen, theme: e.target.value as 'dark' | 'light' })}>
                <option value="dark">כהה (מומלץ לטלוויזיה)</option>
                <option value="light">בהיר</option>
              </Select>
            </Field>
            <Field label="טקסט פתיחה" className="sm:col-span-2">
              <Input value={screen.welcomeText ?? ''} onChange={(e) => setScreen({ ...screen, welcomeText: e.target.value })} />
            </Field>
            <Field label="החלפת הודעות כל (שניות)">
              <Input type="number" min={3} max={300} value={screen.rotateSeconds ?? 12} onChange={(e) => setScreen({ ...screen, rotateSeconds: Number(e.target.value) })} />
            </Field>
            <div className="self-end">
              <Toggle checked={screen.isActive ?? true} onChange={(v) => setScreen({ ...screen, isActive: v })} label="מסך פעיל" />
            </div>
            <div className="sm:col-span-2">
              <span className="label">חדרים מוצגים (ריק = כל חדרי המבנה)</span>
              <div className="flex flex-wrap gap-1.5">
                {rooms
                  .filter((r) => !screen.buildingId || r.buildingId === screen.buildingId)
                  .map((r) => {
                    const on = screen.roomIds?.includes(r.id)
                    return (
                      <button
                        key={r.id}
                        type="button"
                        onClick={() => setScreen({ ...screen, roomIds: on ? screen.roomIds!.filter((x) => x !== r.id) : [...(screen.roomIds ?? []), r.id] })}
                        className={clsx('rounded-xl border px-3 py-1.5 text-xs', on ? 'border-brand bg-brand-soft text-brand-strong' : 'border-line')}
                      >
                        {r.name}
                      </button>
                    )
                  })}
              </div>
            </div>
          </div>
        )}
      </Modal>
      <AnnouncementDialog ann={ann} screens={screens.data ?? []} onClose={() => setAnn(null)} />
    </div>
  )
}

function AnnouncementDialog({ ann, screens, onClose }: { ann: Partial<Announcement> | null; screens: SignageScreen[]; onClose: () => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState<Partial<Announcement>>({})
  const file = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)
  useEffect(() => setF(ann ?? {}), [ann])
  const save = useMutation({
    mutationFn: () => (f.id ? api.put(`/signage/announcements/${f.id}`, f) : api.post<Announcement>('/signage/announcements', f)),
    onSuccess: () => {
      toast.success('ההודעה נשמרה ותוצג במסכים')
      qc.invalidateQueries({ queryKey: ['announcements'] })
      onClose()
    },
    onError: (e) => toast.error(errorMessage(e)),
  })

  const upload = async (fl: File) => {
    // Images are attached to the announcement; create it first if needed.
    setUploading(true)
    try {
      let id = f.id
      if (!id) {
        const created = await api.post<Announcement>('/signage/announcements', { ...f, title: f.title || 'הודעה חדשה' })
        id = created.id
        setF(created)
      }
      const fd = new FormData()
      fd.set('entity', 'announcement')
      fd.set('entityId', id)
      fd.set('file', fl)
      const att = await api.post<Attachment>('/files', fd)
      setF((p) => ({ ...p, id, imageKey: att.id }))
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setUploading(false)
    }
  }

  return (
    <Modal
      open={!!ann}
      onClose={onClose}
      title={f.id ? 'עריכת הודעה' : 'הודעה חדשה'}
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
        <Field label="כותרת" className="sm:col-span-2">
          <Input value={f.title ?? ''} onChange={(e) => setF({ ...f, title: e.target.value })} />
        </Field>
        <Field label="תוכן" className="sm:col-span-2">
          <Textarea value={f.body ?? ''} onChange={(e) => setF({ ...f, body: e.target.value })} />
        </Field>
        <Field label="מוצג מ־">
          <Input type="datetime-local" value={f.startsAt ? toLocalInput(f.startsAt) : ''} onChange={(e) => setF({ ...f, startsAt: fromLocalInput(e.target.value) })} />
        </Field>
        <Field label="עד (ריק = ללא הגבלה)">
          <Input type="datetime-local" value={f.endsAt ? toLocalInput(f.endsAt) : ''} onChange={(e) => setF({ ...f, endsAt: e.target.value ? fromLocalInput(e.target.value) : null })} />
        </Field>
        <Field label="מסך">
          <Select value={f.screenId ?? ''} onChange={(e) => setF({ ...f, screenId: e.target.value || null })}>
            <option value="">כל המסכים</option>
            {screens.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="עדיפות">
          <Select value={f.priority ?? 0} onChange={(e) => setF({ ...f, priority: Number(e.target.value) })}>
            <option value={0}>רגילה</option>
            <option value={1}>גבוהה</option>
            <option value={2}>דחופה (ראשונה)</option>
          </Select>
        </Field>
        <div className="sm:col-span-2">
          <span className="label">תמונה (PNG / JPG)</span>
          <div className="flex items-center gap-3">
            {f.imageKey && <img src={publicFileUrl(f.imageKey)} alt="" className="h-20 w-32 rounded-xl object-cover" />}
            <Button type="button" variant="secondary" icon={<ImagePlus className="h-4 w-4" />} loading={uploading} onClick={() => file.current?.click()}>
              {f.imageKey ? 'החלפת תמונה' : 'העלאת תמונה'}
            </Button>
            {f.imageKey && (
              <Button type="button" variant="ghost" onClick={() => setF({ ...f, imageKey: null })}>
                הסרה
              </Button>
            )}
            <input ref={file} type="file" hidden accept="image/png,image/jpeg,image/webp" onChange={(e) => e.target.files?.[0] && upload(e.target.files[0])} />
          </div>
        </div>
      </div>
    </Modal>
  )
}
