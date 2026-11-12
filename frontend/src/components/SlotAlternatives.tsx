import { AlertTriangle, ArrowLeftRight, Clock } from 'lucide-react'
import { ltr, time, timeRange } from '@/lib/format'
import type { Conflict, RoomOption } from '@/lib/types'

/**
 * Explains why a slot is not available and offers free alternatives: other
 * times the same day in this room (clear of cleaning buffers) and other rooms
 * at the same time. Tapping one applies it to the form.
 */
export function SlotAlternatives({
  conflicts = [],
  outsideHours,
  hours,
  inPast,
  bufferMinutes,
  sameRoom,
  otherRooms,
  onPickTime,
  onPickRoom,
}: {
  conflicts?: Conflict[]
  outsideHours?: boolean
  hours?: string
  inPast?: boolean
  bufferMinutes?: number
  sameRoom?: { start: string; end: string }[] | null
  otherRooms?: RoomOption[] | null
  onPickTime?: (start: Date, end: Date) => void
  onPickRoom?: (roomId: string) => void
}) {
  const onlyBuffer = conflicts.length > 0 && conflicts.every((c) => c.reason === 'buffer')
  const reason = inPast
    ? 'המועד שנבחר כבר עבר'
    : outsideHours
      ? `מחוץ לשעות הפעילות (${hours ? ltr(hours) : ''})`
      : onlyBuffer
        ? `צמוד מדי להזמנה אחרת – נדרשות ${bufferMinutes ?? 15} דק׳ לניקיון והתארגנות בין פעילויות`
        : 'החלל תפוס בשעות שבחרת'
  const times = sameRoom ?? []
  const rooms = otherRooms ?? []

  return (
    <div className="rounded-2xl border border-danger/30 bg-red-50 p-4 dark:bg-red-950/25">
      <p className="flex items-start gap-2 text-sm font-semibold text-red-800 dark:text-red-200">
        <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" /> {reason}
      </p>
      {conflicts.length > 0 && (
        <ul className="mt-2 space-y-1 text-xs text-red-900/80 dark:text-red-100/80">
          {conflicts.map((c, i) => (
            <li key={c.bookingId + i}>
              {c.title ? `${c.title} · ` : 'תפוס · '}
              {timeRange(c.startsAt, c.endsAt)}
              {c.reason === 'buffer' && ` (+${bufferMinutes ?? 15} דק׳ ניקיון)`}
              {c.reason === 'maintenance' && ' · חסימת תחזוקה'}
            </li>
          ))}
        </ul>
      )}
      {(times.length > 0 || rooms.length > 0) && <p className="mt-3 text-xs font-semibold text-ink">הצעות פנויות:</p>}
      {times.length > 0 && (
        <div className="mt-2">
          <p className="mb-1.5 flex items-center gap-1 text-xs text-muted">
            <Clock className="h-3.5 w-3.5" /> באותו חלל, באותו יום
          </p>
          <div className="flex flex-wrap gap-1.5">
            {times.map((t) => (
              <button
                key={t.start}
                type="button"
                onClick={() => onPickTime?.(new Date(t.start), new Date(t.end))}
                className="min-h-10 rounded-xl border border-ok/40 bg-surface px-3 text-sm font-semibold tabular-nums text-ok transition hover:bg-emerald-50 dark:hover:bg-emerald-950/30"
              >
                {ltr(`${time(t.start)}–${time(t.end)}`)}
              </button>
            ))}
          </div>
        </div>
      )}
      {rooms.length > 0 && (
        <div className="mt-3">
          <p className="mb-1.5 flex items-center gap-1 text-xs text-muted">
            <ArrowLeftRight className="h-3.5 w-3.5" /> חלל אחר, באותה שעה
          </p>
          <div className="flex flex-wrap gap-1.5">
            {rooms.map((r) => (
              <button
                key={r.roomId}
                type="button"
                onClick={() => onPickRoom?.(r.roomId)}
                className="min-h-10 rounded-xl border border-ok/40 bg-surface px-3 text-start text-sm transition hover:bg-emerald-50 dark:hover:bg-emerald-950/30"
              >
                <span className="font-semibold text-ok">{r.roomName}</span>
                <span className="ms-1 text-xs text-muted">
                  · {r.buildingName}
                  {r.capacity > 0 && ` · עד ${r.capacity}`}
                </span>
              </button>
            ))}
          </div>
        </div>
      )}
      {times.length === 0 && rooms.length === 0 && !inPast && <p className="mt-2 text-xs text-muted">לא נמצאו חלופות פנויות ביום זה – נסו תאריך אחר.</p>}
    </div>
  )
}
