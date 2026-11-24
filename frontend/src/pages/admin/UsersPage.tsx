import clsx from 'clsx'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { MailPlus, Plus, Search, Send, ShieldCheck } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from '@/components/toast'
import { Avatar, Badge, Button, Field, Input, Modal, PageHeader, PageLoader, Select, Tabs, Toggle } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { useAuth, useUser } from '@/lib/auth'
import { dateTime } from '@/lib/format'
import { roleLabel, staffKindLabel } from '@/lib/labels'
import { useUsers } from '@/lib/queries'
import type { AccountStatus, Invitation, Role, User } from '@/lib/types'

const ROLES: Role[] = ['super_admin', 'admin', 'coordinator', 'staff', 'user']
const roleTone: Record<Role, 'red' | 'violet' | 'teal' | 'orange' | 'gray'> = { super_admin: 'red', admin: 'violet', coordinator: 'teal', staff: 'orange', user: 'gray' }

const permissionsInfo: Record<Role, string[]> = {
  super_admin: ['שליטה מלאה במערכת', 'ניהול משתמשים והרשאות', 'הגדרות מערכת, מבנים וציוד'],
  admin: ['אישור / דחיית בקשות של כל הרכזים', 'יומן גלובלי, חדרים ותמחור', 'כל התשלומים והיתרות', 'משימות צוות ושילוט'],
  coordinator: ['יומן זמינות (פנוי / תפוס + סוג)', 'בקשת מועדים לפעילויות שלו', 'רואה רק את הבקשות והפעילויות שלו', 'בקשת ביטול'],
  staff: ['המשימות שלי וסימון "בוצע"', 'קבלת תזכורות', 'צפייה בזמינות חדרים'],
  user: ['חיפוש חדרים פנויים', 'הגשת בקשות הזמנה', 'מעקב סטטוס ההזמנות שלי'],
}

const statusInfo: Record<AccountStatus, { label: string; tone: 'green' | 'amber' | 'red' }> = {
  active: { label: 'פעיל', tone: 'green' },
  pending: { label: 'ממתין להפעלה', tone: 'amber' },
  disabled: { label: 'מושבת', tone: 'red' },
}

function invitationText(inv: Invitation) {
  switch (inv.status) {
    case 'sent':
      return { label: `הזמנה נשלחה ${dateTime(inv.sentAt!)}`, tone: 'teal' as const }
    case 'failed':
      return { label: 'שליחת ההזמנה נכשלה', tone: 'red' as const }
    case 'expired':
      return { label: `ההזמנה פגה ${dateTime(inv.expiresAt)}`, tone: 'amber' as const }
    case 'used':
      return { label: 'ההזמנה נוצלה', tone: 'gray' as const }
    default:
      return { label: 'הוחלפה בהזמנה חדשה', tone: 'gray' as const }
  }
}

function loginText(u: User) {
  if (u.lastLoginAt) return `כניסה אחרונה: ${dateTime(u.lastLoginAt)}`
  return u.loginHistoryKnown ? 'טרם התחבר/ה' : 'היסטוריה לא זמינה'
}

/** An invitation can be (re)sent while the account was never activated or has no password. */
const canInvite = (u: User) => u.isActive && (u.status === 'pending' || !u.hasPassword)

export function UsersPage() {
  const { data, isLoading } = useUsers()
  const passwordMode = useAuth((s) => s.config?.authMode === 'password')
  const [role, setRole] = useState<Role | 'all'>('all')
  const [q, setQ] = useState('')
  const [edit, setEdit] = useState<Partial<User> | null>(null)
  if (isLoading) return <PageLoader />
  const list = (data ?? []).filter((u) => (role === 'all' || u.role === role) && (!q || `${u.fullName} ${u.username} ${u.email} ${u.phone ?? ''}`.toLowerCase().includes(q.toLowerCase())))

  return (
    <div>
      <PageHeader
        title="משתמשים והרשאות"
        subtitle="מנהלים, רכזים, צוות תפעול ומשתמשי פורטל"
        actions={
          <Button icon={passwordMode ? <MailPlus className="h-4 w-4" /> : <Plus className="h-4 w-4" />} onClick={() => setEdit({ role: 'coordinator', isActive: true })}>
            {passwordMode ? 'הזמנת משתמש' : 'משתמש חדש'}
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
          <div className="relative min-w-0 max-w-xs flex-1">
            <Search className="absolute start-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
            <Input className="ps-9" placeholder="חיפוש" value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          <Tabs value={role} onChange={setRole} items={[{ value: 'all' as const, label: 'הכל' }, ...ROLES.map((r) => ({ value: r, label: roleLabel[r] }))]} />
        </div>
        <ul className="divide-y divide-line">
          {list.map((u) => {
            const st = statusInfo[u.status] ?? statusInfo.active
            const inv = u.invitation && u.status !== 'active' ? invitationText(u.invitation) : null
            return (
              <li key={u.id}>
                <button onClick={() => setEdit(u)} className={clsx('flex w-full items-start gap-3 px-4 py-3 text-start hover:bg-subtle sm:px-5', u.status === 'disabled' && 'opacity-60')}>
                  <Avatar name={u.fullName} color={u.avatarColor} />
                  <span className="min-w-0 flex-1">
                    <span className="flex flex-wrap items-center gap-x-2">
                      <span className="font-medium">{u.fullName}</span>
                      <span className="text-xs text-muted" dir="ltr">
                        @{u.username}
                      </span>
                    </span>
                    <span className="block truncate text-xs text-muted" dir="ltr">
                      {u.email}
                    </span>
                    <span className="mt-1.5 flex flex-wrap items-center gap-1.5">
                      <Badge tone={roleTone[u.role]}>{roleLabel[u.role]}</Badge>
                      {u.staffKind && <Badge>{staffKindLabel[u.staffKind]}</Badge>}
                      <Badge tone={st.tone} dot>
                        {st.label}
                      </Badge>
                      {inv && <Badge tone={inv.tone}>{inv.label}</Badge>}
                      {passwordMode && u.status === 'active' && !u.hasPassword && <Badge tone="amber">ללא סיסמה</Badge>}
                    </span>
                    <span className="mt-1 block text-xs text-muted">{loginText(u)}</span>
                  </span>
                </button>
              </li>
            )
          })}
        </ul>
      </div>
      <UserDialog user={edit} passwordMode={passwordMode} onClose={() => setEdit(null)} />
    </div>
  )
}

function UserDialog({ user, passwordMode, onClose }: { user: Partial<User> | null; passwordMode: boolean; onClose: () => void }) {
  const qc = useQueryClient()
  const me = useUser()
  const [f, setF] = useState<Partial<User>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  useEffect(() => {
    setF(user ?? {})
    setErrors({})
  }, [user])
  const isNew = !user?.id
  const inviting = isNew && passwordMode
  const onError = (e: unknown) => {
    if (e instanceof ApiError) setErrors(e.fields)
    toast.error(errorMessage(e))
  }
  const save = useMutation({
    mutationFn: () => (inviting ? api.post<User>('/users/invite', f) : isNew ? api.post<User>('/users', f) : api.put<User>(`/users/${user!.id}`, f)),
    onSuccess: (saved) => {
      if (inviting && saved?.invitation?.status === 'failed') toast.error('המשתמש נוצר, אך שליחת ההזמנה נכשלה – אפשר לשלוח שוב מכרטיס המשתמש')
      else toast.success(inviting ? `ההזמנה נשלחה ל־${f.email}` : 'המשתמש נשמר')
      qc.invalidateQueries({ queryKey: ['users'] })
      onClose()
    },
    onError,
  })
  const resend = useMutation({
    mutationFn: () => api.post<User>(`/users/${user!.id}/invite`),
    onSuccess: (u) => {
      if (u.invitation?.status === 'failed') toast.error(`שליחת ההזמנה נכשלה: ${u.invitation.deliveryError ?? ''}`)
      else toast.success('הזמנה חדשה נשלחה – הקישורים הקודמים בוטלו')
      qc.invalidateQueries({ queryKey: ['users'] })
      setF((cur) => ({ ...cur, invitation: u.invitation }))
    },
    onError,
  })
  const u = user as User | null
  const inv = f.invitation ? invitationText(f.invitation) : null
  const assignable = ROLES.filter((r) => r !== 'super_admin' || me?.role === 'super_admin')

  return (
    <Modal
      open={!!user}
      onClose={onClose}
      title={inviting ? 'הזמנת משתמש' : isNew ? 'משתמש חדש' : 'עריכת משתמש'}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            ביטול
          </Button>
          <Button onClick={() => save.mutate()} loading={save.isPending} icon={inviting ? <Send className="h-4 w-4" /> : undefined}>
            {inviting ? 'שליחת הזמנה' : 'שמירה'}
          </Button>
        </>
      }
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="שם משתמש (לכניסה)" error={errors.username} hint="אותיות לטיניות קטנות, ספרות, נקודה או מקף">
          <Input dir="ltr" autoCapitalize="none" spellCheck={false} value={f.username ?? ''} onChange={(e) => setF({ ...f, username: e.target.value.toLowerCase() })} placeholder="dana.cohen" />
        </Field>
        <Field label="שם מלא" error={errors.fullName}>
          <Input value={f.fullName ?? ''} onChange={(e) => setF({ ...f, fullName: e.target.value })} />
        </Field>
        <Field label="אימייל" error={errors.email} hint={passwordMode ? 'אליו נשלחים ההזמנה וקודי האימות' : undefined}>
          <Input dir="ltr" type="email" value={f.email ?? ''} onChange={(e) => setF({ ...f, email: e.target.value })} />
        </Field>
        <Field label="תפקיד" error={errors.role}>
          <Select value={f.role ?? 'user'} onChange={(e) => setF({ ...f, role: e.target.value as Role })}>
            {assignable.map((r) => (
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
        <Field label="טלפון נייד">
          <Input dir="ltr" value={f.phone ?? ''} onChange={(e) => setF({ ...f, phone: e.target.value || null })} placeholder="050-0000000" />
        </Field>
        {!isNew && (
          <div className="sm:col-span-2">
            <Toggle checked={f.isActive ?? true} onChange={(v) => setF({ ...f, isActive: v })} label="משתמש פעיל" />
          </div>
        )}
        {inviting && (
          <p className="rounded-xl bg-brand-soft/50 p-3 text-sm sm:col-span-2">
            יישלח אימייל מ־noreply עם קישור אישי (בתוקף 24 שעות) לבחירת סיסמה. בכל כניסה יידרש גם קוד בן 6 ספרות שנשלח לאימייל.
          </p>
        )}
      </div>

      {u?.id && (
        <div className="mt-6 rounded-xl border border-line p-4">
          <p className="mb-3 text-sm font-semibold">גישה למערכת</p>
          <dl className="grid gap-x-4 gap-y-2 text-sm sm:grid-cols-2">
            <Row label="מצב החשבון">
              <Badge tone={statusInfo[u.status].tone} dot>
                {statusInfo[u.status].label}
              </Badge>
            </Row>
            {passwordMode && <Row label="סיסמה">{u.hasPassword ? 'נקבעה' : 'טרם נקבעה'}</Row>}
            <Row label="הזמנה">{inv ? <Badge tone={inv.tone}>{inv.label}</Badge> : <span className="text-muted">לא נשלחה</span>}</Row>
            {f.invitation?.status === 'sent' && <Row label="ההזמנה בתוקף עד">{dateTime(f.invitation.expiresAt)}</Row>}
            <Row label="כניסה ראשונה">{u.firstLoginAt ? (u.loginHistoryKnown ? dateTime(u.firstLoginAt) : 'היסטוריה לא זמינה') : u.loginHistoryKnown ? 'טרם התחבר/ה' : 'היסטוריה לא זמינה'}</Row>
            <Row label="כניסה אחרונה">{u.lastLoginAt ? dateTime(u.lastLoginAt) : u.loginHistoryKnown ? '—' : 'היסטוריה לא זמינה'}</Row>
          </dl>
          {f.invitation?.status === 'failed' && f.invitation.deliveryError && (
            <p className="mt-3 rounded-lg bg-rose-50 p-2 text-xs text-rose-700 dark:bg-rose-950/40 dark:text-rose-200" dir="ltr">
              {f.invitation.deliveryError}
            </p>
          )}
          {passwordMode && canInvite(u) && (
            <Button className="mt-4 w-full sm:w-auto" variant="secondary" icon={<Send className="h-4 w-4" />} loading={resend.isPending} onClick={() => resend.mutate()}>
              {f.invitation ? 'שליחת הזמנה מחדש' : 'שליחת הזמנה'}
            </Button>
          )}
        </div>
      )}
    </Modal>
  )
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-3 sm:block">
      <dt className="text-xs text-muted">{label}</dt>
      <dd className="mt-0.5">{children}</dd>
    </div>
  )
}
