import clsx from 'clsx'
import { Minus, Plus } from 'lucide-react'
import type { EquipmentItem } from '@/lib/types'

/** Yes/no questions of the booking form; each maps to equipment codes. */
export const NEEDS: { key: string; label: string; codes: string[] }[] = [
  { key: 'catering', label: 'אירוע עם כיבוד?', codes: ['catering'] },
  { key: 'sound', label: 'הגברה ותאורה?', codes: ['sound', 'lighting'] },
  { key: 'partitions', label: 'מחיצות?', codes: ['partitions'] },
]
const NEED_CODES = new Set(NEEDS.flatMap((n) => n.codes))

export interface Needs {
  chairs: number
  tables: number
  equipment: string[]
}

/**
 * The requirements questionnaire shared by the office booking form and the
 * coordinator / portal request: chairs, tables, catering, sound & lighting,
 * partitions, plus the room's other equipment. Prices are never shown here.
 */
export function NeedsChecklist({
  value,
  onChange,
  roomEquipment = [],
  catalog = [],
  disabled,
}: {
  value: Needs
  onChange: (v: Needs) => void
  roomEquipment?: string[]
  catalog?: EquipmentItem[]
  disabled?: boolean
}) {
  const has = (codes: string[]) => codes.every((c) => value.equipment.includes(c))
  const setNeed = (codes: string[], on: boolean) =>
    onChange({ ...value, equipment: on ? [...new Set([...value.equipment, ...codes])] : value.equipment.filter((c) => !codes.includes(c)) })
  const extras = roomEquipment.filter((c) => !NEED_CODES.has(c))
  const label = (code: string) => catalog.find((e) => e.code === code)?.label ?? code

  return (
    <div className="space-y-3">
      <div className="grid grid-cols-2 gap-3">
        <Counter label="כיסאות?" value={value.chairs} disabled={disabled} onChange={(chairs) => onChange({ ...value, chairs })} step={10} />
        <Counter label="שולחנות?" value={value.tables} disabled={disabled} onChange={(tables) => onChange({ ...value, tables })} step={1} />
      </div>
      <div className="divide-y divide-line rounded-xl border border-line">
        {NEEDS.map((n) => {
          const on = has(n.codes)
          return (
            <div key={n.key} className="flex items-center justify-between gap-3 px-3 py-2">
              <span className="text-sm font-medium">{n.label}</span>
              <YesNo value={on} disabled={disabled} onChange={(v) => setNeed(n.codes, v)} />
            </div>
          )
        })}
      </div>
      {extras.length > 0 && (
        <div>
          <span className="label">ציוד נוסף בחלל</span>
          <div className="flex flex-wrap gap-1.5">
            {extras.map((code) => {
              const on = value.equipment.includes(code)
              return (
                <button
                  type="button"
                  key={code}
                  disabled={disabled}
                  onClick={() => setNeed([code], !on)}
                  className={clsx('min-h-9 rounded-xl border px-3 py-1.5 text-xs transition', on ? 'border-brand bg-brand-soft font-semibold text-brand-strong' : 'border-line hover:bg-subtle')}
                >
                  {label(code)}
                </button>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}

function YesNo({ value, onChange, disabled }: { value: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return (
    <div className="inline-flex shrink-0 rounded-xl border border-line p-0.5">
      {[true, false].map((v) => (
        <button
          key={String(v)}
          type="button"
          disabled={disabled}
          onClick={() => onChange(v)}
          className={clsx('min-h-9 min-w-12 rounded-lg px-3 text-sm', value === v ? 'bg-brand-soft font-semibold text-brand-strong ring-1 ring-brand/40' : 'text-muted')}
        >
          {v ? 'כן' : 'לא'}
        </button>
      ))}
    </div>
  )
}

function Counter({ label, value, onChange, step, disabled }: { label: string; value: number; onChange: (v: number) => void; step: number; disabled?: boolean }) {
  return (
    <div>
      <span className="label">{label}</span>
      <div className="flex h-11 items-center rounded-xl border border-line">
        <button type="button" disabled={disabled || value <= 0} onClick={() => onChange(Math.max(0, value - step))} className="flex h-full w-11 items-center justify-center text-muted disabled:opacity-40" aria-label={`פחות ${label}`}>
          <Minus className="h-4 w-4" />
        </button>
        <input
          type="number"
          inputMode="numeric"
          min={0}
          dir="ltr"
          disabled={disabled}
          value={value}
          onChange={(e) => onChange(Math.max(0, Number(e.target.value) || 0))}
          className="h-full w-full min-w-0 bg-transparent text-center text-base font-semibold tabular-nums outline-none"
          aria-label={label}
        />
        <button type="button" disabled={disabled} onClick={() => onChange(value + step)} className="flex h-full w-11 items-center justify-center text-muted" aria-label={`עוד ${label}`}>
          <Plus className="h-4 w-4" />
        </button>
      </div>
    </div>
  )
}

/** One-line summary for review screens ("כיסאות: 40 · כיבוד: כן …"). */
export function needsSummary(v: Needs, catalog: EquipmentItem[] = []): string {
  const parts = [`כיסאות: ${v.chairs}`, `שולחנות: ${v.tables}`]
  for (const n of NEEDS) parts.push(`${n.label.replace('?', '')}: ${n.codes.every((c) => v.equipment.includes(c)) ? 'כן' : 'לא'}`)
  const extras = v.equipment.filter((c) => !NEED_CODES.has(c)).map((c) => catalog.find((e) => e.code === c)?.label ?? c)
  if (extras.length) parts.push(`ציוד נוסף: ${extras.join(', ')}`)
  return parts.join(' · ')
}
