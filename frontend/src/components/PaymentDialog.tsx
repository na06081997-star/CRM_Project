import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { api, ApiError, errorMessage } from '@/lib/api'
import { money, toLocalInput } from '@/lib/format'
import { paymentMethodLabel } from '@/lib/labels'
import type { Booking, Payment, PaymentMethod } from '@/lib/types'
import { toast } from './toast'
import { Button, Field, Input, Modal, Select, Textarea } from './ui'

export function PaymentDialog({ booking, open, onClose }: { booking: Booking | null; open: boolean; onClose: () => void }) {
  const qc = useQueryClient()
  const balance = booking ? Math.max(0, booking.price - booking.paidAmount) : 0
  const [amount, setAmount] = useState('')
  const [method, setMethod] = useState<PaymentMethod>('bank_transfer')
  const [reference, setReference] = useState('')
  const [paidAt, setPaidAt] = useState(toLocalInput(new Date()))
  const [notes, setNotes] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})

  useEffect(() => {
    if (open) {
      setAmount(balance ? String(balance) : '')
      setReference('')
      setNotes('')
      setErrors({})
      setPaidAt(toLocalInput(new Date()))
    }
  }, [open, balance])

  const save = useMutation({
    mutationFn: () =>
      api.post<Payment>('/payments', {
        bookingId: booking!.id,
        amount: Number(amount),
        method,
        reference,
        notes,
        paidAt: new Date(paidAt).toISOString(),
      }),
    onSuccess: () => {
      toast.success('התשלום נרשם')
      for (const k of ['payments', 'balances', 'bookings', 'dashboard', 'customers']) qc.invalidateQueries({ queryKey: [k] })
      onClose()
    },
    onError: (e) => {
      if (e instanceof ApiError) setErrors(e.fields)
      toast.error(errorMessage(e))
    },
  })

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="רישום תשלום"
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            ביטול
          </Button>
          <Button onClick={() => save.mutate()} loading={save.isPending} disabled={!amount}>
            שמירת תשלום
          </Button>
        </>
      }
    >
      {booking && (
        <div className="mb-4 rounded-xl bg-subtle p-3 text-sm">
          <p className="font-medium">{booking.title}</p>
          <p className="mt-1 text-muted">
            סה״כ {money(booking.price)} · שולם {money(booking.paidAmount)} · <span className="font-semibold text-ink">יתרה {money(balance)}</span>
          </p>
        </div>
      )}
      <div className="grid gap-3 sm:grid-cols-2">
        <Field label="סכום (₪)" error={errors.amount}>
          <Input type="number" min={0} step="0.01" value={amount} onChange={(e) => setAmount(e.target.value)} autoFocus />
        </Field>
        <Field label="אמצעי תשלום" error={errors.method}>
          <Select value={method} onChange={(e) => setMethod(e.target.value as PaymentMethod)}>
            {(Object.keys(paymentMethodLabel) as PaymentMethod[]).map((m) => (
              <option key={m} value={m}>
                {paymentMethodLabel[m]}
              </option>
            ))}
          </Select>
        </Field>
        <Field label={method === 'check' ? 'מספר צ׳ק' : 'אסמכתא'} error={errors.reference}>
          <Input value={reference} onChange={(e) => setReference(e.target.value)} />
        </Field>
        <Field label="תאריך תשלום">
          <Input type="datetime-local" value={paidAt} onChange={(e) => setPaidAt(e.target.value)} />
        </Field>
        <Field label="הערות" className="sm:col-span-2">
          <Textarea value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
        </Field>
      </div>
    </Modal>
  )
}
