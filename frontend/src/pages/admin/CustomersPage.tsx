import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Building, Mail, Phone, Plus, Search, Trash2, UserRound, Users } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { toast } from '@/components/toast'
import { Badge, Button, Confirm, Drawer, Empty, Field, Input, Modal, PageHeader, PageLoader, Select, Tabs, Textarea } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { dateShort, money, timeRange } from '@/lib/format'
import { bookingStatusLabel, customerKindLabel, paymentMethodLabel } from '@/lib/labels'
import { useCustomers } from '@/lib/queries'
import type { Booking, Customer, Payment } from '@/lib/types'

export function CustomersPage() {
  const [params, setParams] = useSearchParams()
  const [q, setQ] = useState('')
  const [debounced, setDebounced] = useState('')
  const [kind, setKind] = useState<'all' | Customer['kind']>('all')
  const [edit, setEdit] = useState<Partial<Customer> | null>(null)
  useEffect(() => {
    const t = setTimeout(() => setDebounced(q), 250)
    return () => clearTimeout(t)
  }, [q])
  const { data, isLoading } = useCustomers(debounced)
  const list = (data ?? []).filter((c) => kind === 'all' || c.kind === kind)
  const openId = params.get('id')

  return (
    <div>
      <PageHeader
        title="לקוחות"
        subtitle="מזמינים פרטיים, ארגונים ועמותות – היסטוריית הזמנות ויתרות"
        actions={
          <Button icon={<Plus className="h-4 w-4" />} onClick={() => setEdit({ kind: 'private' })}>
            לקוח חדש
          </Button>
        }
      />
      <div className="card overflow-hidden">
        <div className="flex flex-wrap items-center gap-3 border-b border-line p-4">
          <div className="relative max-w-sm flex-1">
            <Search className="absolute start-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
            <Input className="ps-9" placeholder="שם, איש קשר, טלפון או אימייל" value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          <Tabs
            value={kind}
            onChange={setKind}
            items={[
              { value: 'all', label: 'הכל' },
              { value: 'private', label: 'פרטיים' },
              { value: 'organization', label: 'ארגונים' },
              { value: 'internal', label: 'פנימי' },
            ]}
          />
        </div>
        {isLoading ? (
          <PageLoader />
        ) : list.length === 0 ? (
          <Empty icon={<Users className="h-6 w-6" />} title="לא נמצאו לקוחות" />
        ) : (
          <div className="scroll-thin overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-subtle/70 text-xs text-muted">
                <tr className="[&>th]:px-4 [&>th]:py-2.5 [&>th]:text-start [&>th]:font-medium">
                  <th>לקוח</th>
                  <th>סוג</th>
                  <th>איש קשר</th>
                  <th>טלפון</th>
                  <th className="text-end">הזמנות</th>
                  <th className="text-end">חויב</th>
                  <th className="text-end">יתרה</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {list.map((c) => {
                  const bal = c.totalBilled - c.totalPaid
                  return (
                    <tr key={c.id} onClick={() => setParams({ id: c.id })} className="cursor-pointer hover:bg-subtle [&>td]:px-4 [&>td]:py-3">
                      <td className="font-medium">
                        <span className="flex items-center gap-2">
                          {c.kind === 'organization' ? <Building className="h-4 w-4 text-muted" /> : <UserRound className="h-4 w-4 text-muted" />}
                          {c.name}
                        </span>
                      </td>
                      <td>
                        <Badge tone={c.kind === 'organization' ? 'blue' : c.kind === 'internal' ? 'violet' : 'gray'}>{customerKindLabel[c.kind]}</Badge>
                      </td>
                      <td className="text-muted">{c.contactName || '—'}</td>
                      <td className="tabular-nums text-muted" dir="ltr">
                        {c.phone || '—'}
                      </td>
                      <td className="text-end tabular-nums">{c.bookingsCount}</td>
                      <td className="text-end tabular-nums">{money(c.totalBilled)}</td>
                      <td className={clsx('text-end tabular-nums', bal > 0 && 'font-semibold text-danger')}>{money(bal)}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
      <CustomerDialog customer={edit} onClose={() => setEdit(null)} />
      <CustomerDrawer id={openId} onClose={() => setParams({})} onEdit={(c) => setEdit(c)} />
    </div>
  )
}

function CustomerDialog({ customer, onClose }: { customer: Partial<Customer> | null; onClose: () => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState<Partial<Customer>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  useEffect(() => {
    setF(customer ?? {})
    setErrors({})
  }, [customer])
  const isNew = !customer?.id
  const save = useMutation({
    mutationFn: () => (isNew ? api.post('/customers', f) : api.put(`/customers/${customer!.id}`, f)),
    onSuccess: () => {
      toast.success('הלקוח נשמר')
      qc.invalidateQueries({ queryKey: ['customers'] })
      qc.invalidateQueries({ queryKey: ['customer'] })
      onClose()
    },
    onError: (e) => {
      if (e instanceof ApiError) setErrors(e.fields)
      toast.error(errorMessage(e))
    },
  })
  const text = (k: keyof Customer) => ({ value: (f[k] as string) ?? '', onChange: (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value }) })
  return (
    <Modal
      open={!!customer}
      onClose={onClose}
      title={isNew ? 'לקוח חדש' : 'עריכת לקוח'}
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
        <Field label="שם הלקוח / הארגון" error={errors.name} className="sm:col-span-2">
          <Input {...text('name')} />
        </Field>
        <Field label="סוג">
          <Select value={f.kind ?? 'private'} onChange={(e) => setF({ ...f, kind: e.target.value as Customer['kind'] })}>
            {Object.entries(customerKindLabel).map(([k, v]) => (
              <option key={k} value={k}>
                {v}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="איש קשר">
          <Input {...text('contactName')} />
        </Field>
        <Field label="טלפון">
          <Input {...text('phone')} dir="ltr" />
        </Field>
        <Field label="אימייל">
          <Input {...text('email')} dir="ltr" type="email" />
        </Field>
        <Field label="ח.פ / ע.מ">
          <Input {...text('taxId')} />
        </Field>
        <Field label="הערות" className="sm:col-span-2">
          <Textarea {...text('notes')} />
        </Field>
      </div>
    </Modal>
  )
}

function CustomerDrawer({ id, onClose, onEdit }: { id: string | null; onClose: () => void; onEdit: (c: Customer) => void }) {
  const qc = useQueryClient()
  const [confirm, setConfirm] = useState(false)
  const { data, isLoading } = useQuery({
    queryKey: ['customer', id],
    queryFn: () => api.get<{ customer: Customer; bookings: Booking[]; payments: Payment[] }>(`/customers/${id}`),
    enabled: !!id,
  })
  const del = useMutation({
    mutationFn: () => api.del(`/customers/${id}`),
    onSuccess: () => {
      toast.success('הלקוח נמחק')
      qc.invalidateQueries({ queryKey: ['customers'] })
      setConfirm(false)
      onClose()
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  if (!id) return null
  const c = data?.customer
  const bal = c ? c.totalBilled - c.totalPaid : 0
  return (
    <Drawer
      open
      onClose={onClose}
      title={c?.name ?? 'לקוח'}
      footer={
        c && (
          <>
            <Button variant="ghost" className="me-auto text-danger" icon={<Trash2 className="h-4 w-4" />} onClick={() => setConfirm(true)}>
              מחיקה
            </Button>
            <Button variant="secondary" onClick={() => onEdit(c)}>
              עריכת פרטים
            </Button>
          </>
        )
      }
    >
      {isLoading || !c ? (
        <PageLoader />
      ) : (
        <div className="space-y-5">
          <div className="flex flex-wrap gap-3 text-sm text-muted">
            <Badge>{customerKindLabel[c.kind]}</Badge>
            {c.phone && (
              <a href={`tel:${c.phone}`} className="flex items-center gap-1 hover:text-brand">
                <Phone className="h-4 w-4" /> {c.phone}
              </a>
            )}
            {c.email && (
              <a href={`mailto:${c.email}`} className="flex items-center gap-1 hover:text-brand">
                <Mail className="h-4 w-4" /> {c.email}
              </a>
            )}
          </div>
          <div className="grid grid-cols-3 gap-2">
            <Mini label="הזמנות" value={String(c.bookingsCount)} />
            <Mini label="שולם" value={money(c.totalPaid)} />
            <Mini label="יתרה" value={money(bal)} danger={bal > 0} />
          </div>
          {c.notes && <p className="rounded-xl bg-subtle p-3 text-sm">{c.notes}</p>}
          <section>
            <h4 className="mb-2 text-sm font-semibold">הזמנות</h4>
            <ul className="divide-y divide-line rounded-xl border border-line">
              {data.bookings.length === 0 && <li className="p-3 text-sm text-muted">אין הזמנות</li>}
              {[...data.bookings].reverse().map((b) => (
                <li key={b.id} className="flex items-center justify-between gap-2 px-3 py-2 text-sm">
                  <span className="min-w-0">
                    <span className="block truncate font-medium">{b.title}</span>
                    <span className="block text-xs text-muted">
                      {dateShort(b.startsAt)} · {timeRange(b.startsAt, b.endsAt)} · {b.roomName}
                    </span>
                  </span>
                  <span className="text-end text-xs">
                    <Badge tone={b.status === 'confirmed' ? 'green' : 'gray'}>{bookingStatusLabel[b.status]}</Badge>
                    {b.price > 0 && <span className="mt-1 block tabular-nums">{money(b.price)}</span>}
                  </span>
                </li>
              ))}
            </ul>
          </section>
          <section>
            <h4 className="mb-2 text-sm font-semibold">תשלומים</h4>
            <ul className="divide-y divide-line rounded-xl border border-line">
              {data.payments.length === 0 && <li className="p-3 text-sm text-muted">אין תשלומים</li>}
              {data.payments.map((p) => (
                <li key={p.id} className="flex items-center justify-between px-3 py-2 text-sm">
                  <span>
                    {paymentMethodLabel[p.method]} {p.reference && <span className="text-xs text-muted">· {p.reference}</span>}
                    <span className="block text-xs text-muted">{dateShort(p.paidAt)}</span>
                  </span>
                  <span className="font-semibold tabular-nums">{money(p.amount)}</span>
                </li>
              ))}
            </ul>
          </section>
        </div>
      )}
      <Confirm open={confirm} onClose={() => setConfirm(false)} onConfirm={() => del.mutate()} loading={del.isPending} danger title="מחיקת לקוח" message="לא ניתן למחוק לקוח עם הזמנות קיימות. להמשיך?" confirmLabel="מחיקה" />
    </Drawer>
  )
}

function Mini({ label, value, danger }: { label: string; value: string; danger?: boolean }) {
  return (
    <div className="rounded-xl bg-subtle p-3">
      <p className="text-[11px] text-muted">{label}</p>
      <p className={clsx('font-bold tabular-nums', danger && 'text-danger')}>{value}</p>
    </div>
  )
}
