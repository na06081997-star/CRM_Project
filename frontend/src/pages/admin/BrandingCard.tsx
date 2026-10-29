import { useMutation } from '@tanstack/react-query'
import { ImagePlus, Palette, RotateCcw, Save, Trash2 } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { OrgMark, ProductMark } from '@/components/layout/Logo'
import { toast } from '@/components/toast'
import { Button, Card, Field, Input } from '@/components/ui'
import { ApiError, api, errorMessage, publicFileUrl } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { PRODUCT } from '@/lib/brand'
import type { Attachment, Organization } from '@/lib/types'

// Matches service.OrganizationEntityID on the backend.
const ORG_ENTITY_ID = '00000000-0000-0000-0000-000000000001'
const PRESETS = ['#0f766e', '#2f8f46', '#1d4ed8', '#7c3aed', '#be123c', '#c2410c', '#0e7490', '#334155']

/** Client (matnas) identity: name, logos and colour. Our product brand stays fixed. */
export function BrandingCard() {
  const config = useAuth((s) => s.config)
  const reload = useAuth((s) => s.reloadConfig)
  const [o, setO] = useState<Organization | null>(config?.organization ?? null)
  const [errors, setErrors] = useState<Record<string, string>>({})
  useEffect(() => setO(config?.organization ?? null), [config?.organization])

  const save = useMutation({
    mutationFn: (v: Organization) => api.put<Organization>('/organization', v),
    onSuccess: async () => {
      setErrors({})
      await reload()
      toast.success('המיתוג נשמר ומוצג לכל המשתמשים')
    },
    onError: (e) => {
      if (e instanceof ApiError) setErrors(e.fields ?? {})
      toast.error(errorMessage(e))
    },
  })
  if (!o) return null
  const set = (patch: Partial<Organization>) => setO({ ...o, ...patch })

  return (
    <Card
      title={
        <span className="flex items-center gap-2">
          <Palette className="h-4 w-4" /> מיתוג הארגון (הלקוח)
        </span>
      }
      actions={
        <Button size="sm" icon={<Save className="h-3.5 w-3.5" />} loading={save.isPending} onClick={() => save.mutate(o)}>
          שמירה
        </Button>
      }
    >
      <div className="grid gap-6 lg:grid-cols-[1fr_320px]">
        <div className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="שם מלא" error={errors.name} hint="מופיע במסך הכניסה ובשילוט">
              <Input value={o.name} onChange={(e) => set({ name: e.target.value })} placeholder="מינהל קהילתי מעלה רמות" />
            </Field>
            <Field label="שם קצר" hint="מופיע בתפריט ובכותרת הלשונית">
              <Input value={o.shortName} onChange={(e) => set({ shortName: e.target.value })} placeholder="מעלה רמות" />
            </Field>
            <Field label="תיאור" className="sm:col-span-2">
              <Input value={o.subtitle} onChange={(e) => set({ subtitle: e.target.value })} placeholder="מינהל קהילתי / מתנ״ס" />
            </Field>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <LogoField label="לוגו מלא" hint="כולל טקסט · למסך הכניסה ולשילוט" value={o.logoUrl} error={errors.logoUrl} onChange={(logoUrl) => set({ logoUrl })} />
            <LogoField label="סמל (ריבועי)" hint="בלי טקסט · לתפריט ולאייקון הלשונית" value={o.markUrl} error={errors.markUrl} onChange={(markUrl) => set({ markUrl })} />
          </div>

          <Field label="צבע מותג" error={errors.brandColor} hint="צובע כפתורים, תפריט והדגשות. ברירת מחדל: הצבע של המערכת">
            <div className="flex flex-wrap items-center gap-2">
              {PRESETS.map((c) => (
                <button
                  key={c}
                  type="button"
                  onClick={() => set({ brandColor: c })}
                  className="h-8 w-8 rounded-full transition hover:scale-110"
                  style={{ background: c, boxShadow: o.brandColor === c ? `0 0 0 2px rgb(var(--surface)), 0 0 0 4px ${c}` : undefined }}
                  aria-label={c}
                />
              ))}
              <input
                type="color"
                value={o.brandColor || PRODUCT.color}
                onChange={(e) => set({ brandColor: e.target.value })}
                className="h-8 w-10 cursor-pointer rounded-lg border border-line bg-surface"
                aria-label="צבע מותאם"
              />
              <Button size="sm" variant="ghost" icon={<RotateCcw className="h-3.5 w-3.5" />} onClick={() => set({ brandColor: '' })}>
                ברירת מחדל
              </Button>
            </div>
          </Field>

          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="טלפון">
              <Input dir="ltr" value={o.phone} onChange={(e) => set({ phone: e.target.value })} />
            </Field>
            <Field label="דוא״ל">
              <Input dir="ltr" type="email" value={o.email} onChange={(e) => set({ email: e.target.value })} />
            </Field>
            <Field label="אתר" error={errors.website}>
              <Input dir="ltr" value={o.website} onChange={(e) => set({ website: e.target.value })} placeholder="https://" />
            </Field>
            <Field label="כתובת">
              <Input value={o.address} onChange={(e) => set({ address: e.target.value })} />
            </Field>
          </div>
        </div>

        <Preview o={o} />
      </div>
    </Card>
  )
}

function LogoField({ label, hint, value, error, onChange }: { label: string; hint: string; value: string; error?: string; onChange: (url: string) => void }) {
  const input = useRef<HTMLInputElement>(null)
  const upload = useMutation({
    mutationFn: (file: File) => {
      const fd = new FormData()
      fd.set('entity', 'organization')
      fd.set('entityId', ORG_ENTITY_ID)
      fd.set('file', file)
      return api.post<Attachment>('/files', fd)
    },
    onSuccess: (a) => onChange(publicFileUrl(a.id)),
    onError: (e) => toast.error(errorMessage(e)),
  })
  return (
    <Field label={label} hint={hint} error={error}>
      <div className="flex items-center gap-3 rounded-xl border border-dashed border-line p-3">
        <span className="flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-white ring-1 ring-line">
          {value ? <img src={value} alt="" className="h-full w-full object-contain" /> : <ImagePlus className="h-5 w-5 text-slate-400" />}
        </span>
        <div className="flex flex-wrap gap-1.5">
          <Button size="sm" variant="secondary" loading={upload.isPending} onClick={() => input.current?.click()}>
            {value ? 'החלפה' : 'העלאה'}
          </Button>
          {value && (
            <Button size="sm" variant="ghost" icon={<Trash2 className="h-3.5 w-3.5" />} onClick={() => onChange('')}>
              הסרה
            </Button>
          )}
        </div>
        <input
          ref={input}
          type="file"
          accept="image/png,image/jpeg,image/webp"
          hidden
          onChange={(e) => {
            const f = e.target.files?.[0]
            if (f) upload.mutate(f)
            e.target.value = ''
          }}
        />
      </div>
    </Field>
  )
}

/** Shows how the client and product brands sit together. */
function Preview({ o }: { o: Organization }) {
  const mark = o.markUrl || o.logoUrl
  const color = o.brandColor || PRODUCT.color
  return (
    <div className="space-y-3">
      <p className="label">תצוגה מקדימה</p>
      <div className="space-y-3 rounded-2xl border border-line bg-surface p-4">
        <div className="flex items-center gap-2.5">
          {mark ? <OrgMark src={mark} name={o.name} /> : <span className="h-10 w-10 rounded-xl" style={{ background: color }} />}
          <div className="leading-tight">
            <p className="font-extrabold">{o.shortName || o.name || 'שם הארגון'}</p>
            <p className="text-xs text-muted">ממשק ניהול</p>
          </div>
        </div>
        <div className="rounded-xl px-3 py-2 text-sm font-medium" style={{ background: `${color}22`, color }}>
          יומן הזמנות
        </div>
        <div className="px-3 py-1 text-sm text-muted">חדרים ואולמות</div>
        <div className="rounded-xl px-3 py-2 text-center text-sm font-semibold text-white" style={{ background: color }}>
          בקשה חדשה
        </div>
        <div className="flex items-center gap-2 border-t border-line pt-3 text-[11px] text-muted">
          <ProductMark className="h-5 w-5" /> מופעל על ידי <b className="text-ink/80">{PRODUCT.name}</b>
        </div>
      </div>
      <p className="text-xs leading-relaxed text-muted">
        המותג של הלקוח מוביל בכל המסכים. {PRODUCT.name} מופיע בקטן בתחתית התפריט, במסך הכניסה ובשילוט.
      </p>
    </div>
  )
}
