import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, UserRound, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api, errorMessage } from '@/lib/api'
import { useCustomers } from '@/lib/queries'
import type { Customer } from '@/lib/types'
import { toast } from './toast'
import { Button, Input } from './ui'

/** Searchable customer select with inline quick-create. */
export function CustomerPicker({ value, valueName, onChange }: { value: string | null; valueName?: string | null; onChange: (id: string | null, name?: string) => void }) {
  const [q, setQ] = useState('')
  const [debounced, setDebounced] = useState('')
  const [open, setOpen] = useState(false)
  const qc = useQueryClient()
  useEffect(() => {
    const t = setTimeout(() => setDebounced(q), 200)
    return () => clearTimeout(t)
  }, [q])
  const { data = [] } = useCustomers(debounced, open)

  const create = useMutation({
    mutationFn: () => api.post<Customer>('/customers', { name: q.trim(), kind: 'private' }),
    onSuccess: (c) => {
      qc.invalidateQueries({ queryKey: ['customers'] })
      onChange(c.id, c.name)
      setOpen(false)
      setQ('')
    },
    onError: (e) => toast.error(errorMessage(e)),
  })

  if (value && !open)
    return (
      <div className="input flex items-center gap-2">
        <UserRound className="h-4 w-4 text-muted" />
        <span className="flex-1 truncate">{valueName || 'לקוח נבחר'}</span>
        <button type="button" onClick={() => onChange(null)} className="text-muted hover:text-ink" aria-label="ניקוי">
          <X className="h-4 w-4" />
        </button>
      </div>
    )

  return (
    <div className="relative">
      <Input value={q} placeholder="חיפוש לקוח לפי שם או טלפון…" onFocus={() => setOpen(true)} onChange={(e) => setQ(e.target.value)} />
      {open && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
          <div className="absolute inset-x-0 top-full z-50 mt-1 overflow-hidden rounded-xl border border-line bg-surface shadow-pop">
            <div className="scroll-thin max-h-56 overflow-y-auto p-1">
              {data.slice(0, 30).map((c) => (
                <button
                  type="button"
                  key={c.id}
                  onClick={() => {
                    onChange(c.id, c.name)
                    setOpen(false)
                    setQ('')
                  }}
                  className="flex w-full items-center justify-between rounded-lg px-3 py-2 text-start text-sm hover:bg-subtle"
                >
                  <span className="truncate">{c.name}</span>
                  <span className="text-xs text-muted">{c.phone}</span>
                </button>
              ))}
              {data.length === 0 && <p className="px-3 py-2 text-sm text-muted">לא נמצאו לקוחות</p>}
            </div>
            {q.trim().length > 1 && (
              <div className="border-t border-line p-1">
                <Button type="button" variant="ghost" size="sm" className="w-full justify-start" icon={<Plus className="h-4 w-4" />} loading={create.isPending} onClick={() => create.mutate()}>
                  יצירת לקוח חדש: “{q.trim()}”
                </Button>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  )
}
