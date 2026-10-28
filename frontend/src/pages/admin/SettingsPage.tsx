import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Building2, Plus, Save, Wrench } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from '@/components/toast'
import { Badge, Button, Card, Field, Input, PageHeader } from '@/components/ui'
import { api, errorMessage } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { money } from '@/lib/format'
import { useBuildings, useEquipment } from '@/lib/queries'
import type { Building, EquipmentItem } from '@/lib/types'

export function SettingsPage() {
  const config = useAuth((s) => s.config)
  const health = useQuery({ queryKey: ['health'], queryFn: () => fetch('/api/health').then((r) => r.json() as Promise<Record<string, unknown>>) })
  return (
    <div className="space-y-6">
      <PageHeader title="הגדרות מערכת" subtitle="מבנים ושעות פעילות, קטלוג ציוד ותמחור, מצב שירותים" />
      <Buildings />
      <Equipment />
      <Card title="מצב מערכת">
        <dl className="grid gap-3 text-sm sm:grid-cols-2 lg:grid-cols-4">
          <Item label="גרסה" value={String(health.data?.version ?? config?.version ?? '—')} />
          <Item label="אימות" value={config?.authMode === 'firebase' ? 'Google / Firebase' : 'מצב הדגמה (dev)'} tone={config?.authMode === 'firebase' ? 'green' : 'amber'} />
          <Item label="מסד נתונים" value={health.data?.status === 'ok' ? 'תקין' : 'בעיה'} tone={health.data?.status === 'ok' ? 'green' : 'red'} />
          <Item label="אחסון קבצים (Garage S3)" value={health.data?.storage === 'ok' ? 'תקין' : String(health.data?.storage ?? '—')} tone={health.data?.storage === 'ok' ? 'green' : 'amber'} />
          <Item label="חיבורי זמן אמת" value={String(health.data?.wsClients ?? 0)} />
          <Item label="אזור זמן" value={config?.timezone ?? '—'} />
        </dl>
      </Card>
    </div>
  )
}

function Item({ label, value, tone }: { label: string; value: string; tone?: 'green' | 'amber' | 'red' }) {
  return (
    <div className="rounded-xl bg-subtle p-3">
      <dt className="text-xs text-muted">{label}</dt>
      <dd className="mt-1 font-medium">{tone ? <Badge tone={tone}>{value}</Badge> : value}</dd>
    </div>
  )
}

function Buildings() {
  const qc = useQueryClient()
  const { data = [] } = useBuildings()
  const [rows, setRows] = useState<Partial<Building>[]>([])
  useEffect(() => setRows(data), [data])
  const save = useMutation({
    mutationFn: (b: Partial<Building>) => (b.id ? api.put(`/buildings/${b.id}`, b) : api.post('/buildings', b)),
    onSuccess: () => {
      toast.success('המבנה נשמר')
      qc.invalidateQueries({ queryKey: ['buildings'] })
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  const upd = (i: number, patch: Partial<Building>) => setRows((r) => r.map((x, j) => (j === i ? { ...x, ...patch } : x)))
  return (
    <Card
      title={
        <span className="flex items-center gap-2">
          <Building2 className="h-4 w-4" /> מבנים ושעות פעילות
        </span>
      }
      actions={
        <Button size="sm" variant="secondary" icon={<Plus className="h-3.5 w-3.5" />} onClick={() => setRows((r) => [...r, { name: '', address: '', opensAt: '07:00', closesAt: '23:00' }])}>
          מבנה
        </Button>
      }
    >
      <div className="space-y-3">
        {rows.map((b, i) => (
          <div key={b.id ?? `new-${i}`} className="grid items-end gap-3 rounded-xl border border-line p-3 sm:grid-cols-[1fr_1fr_110px_110px_auto]">
            <Field label="שם">
              <Input value={b.name ?? ''} onChange={(e) => upd(i, { name: e.target.value })} />
            </Field>
            <Field label="כתובת">
              <Input value={b.address ?? ''} onChange={(e) => upd(i, { address: e.target.value })} />
            </Field>
            <Field label="פתיחה">
              <Input type="time" value={b.opensAt ?? ''} onChange={(e) => upd(i, { opensAt: e.target.value })} />
            </Field>
            <Field label="סגירה">
              <Input type="time" value={b.closesAt ?? ''} onChange={(e) => upd(i, { closesAt: e.target.value })} />
            </Field>
            <Button size="md" icon={<Save className="h-4 w-4" />} onClick={() => save.mutate(b)} loading={save.isPending && save.variables === b}>
              שמירה
            </Button>
          </div>
        ))}
      </div>
    </Card>
  )
}

function Equipment() {
  const qc = useQueryClient()
  const { data = [] } = useEquipment()
  const [draft, setDraft] = useState<Partial<EquipmentItem>>({ code: '', label: '', flatPrice: 0 })
  const save = useMutation({
    mutationFn: (e: Partial<EquipmentItem>) => api.post('/equipment', e),
    onSuccess: () => {
      toast.success('הפריט נשמר')
      qc.invalidateQueries({ queryKey: ['equipment'] })
      setDraft({ code: '', label: '', flatPrice: 0 })
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  return (
    <Card
      title={
        <span className="flex items-center gap-2">
          <Wrench className="h-4 w-4" /> קטלוג ציוד ושירותים
        </span>
      }
    >
      <p className="mb-3 text-xs text-muted">תוספת מחיר קבועה לכל הזמנה בשכירות. החדרים מגדירים איזה ציוד זמין אצלם.</p>
      <ul className="mb-4 divide-y divide-line rounded-xl border border-line">
        {data.map((e) => (
          <EquipmentRow key={e.id} item={e} onSave={(x) => save.mutate(x)} />
        ))}
      </ul>
      <div className="grid items-end gap-3 sm:grid-cols-[140px_1fr_140px_auto]">
        <Field label="קוד (באנגלית)">
          <Input dir="ltr" value={draft.code} onChange={(e) => setDraft({ ...draft, code: e.target.value })} placeholder="sound" />
        </Field>
        <Field label="שם">
          <Input value={draft.label} onChange={(e) => setDraft({ ...draft, label: e.target.value })} placeholder="מערכת הגברה" />
        </Field>
        <Field label="מחיר (₪)">
          <Input type="number" min={0} value={draft.flatPrice} onChange={(e) => setDraft({ ...draft, flatPrice: Number(e.target.value) })} />
        </Field>
        <Button icon={<Plus className="h-4 w-4" />} onClick={() => save.mutate(draft)} disabled={!draft.code || !draft.label}>
          הוספה
        </Button>
      </div>
    </Card>
  )
}

function EquipmentRow({ item, onSave }: { item: EquipmentItem; onSave: (e: EquipmentItem) => void }) {
  const [price, setPrice] = useState(item.flatPrice)
  const [label, setLabel] = useState(item.label)
  const dirty = price !== item.flatPrice || label !== item.label
  return (
    <li className="flex flex-wrap items-center gap-3 px-3 py-2">
      <code className="w-24 text-xs text-muted" dir="ltr">
        {item.code}
      </code>
      <Input className="h-8 max-w-xs flex-1" value={label} onChange={(e) => setLabel(e.target.value)} />
      <Input className="h-8 w-28" type="number" min={0} value={price} onChange={(e) => setPrice(Number(e.target.value))} />
      <span className="w-20 text-xs text-muted">{money(price)}</span>
      {dirty && (
        <Button size="sm" onClick={() => onSave({ ...item, label, flatPrice: price })}>
          שמירה
        </Button>
      )}
    </li>
  )
}
