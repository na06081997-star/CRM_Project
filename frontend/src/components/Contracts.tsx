import clsx from 'clsx'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Copy, FileSignature, FileText, MessageCircle, Send } from 'lucide-react'
import { useState } from 'react'
import { Attachments } from '@/components/Attachments'
import { PhotoGallery } from '@/components/Photos'
import { toast } from '@/components/toast'
import { Badge, Button, Field, Input, Modal, Textarea } from '@/components/ui'
import { api, ApiError, errorMessage } from '@/lib/api'
import { useOrg } from '@/lib/brand'
import { dateTime } from '@/lib/format'
import { contractStatusLabel } from '@/lib/labels'
import type { Contract, Customer, User } from '@/lib/types'

export interface ContractResult {
  user: User
  contract: Contract | null
  signUrl?: string
  sentVia?: string[]
}

export function coordinatorTemplate(org: string, name: string) {
  return `הסכם התקשרות – רכז/ת פעילות ב${org}

1. ${name || 'הרכז/ת'} ירכז/תרכז פעילויות קהילתיות בהתאם לתוכנית העבודה שתיקבע עם הנהלת המתנ״ס.
2. הזמנת חללים תיעשה דרך המערכת בלבד ובכפוף לאישור המתנ״ס.
3. הרכז/ת אחראי/ת לשמירה על החללים והציוד, ולדיווח על כל תקלה דרך המערכת.
4. הרכז/ת ישמור/תשמור על נהלי הבטיחות, הניקיון ושעות הפעילות של המתנ״ס.
5. הרכז/ת ישמור/תשמור על פרטיות המשתתפים ועל סודיות המידע שיגיע לידיעתו/ה.
6. כל צד רשאי לסיים את ההתקשרות בהודעה מוקדמת בכתב.`
}

export function customerTemplate(org: string, name: string) {
  return `הסכם שימוש בחללי ${org}

1. ${name || 'המזמין/ה'} (להלן: "המזמין/ה") ישתמש/תשתמש בחללי המתנ״ס בהתאם להזמנות שיאושרו במערכת.
2. התשלום יבוצע לפי המחיר והתנאים שנמסרו בהזמנה, ולפני מועד הפעילות אלא אם סוכם אחרת.
3. ביטול הזמנה ייעשה דרך המערכת או בהודעה למתנ״ס, בהתאם למדיניות הביטולים של המתנ״ס.
4. המזמין/ה אחראי/ת לשמירה על החלל והציוד ולהחזרתם במצב שבו התקבלו; נזק שייגרם יחויב.
5. יש לשמור על נהלי הבטיחות, הניקיון, שעות הפעילות ומספר המשתתפים המותר בחלל.
6. החתימה פותחת למזמין/ה חשבון במערכת לצפייה בהזמנות ובתשלומים ולהגשת בקשות חדשות.`
}

const channelLabel: Record<string, string> = { sms019: 'SMS', twilio: 'SMS', email: 'אימייל', telegram: 'טלגרם' }

/** WhatsApp link with the message ready (to the person's number when known). */
function whatsappUrl(text: string, phone?: string | null) {
  const digits = (phone ?? '').replace(/\D/g, '')
  const intl = digits.startsWith('0') ? '972' + digits.slice(1) : digits
  return `https://wa.me/${intl}?text=${encodeURIComponent(text)}`
}

/** Signing link: sent automatically when a channel is configured, otherwise shared by hand. */
export function SignLink({ url, sentVia = [], phone, name }: { url: string; sentVia?: string[]; phone?: string | null; name?: string }) {
  const via = [...new Set(sentVia.map((c) => channelLabel[c] ?? c))]
  const message = `שלום${name ? ' ' + name : ''}, זה הקישור לחתימה על החוזה מול המתנ״ס: ${url}`
  return (
    <div className="rounded-xl border border-ok/30 bg-emerald-50 p-3 text-sm dark:bg-emerald-950/20">
      <p className="font-semibold text-ok">{via.length ? `הקישור לחתימה נשלח (${via.join(' + ')})` : 'הקישור לחתימה מוכן'}</p>
      {!via.length && <p className="mt-0.5 text-xs text-muted">שליחה אוטומטית ב־SMS / אימייל עדיין לא הוגדרה – שלחו את הקישור בוואטסאפ או העתיקו אותו.</p>}
      <div className="mt-2 flex items-center gap-2">
        <code dir="ltr" className="min-w-0 flex-1 truncate rounded-lg bg-surface px-2 py-1.5 text-xs">
          {url}
        </code>
        <Button
          size="sm"
          variant="secondary"
          icon={<Copy className="h-3.5 w-3.5" />}
          onClick={() => navigator.clipboard.writeText(url).then(() => toast.success('הקישור הועתק'), () => toast.error('לא ניתן להעתיק'))}
        >
          העתקה
        </Button>
      </div>
      <a href={whatsappUrl(message, phone)} target="_blank" rel="noreferrer" className="mt-2 block">
        <Button size="sm" className="w-full bg-[#25d366] text-white hover:bg-[#1ebe5b]" icon={<MessageCircle className="h-4 w-4" />}>
          שליחה בוואטסאפ
        </Button>
      </a>
      <p className="mt-2 text-xs text-muted">החשבון ייפתח אוטומטית מיד לאחר החתימה. הקישור בתוקף 14 יום.</p>
    </div>
  )
}

export function ContractFields({ title, body, onTitle, onBody, errors }: { title: string; body: string; onTitle: (v: string) => void; onBody: (v: string) => void; errors: Record<string, string> }) {
  return (
    <>
      <Field label="כותרת החוזה">
        <Input value={title} onChange={(e) => onTitle(e.target.value)} />
      </Field>
      <Field label="נוסח החוזה" error={errors.body} hint="נוסח לדוגמה – ערכו לפי ההסכם של המתנ״ס">
        <Textarea value={body} onChange={(e) => onBody(e.target.value)} rows={8} />
      </Field>
    </>
  )
}

/** Who receives the contract: an existing user (רכז) or a customer (renter). */
export type ContractTarget = { kind: 'user'; user: User } | { kind: 'customer'; customer: Customer }

export function SendContractDialog({ target, onClose }: { target: ContractTarget | null; onClose: () => void }) {
  const qc = useQueryClient()
  const org = useOrg()
  const isCustomer = target?.kind === 'customer'
  const name = target ? (target.kind === 'user' ? target.user.fullName : target.customer.contactName || target.customer.name) : ''
  const phone = target ? (target.kind === 'user' ? target.user.phone : target.customer.phone) : null
  const email = target ? (target.kind === 'user' ? target.user.email : target.customer.email) : null
  const [method, setMethod] = useState<'digital' | 'scan'>('digital')
  const [title, setTitle] = useState<string | null>(null)
  const [body, setBody] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [result, setResult] = useState<ContractResult | null>(null)
  const orgName = org?.name ?? 'המתנ״ס'
  const heading = title ?? (isCustomer ? `הסכם שימוש בחללי ${orgName}` : 'הסכם התקשרות – רכז/ת')
  const text = body || (isCustomer ? customerTemplate(orgName, name) : coordinatorTemplate(orgName, name))
  const close = () => {
    setResult(null)
    setBody('')
    setTitle(null)
    setErrors({})
    onClose()
  }
  const send = useMutation({
    mutationFn: () =>
      target!.kind === 'user'
        ? api.post<ContractResult>('/contracts', { userId: target!.user.id, method, title: heading, body: text })
        : api.post<ContractResult>(`/customers/${target!.customer.id}/contract`, { method, title: heading, body: text }),
    onSuccess: (r) => {
      qc.invalidateQueries({ queryKey: ['contracts'] })
      qc.invalidateQueries({ queryKey: ['customer'] })
      qc.invalidateQueries({ queryKey: ['users'] })
      setResult(r)
    },
    onError: (e) => {
      if (e instanceof ApiError) setErrors(e.fields)
      toast.error(errorMessage(e))
    },
  })
  return (
    <Modal
      open={!!target}
      onClose={close}
      size="lg"
      title={`חוזה ל${name}`}
      footer={
        result ? (
          <Button onClick={close}>סיום</Button>
        ) : (
          <>
            <Button variant="secondary" onClick={close}>
              ביטול
            </Button>
            <Button icon={method === 'digital' ? <Send className="h-4 w-4" /> : undefined} loading={send.isPending} onClick={() => send.mutate()}>
              {method === 'digital' ? 'שליחה לחתימה' : 'יצירה'}
            </Button>
          </>
        )
      }
    >
      {result ? (
        <div className="space-y-4">
          {result.signUrl && <SignLink url={result.signUrl} sentVia={result.sentVia} phone={result.user.phone ?? phone} name={name} />}
          {result.contract?.method === 'scan' && <ScanUpload contract={result.contract} onSigned={close} />}
        </div>
      ) : (
        <div className="grid gap-4">
          <div className="grid grid-cols-2 gap-2">
            {(['digital', 'scan'] as const).map((m) => (
              <button
                key={m}
                type="button"
                onClick={() => setMethod(m)}
                className={clsx('min-h-12 rounded-xl border p-3 text-sm font-semibold', method === m ? 'border-brand bg-brand-soft text-brand-strong' : 'border-line')}
              >
                {m === 'digital' ? 'קישור לחתימה דיגיטלית' : 'סריקת חוזה חתום'}
                <span className="mt-0.5 block text-[11px] font-normal text-muted">{m === 'digital' ? 'חותמים באצבע מהטלפון' : 'צילום / PDF של החוזה החתום'}</span>
              </button>
            ))}
          </div>
          {isCustomer && (
            <p className={clsx('rounded-xl p-3 text-xs', email ? 'bg-subtle text-muted' : 'bg-amber-50 text-amber-800 dark:bg-amber-950/20')}>
              {email ? (
                <>
                  לאחר החתימה ייפתח ללקוח חשבון במערכת – כניסה עם חשבון Google של <b dir="ltr">{email}</b>.
                </>
              ) : (
                'ללקוח אין אימייל: הוסיפו אימייל (חשבון Google) בפרטי הלקוח כדי לפתוח לו חשבון.'
              )}
            </p>
          )}
          {errors.email && <p className="text-xs text-danger">{errors.email}</p>}
          {method === 'digital' && !phone && <p className="text-xs text-amber-700">אין מספר טלפון – שלחו את הקישור באימייל או העתיקו אותו.</p>}
          <ContractFields title={heading} body={text} onTitle={setTitle} onBody={setBody} errors={errors} />
        </div>
      )}
    </Modal>
  )
}

/** Upload a scanned / photographed signed contract, then confirm (opens the account). */
export function ScanUpload({ contract, onSigned }: { contract: Contract; onSigned: () => void }) {
  const qc = useQueryClient()
  const mark = useMutation({
    mutationFn: () => api.post<Contract>(`/contracts/${contract.id}/mark-signed`),
    onSuccess: () => {
      toast.success('החוזה סומן כחתום והחשבון הופעל')
      qc.invalidateQueries({ queryKey: ['contracts'] })
      qc.invalidateQueries({ queryKey: ['users'] })
      qc.invalidateQueries({ queryKey: ['customer'] })
      onSigned()
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  return (
    <div className="space-y-3 rounded-xl border border-line p-3">
      <Attachments entity="contract" entityId={contract.id} kind="doc" title="החוזה החתום" empty="צלמו את החוזה החתום או צרפו קובץ PDF / תמונה" />
      <Button className="w-full" icon={<FileSignature className="h-4 w-4" />} loading={mark.isPending} onClick={() => mark.mutate()}>
        אישור – החוזה נחתם, הפעלת החשבון
      </Button>
    </div>
  )
}

export function ContractDialog({ contract, onClose }: { contract: Contract | null; onClose: () => void }) {
  const qc = useQueryClient()
  const [link, setLink] = useState<{ url: string; sentVia?: string[] } | null>(null)
  const resend = useMutation({
    mutationFn: () => api.post<ContractResult>(`/contracts/${contract!.id}/resend`),
    onSuccess: (r) => {
      setLink(r.signUrl ? { url: r.signUrl, sentVia: r.sentVia } : null)
      qc.invalidateQueries({ queryKey: ['contracts'] })
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  const cancel = useMutation({
    mutationFn: () => api.post(`/contracts/${contract!.id}/cancel`),
    onSuccess: () => {
      toast.success('החוזה בוטל')
      qc.invalidateQueries({ queryKey: ['contracts'] })
      close()
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  const close = () => {
    setLink(null)
    onClose()
  }
  if (!contract) return null
  const pending = contract.status === 'pending'
  return (
    <Modal
      open
      onClose={close}
      size="lg"
      title={
        <span className="flex flex-wrap items-center gap-2">
          {contract.title}
          <Badge tone={contract.status === 'signed' ? 'green' : pending ? 'amber' : 'gray'}>{contractStatusLabel[contract.status]}</Badge>
        </span>
      }
      footer={
        <>
          {pending && (
            <Button variant="ghost" className="me-auto text-danger" loading={cancel.isPending} onClick={() => cancel.mutate()}>
              ביטול החוזה
            </Button>
          )}
          {pending && contract.method === 'digital' && (
            <Button variant="secondary" icon={<Send className="h-4 w-4" />} loading={resend.isPending} onClick={() => resend.mutate()}>
              שליחה חוזרת
            </Button>
          )}
          <Button onClick={close}>סגירה</Button>
        </>
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-muted">
          {contract.userName} · {contract.method === 'digital' ? 'חתימה דיגיטלית' : 'סריקה'}
          {contract.sentAt && ` · נשלח ${dateTime(contract.sentAt)}`}
        </p>
        {link && <SignLink url={link.url} sentVia={link.sentVia} phone={contract.userPhone} name={contract.userName} />}
        {contract.status === 'signed' && (
          <div className="rounded-xl bg-emerald-50 p-3 text-sm dark:bg-emerald-950/20">
            נחתם ע״י <b>{contract.signedName}</b> {contract.signedAt && `ב־${dateTime(contract.signedAt)}`}
          </div>
        )}
        <PhotoGallery entity="contract" entityId={contract.id} title={contract.method === 'digital' ? 'חתימה' : 'החוזה החתום'} />
        {pending && contract.method === 'scan' ? (
          <ScanUpload contract={contract} onSigned={close} />
        ) : (
          <Attachments entity="contract" entityId={contract.id} kind="doc" canDelete={pending} canAdd={contract.status !== 'cancelled'} />
        )}
        <details className="rounded-xl border border-line p-3">
          <summary className="flex min-h-8 cursor-pointer items-center gap-2 text-sm font-medium">
            <FileText className="h-4 w-4 text-muted" /> נוסח החוזה
          </summary>
          <p className="mt-3 whitespace-pre-line text-sm leading-relaxed">{contract.body}</p>
        </details>
      </div>
    </Modal>
  )
}
