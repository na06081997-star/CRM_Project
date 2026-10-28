import { useQuery } from '@tanstack/react-query'
import { addMonths, startOfMonth } from 'date-fns'
import { Printer } from 'lucide-react'
import { useState } from 'react'
import { Button, Card, Field, Input, PageHeader, PageLoader, Stat } from '@/components/ui'
import { api } from '@/lib/api'
import { fmt, money, num } from '@/lib/format'
import { bookingTypeLabel, paymentMethodLabel } from '@/lib/labels'
import type { BookingType, PaymentMethod, Report, ReportRow } from '@/lib/types'

export function ReportsPage() {
  const [from, setFrom] = useState(fmt(addMonths(startOfMonth(new Date()), -5), 'yyyy-MM-dd'))
  const [to, setTo] = useState(fmt(addMonths(startOfMonth(new Date()), 1), 'yyyy-MM-dd'))
  const { data, isLoading } = useQuery({ queryKey: ['reports', from, to], queryFn: () => api.get<Report>('/reports', { from, to }) })

  return (
    <div>
      <PageHeader
        title="דוחות"
        subtitle="הכנסות, שעות פעילות ותפוסה לפי חודש, חדר וסוג"
        actions={
          <>
            <Field label="מתאריך">
              <Input type="date" value={from} onChange={(e) => setFrom(e.target.value)} />
            </Field>
            <Field label="עד תאריך">
              <Input type="date" value={to} onChange={(e) => setTo(e.target.value)} />
            </Field>
            <Button variant="secondary" className="self-end" icon={<Printer className="h-4 w-4" />} onClick={() => window.print()}>
              הדפסה
            </Button>
          </>
        }
      />
      {isLoading || !data ? (
        <PageLoader />
      ) : (
        <div className="space-y-6">
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <Stat label="הזמנות מאושרות" value={num(data.totals.bookings)} />
            <Stat label="שעות פעילות" value={num(Math.round(data.totals.hours))} tone="blue" />
            <Stat label="הכנסות (חיובים)" value={money(data.totals.revenue)} tone="green" />
            <Stat label="נגבה בפועל" value={money(data.totals.paid)} tone="violet" hint={data.totals.revenue ? `${Math.round((data.totals.paid / data.totals.revenue) * 100)}% גבייה` : undefined} />
          </div>
          <Card title="הכנסות ושעות לפי חודש">
            <MonthChart rows={data.byMonth} />
          </Card>
          <div className="grid gap-6 lg:grid-cols-2">
            <Card title="לפי חדר" padded={false}>
              <RowsTable rows={data.byRoom} />
            </Card>
            <div className="space-y-6">
              <Card title="לפי סוג הזמנה" padded={false}>
                <RowsTable rows={data.byType.map((r) => ({ ...r, label: bookingTypeLabel[r.key as BookingType] ?? r.label }))} />
              </Card>
              <Card title="גבייה לפי אמצעי תשלום" padded={false}>
                <ul className="divide-y divide-line">
                  {data.paymentsByMethod.map((r) => (
                    <li key={r.key} className="flex justify-between px-5 py-2.5 text-sm">
                      <span>
                        {paymentMethodLabel[r.key as PaymentMethod] ?? r.key} <span className="text-xs text-muted">({r.bookings})</span>
                      </span>
                      <span className="font-semibold tabular-nums">{money(r.paid)}</span>
                    </li>
                  ))}
                  {data.paymentsByMethod.length === 0 && <li className="px-5 py-4 text-sm text-muted">אין תשלומים בטווח</li>}
                </ul>
              </Card>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function MonthChart({ rows }: { rows: ReportRow[] }) {
  const max = Math.max(1, ...rows.map((r) => r.revenue))
  const maxH = Math.max(1, ...rows.map((r) => r.hours))
  if (!rows.length) return <p className="text-sm text-muted">אין נתונים בטווח שנבחר</p>
  return (
    <div className="flex h-56 gap-4">
      {rows.map((r) => (
        <div key={r.key} className="flex flex-1 flex-col items-center gap-1.5">
          <span className="text-[11px] font-semibold tabular-nums">{money(r.revenue)}</span>
          <div className="flex w-full flex-1 items-end justify-center gap-1">
            <div className="w-1/2 max-w-10 rounded-t-lg bg-brand" style={{ height: `${(r.revenue / max) * 100}%`, minHeight: 3 }} title={`הכנסות ${money(r.revenue)}`} />
            <div className="w-1/3 max-w-6 rounded-t-lg bg-violet-400/70" style={{ height: `${(r.hours / maxH) * 100}%`, minHeight: 3 }} title={`${r.hours.toFixed(1)} שעות`} />
          </div>
          <span className="text-xs text-muted">{fmt(new Date(`${r.key}-01T12:00:00`), 'MMM yy')}</span>
        </div>
      ))}
    </div>
  )
}

function RowsTable({ rows }: { rows: ReportRow[] }) {
  return (
    <table className="w-full text-sm">
      <thead className="bg-subtle/70 text-xs text-muted">
        <tr className="[&>th]:px-4 [&>th]:py-2 [&>th]:text-start [&>th]:font-medium">
          <th>שם</th>
          <th className="text-end">הזמנות</th>
          <th className="text-end">שעות</th>
          <th className="text-end">הכנסות</th>
        </tr>
      </thead>
      <tbody className="divide-y divide-line">
        {rows.map((r) => (
          <tr key={r.key} className="[&>td]:px-4 [&>td]:py-2.5">
            <td className="font-medium">{r.label}</td>
            <td className="text-end tabular-nums">{r.bookings}</td>
            <td className="text-end tabular-nums">{r.hours.toFixed(1)}</td>
            <td className="text-end tabular-nums">{money(r.revenue)}</td>
          </tr>
        ))}
        {rows.length === 0 && (
          <tr>
            <td colSpan={4} className="px-4 py-4 text-muted">
              אין נתונים
            </td>
          </tr>
        )}
      </tbody>
    </table>
  )
}
