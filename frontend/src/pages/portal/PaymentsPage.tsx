import { useQuery } from '@tanstack/react-query'
import { Banknote, FileClock, Wallet } from 'lucide-react'
import { Card, Empty, PageHeader, PageLoader, Stat } from '@/components/ui'
import { api } from '@/lib/api'
import { dateShort, dateTime, money, timeRange } from '@/lib/format'
import { paymentMethodLabel } from '@/lib/labels'
import type { Payment } from '@/lib/types'
import { useMyBalances } from './PortalLayout'

/** Own payment status: what is due and what was recorded by the matnas. */
export function PaymentsPage() {
  const balances = useMyBalances()
  const payments = useQuery({ queryKey: ['payments', 'mine'], queryFn: () => api.get<Payment[]>('/payments') })
  if (balances.isLoading || payments.isLoading) return <PageLoader />
  const due = (balances.data ?? []).reduce((s, b) => s + b.price - b.paidAmount, 0)
  const paid = (payments.data ?? []).reduce((s, p) => s + p.amount, 0)

  return (
    <div className="space-y-6">
      <PageHeader title="תשלומים" subtitle="מצב התשלום של ההזמנות שלך בלבד. התשלומים נרשמים על ידי המתנ״ס – אין תשלום מקוון." />
      <div className="grid gap-4 sm:grid-cols-3">
        <Stat label="יתרה לתשלום" value={money(due)} tone={due > 0 ? 'red' : 'teal'} icon={<FileClock className="h-5 w-5" />} hint={`${balances.data?.length ?? 0} הזמנות פתוחות`} />
        <Stat label="שולם עד כה" value={money(paid)} tone="green" icon={<Banknote className="h-5 w-5" />} hint={`${payments.data?.length ?? 0} תשלומים`} />
        <Stat label="אמצעי תשלום" value="העברה · צ׳ק · מזומן" tone="blue" icon={<Wallet className="h-5 w-5" />} hint="בתיאום מול המזכירות" />
      </div>

      <Card title="הזמנות עם יתרה" padded={false}>
        {(balances.data ?? []).length === 0 ? (
          <Empty title="אין יתרות פתוחות 🎉" />
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-subtle/70 text-xs text-muted">
              <tr className="[&>th]:px-4 [&>th]:py-2.5 [&>th]:text-start [&>th]:font-medium">
                <th>הזמנה</th>
                <th>מועד</th>
                <th className="text-end">מחיר</th>
                <th className="text-end">שולם</th>
                <th className="text-end">יתרה</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {(balances.data ?? []).map((b) => (
                <tr key={b.id} className="[&>td]:px-4 [&>td]:py-3">
                  <td>
                    <p className="font-medium">{b.title}</p>
                    <p className="text-xs text-muted">{b.roomName}</p>
                  </td>
                  <td className="whitespace-nowrap tabular-nums text-muted">
                    {dateShort(b.startsAt)} {timeRange(b.startsAt, b.endsAt)}
                  </td>
                  <td className="text-end tabular-nums">{money(b.price)}</td>
                  <td className="text-end tabular-nums">{money(b.paidAmount)}</td>
                  <td className="text-end font-semibold tabular-nums text-danger">{money(b.price - b.paidAmount)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>

      <Card title="תשלומים שנרשמו" padded={false}>
        {(payments.data ?? []).length === 0 ? (
          <Empty title="עדיין לא נרשמו תשלומים" />
        ) : (
          <ul className="divide-y divide-line">
            {(payments.data ?? []).map((p) => (
              <li key={p.id} className="flex items-center justify-between gap-3 px-5 py-3 text-sm">
                <span>
                  <span className="block font-medium">{p.bookingTitle}</span>
                  <span className="block text-xs text-muted">
                    {dateTime(p.paidAt)} · {paymentMethodLabel[p.method]}
                    {p.reference && ` · ${p.reference}`}
                  </span>
                </span>
                <span className="font-semibold tabular-nums">{money(p.amount, true)}</span>
              </li>
            ))}
          </ul>
        )}
      </Card>
      <p className="text-xs text-muted">רק בעל הרשאת גבייה במתנ״ס מעדכן מחיר ותשלום. כל שינוי מתועד. לא ניתן לסמן לעצמך ״שולם״.</p>
    </div>
  )
}
