import { useQuery } from '@tanstack/react-query'
import { Camera, ImageOff, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { api, errorMessage, getToken, openFile } from '@/lib/api'
import type { Attachment } from '@/lib/types'
import { toast } from './toast'
import { Spinner } from './ui'

const MAX_SIDE = 1600

/** Phone photos are large: scale them down to MAX_SIDE px JPEG before upload. */
async function shrink(file: File): Promise<File> {
  if (!file.type.startsWith('image/') || file.size < 600_000) return file
  try {
    const bitmap = await createImageBitmap(file)
    const scale = Math.min(1, MAX_SIDE / Math.max(bitmap.width, bitmap.height))
    const canvas = document.createElement('canvas')
    canvas.width = Math.round(bitmap.width * scale)
    canvas.height = Math.round(bitmap.height * scale)
    canvas.getContext('2d')?.drawImage(bitmap, 0, 0, canvas.width, canvas.height)
    const blob = await new Promise<Blob | null>((r) => canvas.toBlob(r, 'image/jpeg', 0.82))
    if (!blob || blob.size >= file.size) return file
    return new File([blob], file.name.replace(/\.\w+$/, '') + '.jpg', { type: 'image/jpeg' })
  } catch {
    return file
  }
}

/** Uploads photos to an entity (after it was created). */
export async function uploadPhotos(entity: string, entityId: string, files: File[]) {
  for (const f of files) {
    const fd = new FormData()
    fd.set('entity', entity)
    fd.set('entityId', entityId)
    fd.set('file', await shrink(f))
    await api.post<Attachment>('/files', fd)
  }
}

/** Take or choose up to `max` photos; previews them before they are sent. */
export function PhotoPicker({ files, onChange, max = 3, label = 'צילום / הוספת תמונה' }: { files: File[]; onChange: (f: File[]) => void; max?: number; label?: string }) {
  const input = useRef<HTMLInputElement>(null)
  const [urls, setUrls] = useState<string[]>([])
  useEffect(() => {
    const u = files.map((f) => URL.createObjectURL(f))
    setUrls(u)
    return () => u.forEach((x) => URL.revokeObjectURL(x))
  }, [files])

  return (
    <div>
      <div className="flex flex-wrap gap-2">
        {urls.map((u, i) => (
          <div key={u} className="relative h-20 w-20 overflow-hidden rounded-xl border border-line">
            <img src={u} alt="" className="h-full w-full object-cover" />
            <button
              type="button"
              onClick={() => onChange(files.filter((_, j) => j !== i))}
              className="absolute end-1 top-1 flex h-6 w-6 items-center justify-center rounded-full bg-slate-900/70 text-white"
              aria-label="הסרת תמונה"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        ))}
        {files.length < max && (
          <button
            type="button"
            onClick={() => input.current?.click()}
            className="flex h-20 min-w-20 flex-col items-center justify-center gap-1 rounded-xl border-2 border-dashed border-brand/40 px-3 text-xs font-medium text-brand-strong hover:bg-brand-soft/40"
          >
            <Camera className="h-5 w-5" />
            {label}
          </button>
        )}
      </div>
      <input
        ref={input}
        type="file"
        accept="image/*"
        capture="environment"
        hidden
        multiple={max > 1}
        onChange={(e) => {
          const picked = Array.from(e.target.files ?? [])
          if (picked.some((f) => f.size > 15 * 1024 * 1024)) toast.error('תמונה גדולה מ־15MB')
          onChange([...files, ...picked.filter((f) => f.size <= 15 * 1024 * 1024)].slice(0, max))
          e.target.value = ''
        }}
      />
    </div>
  )
}

/** Thumbnails of the images attached to an entity (fetched with the auth header). */
export function PhotoGallery({ entity, entityId, title }: { entity: string; entityId: string; title?: string }) {
  const { data = [], isLoading } = useQuery({ queryKey: ['files', entity, entityId], queryFn: () => api.get<Attachment[]>('/files', { entity, entityId }) })
  const images = data.filter((f) => f.contentType.startsWith('image/'))
  if (isLoading) return <Spinner />
  if (images.length === 0) return null
  return (
    <div>
      {title && <p className="mb-1.5 text-xs font-medium text-muted">{title}</p>}
      <div className="flex flex-wrap gap-2">
        {images.map((f) => (
          <Thumb key={f.id} id={f.id} />
        ))}
      </div>
    </div>
  )
}

function Thumb({ id }: { id: string }) {
  const [url, setUrl] = useState<string | null>(null)
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    let alive = true
    let made: string | null = null
    const token = getToken()
    fetch(`/api/files/${id}`, { headers: token ? { Authorization: `Bearer ${token}` } : {} })
      .then((r) => (r.ok ? r.blob() : Promise.reject(new Error(String(r.status)))))
      .then((b) => {
        made = URL.createObjectURL(b)
        if (alive) setUrl(made)
      })
      .catch(() => alive && setFailed(true))
    return () => {
      alive = false
      if (made) URL.revokeObjectURL(made)
    }
  }, [id])
  return (
    <button
      type="button"
      onClick={() => openFile(id).catch((e) => toast.error(errorMessage(e)))}
      className="flex h-20 w-20 items-center justify-center overflow-hidden rounded-xl border border-line bg-subtle"
      aria-label="פתיחת תמונה"
    >
      {url ? <img src={url} alt="" className="h-full w-full object-cover" /> : failed ? <ImageOff className="h-5 w-5 text-muted" /> : <Spinner />}
    </button>
  )
}
