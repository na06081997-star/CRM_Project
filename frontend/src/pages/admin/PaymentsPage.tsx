import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Banknote, CreditCard, Download, FileClock, Landmark, ReceiptText, Trash2, Wallet } from 'lucide-react'
import { useState } from 'react'
import { PaymentDialog } from '@/components/PaymentDialog'
import { toast } from '@/components/toast'
import { Badge, Button, Card, Confirm, Empty, IconButton, PageHeader, PageLoader, Stat, Tabs } from '@/components/ui'
import { api, errorMessage } from '@/lib/api'
import { useCan } from '@/lib/auth'
import { dateShort, dateTime, money, timeRange } from '@/lib/format'
import { paymentMethodLabel } from '@/lib/labels'
import type { AuditEntry, Booking, Payment, PaymentMethod, PaymentSummary } from '@/lib/types'

const methodIcon: Record<PaymentMethod, JSX.Element> = {
  bank_transfer: <Landmark className="h-4 w-4" />,
  check: <ReceiptText className="h-4 w-4" />,
  cash: <Banknote className="h-4 w-4" />,
  credit_card: <CreditCard className="h-4 w-4" />,
}

export function PaymentsPage() {
  const can = useCan()
  const [tab, setTab] = useState<'balances' | 'payments' | 'audit'>('balances')
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
        className="mb-4"
        value={tab}
        onChange={setTab}
        items={[
          { value: 'balances', label: 'יתרות פתוחות' },
          { value: 'payments', label: 'תשלומים שהתקבלו' },
          { value: 'audit', label: 'יומן ביקורת' },
        ]}
      />
      {tab === 'balances' && <Balances canPay={can('payments:manage')} />}
      {tab === 'payments' && <PaymentsList canDelete={can('payments:manage')} />}
      {tab === 'audit' && <Audit />}
    </div>
  )
}

function Balances({ canPay }: { canPay: boolean }) {
  const [paying, setPaying] = useState<Booking | null>(null)
  const { data = [], isLoading } = useQuery({ queryKey: ['balances'], queryFn: () => api.get<Booking[]>('/payments/balances') })
  if (isLoading) return <PageLoader />
  const total = data.reduce((s, b) => s + b.price - b.paidAmount, 0)
  return (
    <Card padded={false} title={`${data.length} הזמנות עם יתרה · ${money(total)}`}>
      {data.length === 0 ? (
        <Empty title="כל ההזמנות שולמו במלואן 🎉" />
      ) : (
        <div className="scroll-thin overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-subtle/70 text-xs text-muted">
              <tr className="[&>th]:px-4 [&>th]:py-2.5 [&>th]:text-start [&>th]:font-medium">
                <th>לקוח</th>
                <th>הזמנה</th>
                <th>מועד</th>
                <th className="text-end">מחיר</th>
                <th className="text-end">שולם</th>
                <th className="text-end">יתרה</th>
                <th>התקדמות</th>
                <th />
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {data.map((b) => {
                const pct = b.price ? Math.round((b.paidAmount / b.price) * 100) : 0
                const overdue = new Date(b.startsAt).getTime() < Date.now()
                return (
                  <tr key={b.id} className="[&>td]:px-4 [&>td]:py-3">
                    <td className="font-medium">{b.customerName || '—'}</td>
                    <td className="max-w-[220px] truncate">{b.title}</td>
                    <td className="whitespace-nowrap tabular-nums text-muted">
                      {dateShort(b.startsAt)} {timeRange(b.startsAt, b.endsAt)} {overdue && <Badge tone="red">עבר</Badge>}
                    </td>
                    <td className="text-end tabular-nums">{money(b.price)}</td>
                    <td className="text-end tabular-nums">{money(b.paidAmount)}</td>
                    <td className="text-end font-semibold tabular-nums text-danger">{money(b.price - b.paidAmount)}</td>
                    <td className="w-32">
                      <div className="h-1.5 overflow-hidden rounded-full bg-subtle">
                        <div className={clsx('h-full rounded-full', pct >= 50 ? 'bg-ok' : 'bg-warn')} style={{ width: `${pct}%` }} />
                      </div>
                      <span className="text-[10px] text-muted">{pct}%</span>
                    </td>
                    <td>
                      {canPay && (
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
      )}
      <PaymentDialog booking={paying} open={!!paying} onClose={() => setPaying(null)} />
    </Card>
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
