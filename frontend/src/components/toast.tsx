import clsx from 'clsx'
import { CheckCircle2, Info, XCircle } from 'lucide-react'
import { create } from 'zustand'

type Kind = 'success' | 'error' | 'info'
interface ToastItem {
  id: number
  kind: Kind
  text: string
}

const useToasts = create<{ items: ToastItem[]; push: (k: Kind, t: string) => void; drop: (id: number) => void }>((set) => ({
  items: [],
  push: (kind, text) => {
    const id = Date.now() + Math.random()
    set((s) => ({ items: [...s.items.slice(-3), { id, kind, text }] }))
    setTimeout(() => set((s) => ({ items: s.items.filter((i) => i.id !== id) })), kind === 'error' ? 6000 : 3500)
  },
  drop: (id) => set((s) => ({ items: s.items.filter((i) => i.id !== id) })),
}))

export const toast = {
  success: (t: string) => useToasts.getState().push('success', t),
  error: (t: string) => useToasts.getState().push('error', t),
  info: (t: string) => useToasts.getState().push('info', t),
}

const icons = { success: CheckCircle2, error: XCircle, info: Info }
const colors = { success: 'text-ok', error: 'text-danger', info: 'text-brand' }

export function Toaster() {
  const { items, drop } = useToasts()
  return (
    <div className="pointer-events-none fixed inset-x-0 bottom-4 z-[60] flex flex-col items-center gap-2 px-4">
      {items.map((t) => {
        const Icon = icons[t.kind]
        return (
          <button
            key={t.id}
            onClick={() => drop(t.id)}
            className="pointer-events-auto flex max-w-md animate-slide-up items-center gap-3 rounded-xl border border-line bg-surface px-4 py-3 text-sm shadow-pop"
          >
            <Icon className={clsx('h-5 w-5 shrink-0', colors[t.kind])} />
            <span className="text-start">{t.text}</span>
          </button>
        )
      })}
    </div>
  )
}
