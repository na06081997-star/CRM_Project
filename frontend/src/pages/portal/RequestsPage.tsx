import clsx from 'clsx'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { History, Plus } from 'lucide-react'
import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Attachments } from '@/components/Attachments'
import { CancelDialog } from '@/components/CancelDialog'
import { toast } from '@/components/toast'
import { Badge, Button, Empty, PageHeader, PageLoader } from '@/components/ui'
import { api, errorMessage } from '@/lib/api'
import { dateTime, fmt, money, timeRange } from '@/lib/format'
import { paymentMethodLabel, requestRouteLabel, requestStatusLabel } from '@/lib/labels'
import { useLookups } from '@/lib/queries'
import type { BookingRequest } from '@/lib/types'
import { useMyRequests } from './PortalLayout'

type Tone = 'amber' | 'green' | 'red' | 'gray'
export function requestTone(r: BookingRequest): Tone {
  if (r.bookingStatus === 'cancelled') return 'gray'
  return ({ pending: 'amber', approved: 'green', rejected: 'red', cancelled: 'gray' } as const)[r.status]
}
const statusText = (r: BookingRequest) => (r.bookingStatus === 'cancelled' ? 'בוטלה' : requestStatusLabel[r.status] === 'אושרה' ? 'מאושרת' : requestStatusLabel[r.status])

/** Mockup portal screen 5: "הבקשות שלי — בלי לרדוף אחרי תשובות". */
export function RequestsPage() {
  const [params, setParams] = useSearchParams()
  const { data, isLoading } = useMyRequests()
  const [filter, setFilter] = useState<'all' | 'pending' | 'approved' | 'rejected' | 'cancelled'>('all')
  const list = (data ?? []).filter((r) => filter === 'all' || r.status === filter)
  const selected = list.find((r) => r.id === params.get('id')) ?? list[0]

  return (
    <div>
      <PageHeader
        title="הבקשות שלי"
        subtitle="מוצגות רק בקשות השייכות לחשבון שלך · סטטוס, פרטי אישור, תשלום ובקשת ביטול"
        actions={
          <Link to="/portal/new">
            <Button icon={<Plus className="h-4 w-4" />}>בקשה חדשה</Button>
          </Link>
        }
      />
      <div className="mb-4 flex flex-wrap gap-2">
        {(['all', 'pending', 'approved', 'rejected', 'cancelled'] as const).map((s) => (
          <button
            key={s}
            onClick={() => setFilter(s)}
            className={clsx('rounded-full border px-3.5 py-1.5 text-sm transition', filter === s ? 'border-ink bg-ink text-surface' : 'border-line bg-surface hover:bg-subtle')}
          >
            {s === 'all' ? 'הכל' : requestStatusLabel[s]}
            <span className="ms-1.5 text-xs opacity-70">{(data ?? []).filter((r) => s === 'all' || r.status === s).length}</span>
          </button>
        ))}
      </div>

      {isLoading ? (
        <PageLoader />
      ) : list.length === 0 ? (
        <div className="card">
          <Empty
            icon={<History className="h-6 w-6" />}
            title="אין בקשות להצגה"
            hint="חפשו חלל פנוי ושלחו בקשה – היא תופיע כאן עם סטטוס בזמן אמת."
            action={
              <Link to="/portal/search">
                <Button size="sm">חיפוש חלל</Button>
              </Link>
            }
          />
        </div>
      ) : (
        <div className="grid items-start gap-5 xl:grid-cols-[1.25fr_1fr]">
          <div className="space-y-4">
            <div className="card overflow-hidden">
              <table className="w-full text-sm">
                <thead className="bg-subtle/70 text-xs text-muted">
                  <tr className="[&>th]:px-4 [&>th]:py-2.5 [&>th]:text-start [&>th]:font-medium">
                    <th>בקשה / חלל</th>
                    <th>מועד</th>
                    <th>מצב</th>
                    <th />
                  </tr>
                </thead>
                <tbody className="divide-y divide-line">
                  {list.map((r) => (
                    <tr
                      key={r.id}
                      onClick={() => setParams({ id: r.id })}
                      className={clsx('cursor-pointer transition [&>td]:px-4 [&>td]:py-3', selected?.id === r.id ? 'bg-brand-soft/50' : 'hover:bg-subtle')}
                    >
                      <td>
                        <p className="font-semibold">#{r.number}</p>
                        <p className="text-xs text-muted">
                          {r.roomName} · {r.title}
                        </p>
                      </td>
                      <td className="whitespace-nowrap tabular-nums">
                        <p>{fmt(r.startsAt, 'dd.MM.yyyy')}</p>
                        <p className="text-xs text-muted">{timeRange(r.startsAt, r.endsAt)}</p>
                      </td>
                      <td>
                        <Badge tone={requestTone(r)}>{statusText(r)}</Badge>
                        {r.cancelRequested && <p className="mt-1 text-[11px] text-danger">ביטול ממתין לאישור</p>}
                      </td>
                      <td className="text-end text-xs font-medium text-brand">{r.status === 'rejected' ? 'נימוק' : 'פרטים'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="card p-4 text-sm">
              <h3 className="font-semibold">כשבקשה נדחית</h3>
              <p className="mt-1 text-muted">מוצג נימוק המנהל המיועד למבקש, עם קישור לחיפוש מועד או חלל אחר. לא מוצגים פרטי ההזמנה המתנגשת.</p>
            </div>
            <p className="rounded-xl border-s-4 border-brand bg-subtle px-4 py-3 text-xs text-muted">אין שינוי מועד להזמנה מאושרת בשירות עצמי. בקשה לשינוי נשלחת למתנ״ס; שינוי מתבצע רק אחרי בדיקת זמינות מחדש.</p>
          </div>
          {selected && <RequestDetail r={selected} />}
        </div>
      )}
    </div>
  )
}

function RequestDetail({ r }: { r: BookingRequest }) {
  const qc = useQueryClient()
  const { equipmentLabel } = useLookups()
  const [cancelOpen, setCancelOpen] = useState(false)
  const withdraw = useMutation({
    mutationFn: () => api.post(`/requests/${r.id}/cancel`),
    onSuccess: () => {
      toast.success('הבקשה בוטלה')
      qc.invalidateQueries({ queryKey: ['requests'] })
      qc.invalidateQueries({ queryKey: ['blocks'] })
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  const approved = r.status === 'approved' && r.bookingStatus !== 'cancelled'
  const price = r.finalPrice ?? r.bookingPrice
  const paid = price != null && price > 0 ? r.paidAmount >= price : null
  const timeline = [
    { at: r.createdAt, text: 'הבקשה נקלטה' },
    r.reviewedAt && { at: r.reviewedAt, text: r.status === 'approved' ? 'אושרה על ידי המתנ״ס' : r.status === 'rejected' ? 'נדחתה על ידי המתנ״ס' : 'טופלה' },
    r.paidAt && { at: r.paidAt, text: paid ? 'התשלום נרשם במלואו' : 'נרשם תשלום חלקי' },
    r.bookingStatus === 'cancelled' && { at: null, text: 'ההזמנה בוטלה' },
  ].filter(Boolean) as { at: string | null; text: string }[]

  return (
    <section className="card sticky top-4 p-5">
      <div className="flex items-center justify-between gap-2">
        <h2 className="font-semibold">פרטי הזמנה #{r.number}</h2>
        <Badge tone={requestTone(r)}>{statusText(r)}</Badge>
      </div>
      <p className="mt-3 text-2xl font-bold">{r.roomName}</p>
      <p className="mt-1 text-sm">
        {fmt(r.startsAt, 'dd.MM.yyyy')} · {timeRange(r.startsAt, r.endsAt)}
      </p>
      <p className="mt-1 text-xs text-muted">
        {r.eventKind || r.title} · {requestRouteLabel[r.route]}
        {r.equipment.length > 0 && ` · ${r.equipment.map(equipmentLabel).join(', ')}`}
        {(r.tablesCount > 0 || r.chairsCount > 0) && ` · ${r.tablesCount} שולחנות / ${r.chairsCount} כיסאות`}
      </p>

      {r.status === 'rejected' && (
        <div className="mt-4 rounded-xl bg-red-50 p-3 text-sm text-red-800 dark:bg-red-950/30 dark:text-red-200">
          <b>נימוק:</b> {r.rejectionReason}
          <Link to="/portal/search" className="mt-2 block text-xs font-medium underline">
            חיפוש מועד או חלל אחר
          </Link>
        </div>
      )}

      {r.route === 'rental' && (
        <dl className="mt-4 space-y-2 border-t border-line pt-4 text-sm">
          <div className="flex items-center justify-between">
            <dt className="text-muted">{approved ? 'מחיר מאושר' : 'הערכת מחיר'}</dt>
            <dd className="text-lg font-bold">{approved && price != null ? money(price) : r.quotedPrice ? `≈ ${money(r.quotedPrice)}` : 'לאחר בדיקה'}</dd>
          </div>
          {approved && (
            <>
              <div className="flex items-center justify-between">
                <dt className="text-muted">שולם?</dt>
                <dd>{paid === null ? '—' : <Badge tone={paid ? 'green' : 'amber'}>{paid ? 'כן' : r.paidAmount > 0 ? `חלקית (${money(r.paidAmount)})` : 'לא'}</Badge>}</dd>
              </div>
              <div className="flex items-center justify-between">
                <dt className="text-muted">אמצעי תשלום</dt>
                <dd className="font-semibold">{r.paymentMethod ? paymentMethodLabel[r.paymentMethod] : 'טרם נרשם'}</dd>
              </div>
              <p className="text-xs text-muted">התשלום מתואם מול המתנ״ס: העברה בנקאית, צ׳ק או מזומן. אין תשלום מקוון.</p>
            </>
          )}
        </dl>
      )}

      <ol className="mt-4 space-y-2 border-s-2 border-line ps-4 text-sm">
        {timeline.map((t, i) => (
          <li key={i} className="relative">
            <span className="absolute -start-[21px] top-1.5 h-2.5 w-2.5 rounded-full bg-brand" />
            {t.at && <span className="me-2 font-semibold tabular-nums">{dateTime(t.at)}</span>}
            {t.text}
          </li>
        ))}
      </ol>

      <div className="mt-5 flex flex-wrap gap-2">
        {approved && !r.cancelRequested && r.bookingId && (
          <Button variant="secondary" className="text-danger" onClick={() => setCancelOpen(true)}>
            בקשת ביטול
          </Button>
        )}
        {r.cancelRequested && <Badge tone="red">בקשת ביטול ממתינה לאישור</Badge>}
        {r.status === 'pending' && (
          <Button variant="secondary" className="text-danger" loading={withdraw.isPending} onClick={() => withdraw.mutate()}>
            ביטול הבקשה
          </Button>
        )}
        <Link to="/portal/help">
          <Button variant="ghost">יצירת קשר</Button>
        </Link>
      </div>
      {approved && <p className="mt-2 text-xs text-muted">בקשת ביטול אינה מבטלת מיד. עד לקבלת אישור ביטול ההזמנה ממשיכה לחסום את החלל.</p>}

      <div className="mt-5 border-t border-line pt-4">
        <Attachments entity="request" entityId={r.id} canDelete={false} />
      </div>

      <CancelDialog
        block={cancelOpen && r.bookingId ? { id: r.bookingId, title: r.title, startsAt: r.startsAt, endsAt: r.endsAt, roomName: r.roomName, cancelRequested: false } : null}
        onClose={() => setCancelOpen(false)}
      />
    </section>
  )
}
