import clsx from 'clsx'
import { Check, ChevronDown } from 'lucide-react'
import { useState, type ReactNode } from 'react'

export interface Option {
  value: string
  label: string
  color?: string
  group?: string
}

/** Compact dropdown with checkboxes. Empty selection means "all". */
export function MultiSelect({ label, allLabel, options, value, onChange, className }: { label: string; allLabel: string; options: Option[]; value: string[]; onChange: (v: string[]) => void; className?: string }) {
  const [open, setOpen] = useState(false)
  const selected = new Set(value)
  const summary: ReactNode =
    value.length === 0 ? allLabel : value.length === 1 ? options.find((o) => o.value === value[0])?.label ?? allLabel : `${value.length} נבחרו`
  const toggle = (v: string) => onChange(selected.has(v) ? value.filter((x) => x !== v) : [...value, v])
  let lastGroup: string | undefined

  return (
    <div className={clsx('relative min-w-0', className)}>
      <span className="label">{label}</span>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className={clsx('input flex items-center justify-between gap-2 text-start', value.length > 0 && 'border-brand/50 bg-brand-soft/40')}
      >
        <span className="truncate">{summary}</span>
        <ChevronDown className="h-4 w-4 shrink-0 text-muted" />
      </button>
      {open && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
          <div className="absolute start-0 top-full z-50 mt-1 w-64 animate-slide-up overflow-hidden rounded-xl border border-line bg-surface shadow-pop">
            <div className="scroll-thin max-h-72 overflow-y-auto p-1">
              <button
                type="button"
                onClick={() => onChange([])}
                className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-sm hover:bg-subtle"
              >
                <Box on={value.length === 0} />
                {allLabel}
              </button>
              {options.map((o) => {
                const header = o.group && o.group !== lastGroup ? o.group : null
                lastGroup = o.group
                return (
                  <div key={o.value}>
                    {header && <p className="px-2.5 pb-1 pt-2 text-[11px] font-semibold text-muted">{header}</p>}
                    <button
                      type="button"
                      onClick={() => toggle(o.value)}
                      className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-sm hover:bg-subtle"
                    >
                      <Box on={selected.has(o.value)} color={o.color} />
                      <span className="truncate">{o.label}</span>
                    </button>
                  </div>
                )
              })}
            </div>
          </div>
        </>
      )}
    </div>
  )
}

function Box({ on, color }: { on: boolean; color?: string }) {
  return (
    <span
      className={clsx('flex h-4 w-4 shrink-0 items-center justify-center rounded border', on ? 'border-transparent text-white' : 'border-line')}
      style={{ background: on ? color || 'rgb(var(--brand))' : undefined }}
    >
      {on && <Check className="h-3 w-3" strokeWidth={3} />}
    </span>
  )
}
