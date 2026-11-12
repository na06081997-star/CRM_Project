import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Banknote, Building2, CreditCard, Download, FileClock, Landmark, ReceiptText, Search, Trash2, Wallet } from 'lucide-react'
import { addMonths, startOfMonth, startOfYear } from 'date-fns'
import { useState } from 'react'
import { PaymentDialog } from '@/components/PaymentDialog'
import { toast } from '@/components/toast'
import { Badge, Button, Card, Confirm, Empty, IconButton, Input, PageHeader, PageLoader, Stat, Tabs } from '@/components/ui'
import { api, errorMessage } from '@/lib/api'
import { useCan } from '@/lib/auth'
import { dateShort, dateTime, fmt, money, timeRange } from '@/lib/format'
import { paymentMethodLabel } from '@/lib/labels'
import type { AuditEntry, Booking, Payment, PaymentMethod, PaymentSummary, Report } from '@/lib/types'

const methodIcon: Record<PaymentMethod, JSX.Element> = {
  bank_transfer: <Landmark className="h-4 w-4" />,
  check: <ReceiptText className="h-4 w-4" />,
  cash: <Banknote className="h-4 w-4" />,
  credit_card: <CreditCard className="h-4 w-4" />,
}

export function PaymentsPage() {
  const can = useCan()
  const [tab, setTab] = useState<'billing' | 'rooms' | 'payments' | 'audit'>('billing')
  const summary = useQuery({ queryKey: ['payments', 'summary'], queryFn: () => api.get<PaymentSummary>('/payments/summary') })
  const s = summary.data

  return (
    <div>
      <PageHeader title="תשלומים ויתרות" subtitle="מעקב גבייה לפי הזמנה · העברה בנקאית, צ׳ק, מזומן ואשראי · יומן ביקורת מלא" />
      {s && (
        <div className="mb-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <Stat label="סה״כ חויב (הזמנות פעילות)" value={money(s.totalBilled)} icon={<Wallet className="h-5 w-5" />} />
          <Stat label="נגבה" value={money(s.totalPaid)} tone="green" icon={<Banknote className="h-5 w-5" />} hint={`${s.totalBilled ? Math.round((s.totalPaid / s.totalBilled) * 100) : 0}% מהחיובים`} />
          <Stat label="יתרה לגבייה" value={money(s.outstanding)} tone="red" icon={<FileClock className="h-5 w-5" />} hint={`${s.unpaidCount} הזמנות פתוחות`} />
          <Stat label="נגבה החודש" value={money(s.paidThisMonth)} tone="blue" icon={<CreditCard className="h-5 w-5" />} />
        </div>
      )}
      {s && (
        <div className="mb-6 flex flex-wrap gap-2">
          {(Object.keys(paymentMethodLabel) as PaymentMethod[]).map((m) => (
            <span key={m} className="chip border border-line bg-surface px-3 py-1.5 text-sm">
              {methodIcon[m]} {paymentMethodLabel[m]}: <b className="tabular-nums">{money(s.byMethod[m] ?? 0)}</b>
            </span>
          ))}
        </div>
      )}
      <Tabs
        className="mb-4 max-w-full overflow-x-auto [&>button]:shrink-0 [&>button]:whitespace-nowrap"
        value={tab}
        onChange={setTab}
        items={[
          { value: 'billing', label: 'מי שילם / מי לא' },
          { value: 'rooms', label: 'פילוח לפי אולם' },
          { value: 'payments', label: 'תשלומים שהתקבלו' },
          { value: 'audit', label: 'יומן ביקורת' },
        ]}
      />
      {tab === 'billing' && <Billing canPay={can('payments:manage')} />}
      {tab === 'rooms' && <ByRoom />}
      {tab === 'payments' && <PaymentsList canDelete={can('payments:manage')} />}
      {tab === 'audit' && <Audit />}
    </div>
  )
}

type PayFilter = 'open' | 'unpaid' | 'partial' | 'paid' | 'all'
const payFilters: { value: PayFilter; label: string }[] = [
  { value: 'open', label: 'טרם שולם במלואו' },
  { value: 'unpaid', label: 'לא שילמו כלל' },
  { value: 'partial', label: 'שולם חלקית' },
  { value: 'paid', label: 'שולם במלואו' },
  { value: 'all', label: 'הכל' },
]

function payState(b: Booking) {
  if (b.paidAmount >= b.price) return { label: 'שולם', tone: 'green' as const }
  if (b.paidAmount > 0) return { label: 'חלקי', tone: 'amber' as const }
  return { label: 'לא שולם', tone: 'red' as const }
}

/** Charged bookings by payment state: who paid and who did not. */
function Billing({ canPay }: { canPay: boolean }) {
  const [paying, setPaying] = useState<Booking | null>(null)
  const [filter, setFilter] = useState<PayFilter>('open')
  const [q, setQ] = useState('')
  const { data = [], isLoading } = useQuery({ queryKey: ['billing', filter], queryFn: () => api.get<Booking[]>('/payments/billing', { payment: filter }) })
  const term = q.trim().toLowerCase()
  const list = term ? data.filter((b) => `${b.customerName ?? ''} ${b.title} ${b.roomName}`.toLowerCase().includes(term)) : data
  const due = list.reduce((s, b) => s + Math.max(0, b.price - b.paidAmount), 0)
  const paid = list.reduce((s, b) => s + b.paidAmount, 0)

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="-mx-1 flex max-w-full gap-1.5 overflow-x-auto px-1 pb-1">
          {payFilters.map((p) => (
            <button
              key={p.value}
              onClick={() => setFilter(p.value)}
              className={clsx('min-h-9 shrink-0 rounded-full border px-3 text-sm', filter === p.value ? 'border-brand bg-brand-soft font-semibold text-brand-strong' : 'border-line bg-surface')}
            >
              {p.label}
            </button>
          ))}
        </div>
        <label className="relative w-full sm:ms-auto sm:w-64">
          <Search className="pointer-events-none absolute start-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
          <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="חיפוש לקוח / הזמנה" className="ps-9" />
        </label>
      </div>
      <Card padded={false} title={`${list.length} הזמנות · שולם ${money(paid)} · יתרה ${money(due)}`}>
        {isLoading ? (
          <PageLoader />
        ) : list.length === 0 ? (
          <Empty title={filter === 'open' ? 'כל ההזמנות שולמו במלואן 🎉' : 'אין הזמנות בסינון זה'} />
        ) : (
          <>
            {/* Phones: one card per booking */}
            <ul className="divide-y divide-line sm:hidden">
              {list.map((b) => {
                const st = payState(b)
                return (
                  <li key={b.id} className="px-4 py-3">
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <p className="truncate font-medium">{b.customerName || '—'}</p>
                        <p className="truncate text-xs text-muted">
                          {b.title} · {dateShort(b.startsAt)} · {b.roomName}
                        </p>
                      </div>
                      <Badge tone={st.tone}>{st.label}</Badge>
                    </div>
                    <div className="mt-2 flex items-center justify-between text-sm">
                      <span className="tabular-nums text-muted">
                        {money(b.paidAmount)} / {money(b.price)}
                      </span>
                      {b.price > b.paidAmount && canPay ? (
                        <Button size="sm" variant="soft" onClick={() => setPaying(b)}>
                          רישום תשלום
                        </Button>
                      ) : null}
                    </div>
                  </li>
                )
              })}
            </ul>
            <div className="scroll-thin hidden overflow-x-auto sm:block">
              <table className="w-full text-sm">
                <thead className="bg-subtle/70 text-xs text-muted">
                  <tr className="[&>th]:px-4 [&>th]:py-2.5 [&>th]:text-start [&>th]:font-medium">
                    <th>לקוח</th>
                    <th>הזמנה</th>
                    <th>חלל</th>
                    <th>מועד</th>
                    <th className="text-end">מחיר</th>
                    <th className="text-end">שולם</th>
                    <th className="text-end">יתרה</th>
                    <th>סטטוס</th>
                    <th />
                  </tr>
                </thead>
                <tbody className="divide-y divide-line">
                  {list.map((b) => {
                    const st = payState(b)
                    const balance = b.price - b.paidAmount
                    const overdue = balance > 0 && new Date(b.startsAt).getTime() < Date.now()
                    return (
                      <tr key={b.id} className="[&>td]:px-4 [&>td]:py-3">
                        <td className="font-medium">{b.customerName || '—'}</td>
                        <td className="max-w-[200px] truncate">{b.title}</td>
                        <td className="whitespace-nowrap text-muted">{b.roomName}</td>
                        <td className="whitespace-nowrap tabular-nums text-muted">
                          {dateShort(b.startsAt)} {timeRange(b.startsAt, b.endsAt)} {overdue && <Badge tone="red">עבר</Badge>}
                        </td>
                        <td className="text-end tabular-nums">{money(b.price)}</td>
                        <td className="text-end tabular-nums">{money(b.paidAmount)}</td>
                        <td className={clsx('text-end font-semibold tabular-nums', balance > 0 ? 'text-danger' : 'text-ok')}>{money(Math.max(0, balance))}</td>
                        <td>
                          <Badge tone={st.tone}>{st.label}</Badge>
                        </td>
                        <td>
                          {canPay && balance > 0 && (
                            <Button size="sm" variant="soft" onClick={() => setPaying(b)}>
                              רישום תשלום
                            </Button>
                          )}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </>
        )}
        <PaymentDialog booking={paying} open={!!paying} onClose={() => setPaying(null)} />
      </Card>
    </div>
  )
}

/** Revenue per room / hall over a period. */
function ByRoom() {
  const presets = [
    { key: 'month', label: 'החודש', from: startOfMonth(new Date()), to: addMonths(startOfMonth(new Date()), 1) },
    { key: 'quarter', label: '3 חודשים', from: addMonths(startOfMonth(new Date()), -2), to: addMonths(startOfMonth(new Date()), 1) },
    { key: 'year', label: 'השנה', from: startOfYear(new Date()), to: addMonths(startOfMonth(new Date()), 1) },
  ]
  const [preset, setPreset] = useState('quarter')
  const p = presets.find((x) => x.key === preset)!
  const from = fmt(p.from, 'yyyy-MM-dd')
  const to = fmt(p.to, 'yyyy-MM-dd')
  const { data, isLoading } = useQuery({ queryKey: ['reports', from, to], queryFn: () => api.get<Report>('/reports', { from, to }) })
  const rows = data?.byRoom ?? []
  const max = Math.max(1, ...rows.map((r) => r.revenue))

  return (
    <div className="space-y-3">
      <Tabs value={preset} onChange={setPreset} items={presets.map((x) => ({ value: x.key, label: x.label }))} />
      <Card padded={false} title={data ? `הכנסות ${money(data.totals.revenue)} · נגבה ${money(data.totals.paid)}` : 'פילוח לפי אולם'}>
        {isLoading ? (
          <PageLoader />
        ) : rows.length === 0 ? (
          <Empty icon={<Building2 className="h-6 w-6" />} title="אין הזמנות מאושרות בתקופה" />
        ) : (
          <ul className="divide-y divide-line">
            {rows.map((r) => (
              <li key={r.key} className="px-4 py-3">
                <div className="flex items-baseline justify-between gap-3">
                  <p className="min-w-0 truncate font-medium">{r.label}</p>
                  <p className="shrink-0 font-semibold tabular-nums">{money(r.revenue)}</p>
                </div>
                <div className="mt-1.5 h-2 overflow-hidden rounded-full bg-subtle">
                  <div className="h-full rounded-full bg-brand" style={{ width: `${(r.revenue / max) * 100}%` }} />
                </div>
                <p className="mt-1 flex flex-wrap gap-x-3 text-xs text-muted">
                  <span>{r.bookings} הזמנות</span>
                  <span>{r.hours.toFixed(1)} שעות</span>
                  <span>נגבה {money(r.paid)}</span>
                  {r.revenue > r.paid && <span className="text-danger">יתרה {money(r.revenue - r.paid)}</span>}
                </p>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  )
}

function PaymentsList({ canDelete }: { canDelete: boolean }) {
  const qc = useQueryClient()
  const [method, setMethod] = useState<PaymentMethod | ''>('')
  const [remove, setRemove] = useState<Payment | null>(null)
  const { data = [], isLoading } = useQuery({ queryKey: ['payments', 'list', method], queryFn: () => api.get<Payment[]>('/payments', { method: method || undefined }) })
  const del = useMutation({
    mutationFn: (id: string) => api.del(`/payments/${id}`),
    onSuccess: () => {
      toast.success('התשלום נמחק ונרשם ביומן הביקורת')
      for (const k of ['payments', 'balances', 'bookings', 'dashboard']) qc.invalidateQueries({ queryKey: [k] })
      setRemove(null)
    },
    onError: (e) => toast.error(errorMessage(e)),
  })

  const exportCsv = () => {
    const rows = [['תאריך', 'הזמנה', 'לקוח', 'אמצעי', 'אסמכתא', 'סכום'], ...data.map((p) => [dateTime(p.paidAt), p.bookingTitle, p.customerName ?? '', paymentMethodLabel[p.method], p.reference, String(p.amount)])]
    const csv = '﻿' + rows.map((r) => r.map((c) => `"${c.replace(/"/g, '""')}"`).join(',')).join('\n')
    const a = document.createElement('a')
    a.href = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }))
    a.download = `payments-${new Date().toISOString().slice(0, 10)}.csv`
    a.click()
  }

  if (isLoading) return <PageLoader />
  return (
    <Card
      padded={false}
      title={`${data.length} תשלומים · ${money(data.reduce((s, p) => s + p.amount, 0))}`}
      actions={
        <>
          <select className="input h-8 w-40 text-xs" value={method} onChange={(e) => setMethod(e.target.value as PaymentMethod | '')}>
            <option value="">כל האמצעים</option>
            {(Object.keys(paymentMethodLabel) as PaymentMethod[]).map((m) => (
              <option key={m} value={m}>
                {paymentMethodLabel[m]}
              </option>
            ))}
          </select>
          <Button size="sm" variant="secondary" icon={<Download className="h-3.5 w-3.5" />} onClick={exportCsv}>
            CSV
          </Button>
        </>
      }
    >
      {data.length === 0 ? (
        <Empty title="אין תשלומים" />
      ) : (
        <div className="scroll-thin overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-subtle/70 text-xs text-muted">
              <tr className="[&>th]:px-4 [&>th]:py-2.5 [&>th]:text-start [&>th]:font-medium">
                <th>תאריך</th>
                <th>הזמנה</th>
                <th>לקוח</th>
                <th>אמצעי</th>
                <th>אסמכתא</th>
                <th className="text-end">סכום</th>
                <th />
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {data.map((p) => (
                <tr key={p.id} className="[&>td]:px-4 [&>td]:py-2.5">
                  <td className="whitespace-nowrap tabular-nums">{dateTime(p.paidAt)}</td>
                  <td className="max-w-[220px] truncate">{p.bookingTitle}</td>
                  <td className="text-muted">{p.customerName || '—'}</td>
                  <td>
                    <span className="flex items-center gap-1.5">
                      {methodIcon[p.method]} {paymentMethodLabel[p.method]}
                    </span>
                  </td>
                  <td className="text-muted">{p.reference || '—'}</td>
                  <td className="text-end font-semibold tabular-nums">{money(p.amount, true)}</td>
                  <td>
                    {canDelete && (
                      <IconButton label="מחיקה" onClick={() => setRemove(p)}>
                        <Trash2 className="h-4 w-4" />
                      </IconButton>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <Confirm
        open={!!remove}
        onClose={() => setRemove(null)}
        onConfirm={() => remove && del.mutate(remove.id)}
        loading={del.isPending}
        danger
        title="מחיקת תשלום"
        message={remove && `למחוק תשלום של ${money(remove.amount)} עבור "${remove.bookingTitle}"? הפעולה תירשם ביומן הביקורת.`}
        confirmLabel="מחיקה"
      />
    </Card>
  )
}

const actionLabel: Record<string, string> = {
  create: 'נוצר',
  update: 'עודכן',
  delete: 'נמחק',
  cancel: 'בוטל',
  approve: 'אושר',
  reject: 'נדחה',
  submit: 'הוגש',
  save: 'נשמר',
  attach: 'צורף קובץ',
}
const entityLabel: Record<string, string> = { payment: 'תשלום', booking: 'הזמנה', request: 'בקשה', room: 'חדר', user: 'משתמש', customer: 'לקוח', task: 'משימה' }

function Audit() {
  const { data = [], isLoading } = useQuery({ queryKey: ['audit'], queryFn: () => api.get<AuditEntry[]>('/audit', { limit: 200 }) })
  if (isLoading) return <PageLoader />
  return (
    <Card padded={false} title="200 הפעולות האחרונות">
      {data.length === 0 ? (
        <Empty title="אין רשומות" />
      ) : (
        <ul className="divide-y divide-line">
          {data.map((a) => (
            <li key={a.id} className="flex items-start gap-3 px-5 py-3 text-sm">
              <Badge tone={a.entity === 'payment' ? 'green' : a.action === 'delete' ? 'red' : 'gray'}>{entityLabel[a.entity] ?? a.entity}</Badge>
              <div className="min-w-0 flex-1">
                <p>
                  <b>{a.actorName ?? 'מערכת'}</b> · {actionLabel[a.action] ?? a.action}
                  {typeof a.data?.title === 'string' && ` · ${a.data.title}`}
                  {typeof a.data?.amount === 'number' && ` · ${money(a.data.amount as number)}`}
                </p>
                <p className="text-xs text-muted">{dateTime(a.createdAt)}</p>
              </div>
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}
