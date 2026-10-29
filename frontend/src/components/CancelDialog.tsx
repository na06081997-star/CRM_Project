import { useMutation, useQueryClient } from '@tanstack/react-query'
import { CalendarDays } from 'lucide-react'
import { useState } from 'react'
import { api, errorMessage } from '@/lib/api'
import { dateShort, timeRange } from '@/lib/format'
import type { Block } from '@/lib/types'
import { toast } from './toast'
import { Button, Field, Modal, Textarea } from './ui'

/** Owner asks the manager to cancel a confirmed booking (room stays blocked until approved). */
export function CancelDialog({ block, onClose }: { block: Pick<Block, 'id' | 'title' | 'startsAt' | 'endsAt' | 'roomName' | 'cancelRequested'> | null; onClose: () => void }) {
  const qc = useQueryClient()
  const [reason, setReason] = useState('')
  const send = useMutation({
    mutationFn: () => api.post(`/bookings/${block!.id}/cancel-request`, { reason }),
    onSuccess: () => {
      toast.success('בקשת הביטול נשלחה למנהל. עד לאישור – המועד נשאר שמור.')
      for (const k of ['blocks', 'bookings', 'requests']) qc.invalidateQueries({ queryKey: [k] })
      setReason('')
      onClose()
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  if (!block) return null
  return (
    <Modal
      open
      onClose={onClose}
      size="sm"
      title={block.title}
      footer={
        block.cancelRequested ? (
          <Button variant="secondary" onClick={onClose}>
            סגירה
          </Button>
        ) : (
          <>
            <Button variant="secondary" onClick={onClose}>
              חזרה
            </Button>
            <Button variant="danger" loading={send.isPending} disabled={!reason.trim()} onClick={() => send.mutate()}>
              שליחת בקשת ביטול
            </Button>
          </>
        )
      }
    >
      <p className="flex items-center gap-2 text-sm">
        <CalendarDays className="h-4 w-4 text-muted" /> {dateShort(block.startsAt)} · {timeRange(block.startsAt, block.endsAt)} · {block.roomName}
      </p>
      {block.cancelRequested ? (
        <p className="mt-4 rounded-xl bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-950/40 dark:text-amber-200">בקשת ביטול כבר נשלחה וממתינה לאישור המנהל.</p>
      ) : (
        <Field label="סיבת הביטול" className="mt-4">
          <Textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={3} placeholder="לדוגמה: הפעילות נדחתה לשבוע הבא" autoFocus />
        </Field>
      )}
      <p className="mt-3 text-xs text-muted">הביטול נכנס לתוקף רק לאחר אישור מנהל המתנ״ס.</p>
    </Modal>
  )
}
