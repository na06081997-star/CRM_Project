import clsx from 'clsx'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Search, ShieldCheck } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from '@/components/toast'
import { Avatar, Badge, Button, Field, Input, Modal, PageHeader, PageLoader, Select, Tabs, Toggle } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { roleLabel, staffKindLabel } from '@/lib/labels'
import { useUsers } from '@/lib/queries'
import type { Role, User } from '@/lib/types'

const ROLES: Role[] = ['super_admin', 'admin', 'coordinator', 'staff', 'user']
const roleTone: Record<Role, 'red' | 'violet' | 'teal' | 'orange' | 'gray'> = { super_admin: 'red', admin: 'violet', coordinator: 'teal', staff: 'orange', user: 'gray' }

const permissionsInfo: Record<Role, string[]> = {
  super_admin: ['שליטה מלאה במערכת', 'ניהול משתמשים והרשאות', 'הגדרות מערכת, מבנים וציוד'],
  admin: ['אישור / דחיית בקשות של כל הרכזים', 'יומן גלובלי, חדרים ותמחור', 'כל התשלומים והיתרות', 'משימות צוות ושילוט'],
  coordinator: ['יומן זמינות (פנוי / תפוס + סוג)', 'בקשת מועדים לפעילויות שלו', 'רואה רק את הבקשות והפעילויות שלו', 'בקשת ביטול'],
  staff: ['המשימות שלי וסימון "בוצע"', 'קבלת תזכורות', 'צפייה בזמינות חדרים'],
  user: ['חיפוש חדרים פנויים', 'הגשת בקשות הזמנה', 'מעקב סטטוס ההזמנות שלי'],
}

export function UsersPage() {
  const { data, isLoading } = useUsers()
  const [role, setRole] = useState<Role | 'all'>('all')
  const [q, setQ] = useState('')
  const [edit, setEdit] = useState<Partial<User> | null>(null)
  if (isLoading) return <PageLoader />
  const list = (data ?? []).filter((u) => (role === 'all' || u.role === role) && (!q || `${u.fullName} ${u.email} ${u.phone ?? ''}`.includes(q)))

  return (
    <div>
      <PageHeader
        title="משתמשים והרשאות"
        subtitle="מנהלים, רכזים, צוות תפעול ומשתמשי פורטל"
        actions={
          <Button icon={<Plus className="h-4 w-4" />} onClick={() => setEdit({ role: 'user', isActive: true })}>
            משתמש חדש
          </Button>
        }
      />
      <div className="mb-6 grid gap-3 md:grid-cols-3 xl:grid-cols-5">
        {ROLES.map((r) => (
          <div key={r} className="card p-4">
            <div className="flex items-center justify-between">
              <Badge tone={roleTone[r]}>
                <ShieldCheck className="h-3 w-3" /> {roleLabel[r]}
              </Badge>
              <span className="text-xl font-bold">{(data ?? []).filter((u) => u.role === r).length}</span>
            </div>
            <ul className="mt-3 space-y-1 text-xs text-muted">
              {permissionsInfo[r].map((p) => (
                <li key={p}>• {p}</li>
              ))}
            </ul>
          </div>
        ))}
      </div>
      <div className="card overflow-hidden">
        <div className="flex flex-wrap items-center gap-3 border-b border-line p-4">
          <div className="relative max-w-xs flex-1">
            <Search className="absolute start-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
            <Input className="ps-9" placeholder="חיפוש" value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          <Tabs value={role} onChange={setRole} items={[{ value: 'all' as const, label: 'הכל' }, ...ROLES.map((r) => ({ value: r, label: roleLabel[r] }))]} />
        </div>
        <ul className="divide-y divide-line">
          {list.map((u) => (
            <li key={u.id}>
              <button onClick={() => setEdit(u)} className={clsx('flex w-full items-center gap-3 px-5 py-3 text-start hover:bg-subtle', !u.isActive && 'opacity-50')}>
                <Avatar name={u.fullName} color={u.avatarColor} />
                <span className="min-w-0 flex-1">
                  <span className="block font-medium">{u.fullName}</span>
                  <span className="block truncate text-xs text-muted">
                    {u.email} {u.phone && `· ${u.phone}`}
                  </span>
                </span>
                {u.staffKind && <Badge>{staffKindLabel[u.staffKind]}</Badge>}
                <Badge tone={roleTone[u.role]}>{roleLabel[u.role]}</Badge>
                {!u.isActive && <Badge tone="red">מושבת</Badge>}
              </button>
            </li>
          ))}
        </ul>
      </div>
      <UserDialog user={edit} onClose={() => setEdit(null)} />
    </div>
  )
}

function UserDialog({ user, onClose }: { user: Partial<User> | null; onClose: () => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState<Partial<User>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  useEffect(() => {
    setF(user ?? {})
    setErrors({})
  }, [user])
  const isNew = !user?.id
  const save = useMutation({
    mutationFn: () => (isNew ? api.post('/users', f) : api.put(`/users/${user!.id}`, f)),
    onSuccess: () => {
      toast.success('המשתמש נשמר')
      qc.invalidateQueries({ queryKey: ['users'] })
      onClose()
    },
    onError: (e) => {
      if (e instanceof ApiError) setErrors(e.fields)
      toast.error(errorMessage(e))
    },
  })
  return (
    <Modal
      open={!!user}
      onClose={onClose}
      title={isNew ? 'משתמש חדש' : 'עריכת משתמש'}
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
        <Field label="שם מלא" error={errors.fullName}>
          <Input value={f.fullName ?? ''} onChange={(e) => setF({ ...f, fullName: e.target.value })} />
        </Field>
        <Field label="אימייל (לכניסה עם Google)" error={errors.email}>
          <Input dir="ltr" type="email" value={f.email ?? ''} onChange={(e) => setF({ ...f, email: e.target.value })} />
        </Field>
        <Field label="טלפון נייד (לתזכורות SMS)">
          <Input dir="ltr" value={f.phone ?? ''} onChange={(e) => setF({ ...f, phone: e.target.value || null })} placeholder="050-0000000" />
        </Field>
        <Field label="Telegram chat ID (תזכורות חינם)" hint="המשתמש שולח /start לבוט ומקבל מזהה">
          <Input dir="ltr" value={f.telegramChatId ?? ''} onChange={(e) => setF({ ...f, telegramChatId: e.target.value || null })} />
        </Field>
        <Field label="תפקיד" error={errors.role}>
          <Select value={f.role ?? 'user'} onChange={(e) => setF({ ...f, role: e.target.value as Role })}>
            {ROLES.map((r) => (
              <option key={r} value={r}>
                {roleLabel[r]}
              </option>
            ))}
          </Select>
        </Field>
        {f.role === 'staff' && (
          <Field label="תחום">
            <Select value={f.staffKind ?? ''} onChange={(e) => setF({ ...f, staffKind: e.target.value || null })}>
              <option value="">—</option>
              {Object.entries(staffKindLabel).map(([k, v]) => (
                <option key={k} value={k}>
                  {v}
                </option>
              ))}
            </Select>
          </Field>
        )}
        <div className="sm:col-span-2">
          <Toggle checked={f.isActive ?? true} onChange={(v) => setF({ ...f, isActive: v })} label="משתמש פעיל" />
        </div>
      </div>
    </Modal>
  )
}
