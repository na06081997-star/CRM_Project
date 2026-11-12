import clsx from 'clsx'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Camera, FileText, ImageOff, ImagePlus, Paperclip, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { api, errorMessage, getToken, openFile } from '@/lib/api'
import type { Attachment } from '@/lib/types'
import { toast } from './toast'
import { Button, Modal, Spinner } from './ui'

const MAX_SIDE = 1600
const MAX_BYTES = 15 * 1024 * 1024

/** What a picker takes: photos only, photos + PDF (contracts), or office files too. */
export type FileKind = 'image' | 'doc' | 'any'
const ACCEPT: Record<FileKind, string> = {
  image: 'image/*',
  doc: 'image/*,application/pdf',
  any: 'image/*,application/pdf,.docx,.xlsx',
}
const SERVER_TYPES = ['image/png', 'image/jpeg', 'image/webp', 'image/gif', 'application/pdf', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet']

const isTouch = () => typeof window !== 'undefined' && !!window.matchMedia?.('(pointer: coarse)').matches
const hasWebcam = () => typeof navigator !== 'undefined' && !!navigator.mediaDevices?.getUserMedia

/** Phone photos are large (and iPhones may send HEIC): re-encode to a JPEG of at most MAX_SIDE px. */
async function prepare(file: File): Promise<File> {
  const heic = /image\/hei[cf]/.test(file.type) || /\.hei[cf]$/i.test(file.name)
  if (!file.type.startsWith('image/') && !heic) return file
  if (!heic && file.size < 600_000) return file
  try {
    const bitmap = await createImageBitmap(file)
    const scale = Math.min(1, MAX_SIDE / Math.max(bitmap.width, bitmap.height))
    const canvas = document.createElement('canvas')
    canvas.width = Math.round(bitmap.width * scale)
    canvas.height = Math.round(bitmap.height * scale)
    canvas.getContext('2d')?.drawImage(bitmap, 0, 0, canvas.width, canvas.height)
    const blob = await new Promise<Blob | null>((r) => canvas.toBlob(r, 'image/jpeg', 0.82))
    if (!blob || (!heic && blob.size >= file.size)) return file
    return new File([blob], file.name.replace(/\.\w+$/, '') + '.jpg', { type: 'image/jpeg' })
  } catch {
    if (heic) throw new Error('פורמט HEIC לא נתמך בדפדפן זה – צלמו שוב או שמרו את התמונה כ־JPG')
    return file
  }
}

/** Drops files the server would refuse, with a message. */
function accepted(files: File[]) {
  const ok: File[] = []
  for (const f of files) {
    const heic = /image\/hei[cf]/.test(f.type) || /\.hei[cf]$/i.test(f.name)
    if (f.size > MAX_BYTES) toast.error(`${f.name}: הקובץ גדול מ־15MB`)
    else if (!heic && !SERVER_TYPES.includes(f.type)) toast.error(`${f.name}: סוג קובץ לא נתמך (תמונה, PDF, DOCX, XLSX)`)
    else ok.push(f)
  }
  return ok
}

async function uploadOne(entity: string, entityId: string, file: File) {
  const fd = new FormData()
  fd.set('entity', entity)
  fd.set('entityId', entityId)
  fd.set('file', await prepare(file))
  return api.post<Attachment>('/files', fd)
}

/** Uploads files (stored in Garage S3) to an entity, after it was created. */
export async function uploadPhotos(entity: string, entityId: string, files: File[]) {
  for (const f of files) await uploadOne(entity, entityId, f)
}

/**
 * The two standard buttons: "צילום" opens the camera (phone camera, or the
 * computer's webcam), the other opens the device picker (gallery / files —
 * on a phone it also offers the camera).
 */
export function AddFileButtons({
  onFiles,
  kind = 'image',
  multiple = true,
  busy,
  cameraLabel = 'צילום',
  fileLabel,
  className,
}: {
  onFiles: (files: File[]) => void
  kind?: FileKind
  multiple?: boolean
  busy?: boolean
  cameraLabel?: string
  fileLabel?: string
  className?: string
}) {
  const camera = useRef<HTMLInputElement>(null)
  const picker = useRef<HTMLInputElement>(null)
  const [webcam, setWebcam] = useState(false)
  const touch = isTouch()
  const showCamera = touch || hasWebcam()
  const pick = (list: FileList | null) => {
    const ok = accepted(Array.from(list ?? []))
    if (ok.length) onFiles(multiple ? ok : ok.slice(0, 1))
  }
  const btn =
    'flex min-h-12 items-center justify-center gap-2 rounded-xl border-2 border-dashed border-brand/40 px-3 py-2 text-sm font-medium text-brand-strong transition hover:bg-brand-soft/40 active:scale-[.98] disabled:opacity-50'
  return (
    <div className={clsx('grid gap-2', showCamera ? 'grid-cols-2' : 'grid-cols-1', className)}>
      {showCamera && (
        <button type="button" className={btn} disabled={busy} onClick={() => (touch ? camera.current?.click() : setWebcam(true))}>
          {busy ? <Spinner /> : <Camera className="h-5 w-5 shrink-0" />}
          {cameraLabel}
        </button>
      )}
      <button type="button" className={btn} disabled={busy} onClick={() => picker.current?.click()}>
        {kind === 'image' ? <ImagePlus className="h-5 w-5 shrink-0" /> : <Paperclip className="h-5 w-5 shrink-0" />}
        {fileLabel ?? (kind === 'image' ? (touch ? 'מהגלריה' : 'בחירת תמונה') : touch ? 'צירוף קובץ' : 'בחירת קובץ')}
      </button>
      <input
        ref={camera}
        type="file"
        accept="image/*"
        capture="environment"
        hidden
        onChange={(e) => {
          pick(e.target.files)
          e.target.value = ''
        }}
      />
      <input
        ref={picker}
        type="file"
        accept={ACCEPT[kind]}
        hidden
        multiple={multiple}
        onChange={(e) => {
          pick(e.target.files)
          e.target.value = ''
        }}
      />
      {webcam && (
        <WebcamDialog
          onClose={() => setWebcam(false)}
          onShot={(f) => {
            setWebcam(false)
            onFiles([f])
          }}
          onFallback={() => {
            setWebcam(false)
            picker.current?.click()
          }}
        />
      )}
    </div>
  )
}

/** Computer webcam: live preview and a shutter button. */
function WebcamDialog({ onClose, onShot, onFallback }: { onClose: () => void; onShot: (f: File) => void; onFallback: () => void }) {
  const video = useRef<HTMLVideoElement>(null)
  const [error, setError] = useState('')
  const [ready, setReady] = useState(false)
  useEffect(() => {
    let stream: MediaStream | null = null
    let alive = true
    navigator.mediaDevices
      .getUserMedia({ video: { facingMode: 'environment', width: { ideal: 1600 } }, audio: false })
      .then((s) => {
        if (!alive) return s.getTracks().forEach((t) => t.stop())
        stream = s
        if (video.current) {
          video.current.srcObject = s
          video.current.play().catch(() => {})
        }
      })
      .catch((e: DOMException) => setError(e?.name === 'NotAllowedError' ? 'אין הרשאה למצלמה – אשרו גישה למצלמה בדפדפן ונסו שוב.' : 'לא נמצאה מצלמה במחשב הזה.'))
    return () => {
      alive = false
      stream?.getTracks().forEach((t) => t.stop())
    }
  }, [])
  const shoot = () => {
    const v = video.current
    if (!v || !v.videoWidth) return
    const scale = Math.min(1, MAX_SIDE / Math.max(v.videoWidth, v.videoHeight))
    const canvas = document.createElement('canvas')
    canvas.width = Math.round(v.videoWidth * scale)
    canvas.height = Math.round(v.videoHeight * scale)
    canvas.getContext('2d')?.drawImage(v, 0, 0, canvas.width, canvas.height)
    canvas.toBlob((b) => b && onShot(new File([b], `צילום-${Date.now()}.jpg`, { type: 'image/jpeg' })), 'image/jpeg', 0.85)
  }
  return (
    <Modal
      open
      onClose={onClose}
      title="צילום"
      footer={
        error ? (
          <Button onClick={onFallback} icon={<Paperclip className="h-4 w-4" />}>
            בחירת קובץ מהמחשב
          </Button>
        ) : (
          <Button size="lg" disabled={!ready} onClick={shoot} icon={<Camera className="h-5 w-5" />}>
            צילום
          </Button>
        )
      }
    >
      {error ? (
        <p className="rounded-xl bg-amber-50 p-3 text-sm dark:bg-amber-950/20">{error}</p>
      ) : (
        <div className="relative overflow-hidden rounded-xl bg-slate-900">
          <video ref={video} playsInline muted onLoadedData={() => setReady(true)} className="aspect-[4/3] w-full object-contain" />
          {!ready && (
            <div className="absolute inset-0 flex items-center justify-center text-white">
              <Spinner />
            </div>
          )}
        </div>
      )}
    </Modal>
  )
}

/** Files chosen before the entity exists: previews, remove, then `uploadPhotos` after saving. */
export function PhotoPicker({
  files,
  onChange,
  max = 3,
  kind = 'image',
  cameraLabel,
  fileLabel,
}: {
  files: File[]
  onChange: (f: File[]) => void
  max?: number
  kind?: FileKind
  cameraLabel?: string
  fileLabel?: string
  /** @deprecated kept for older call sites */
  label?: string
}) {
  const [urls, setUrls] = useState<(string | null)[]>([])
  useEffect(() => {
    const u = files.map((f) => (f.type.startsWith('image/') ? URL.createObjectURL(f) : null))
    setUrls(u)
    return () => u.forEach((x) => x && URL.revokeObjectURL(x))
  }, [files])

  return (
    <div className="space-y-2">
      {files.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {files.map((f, i) => (
            <div key={i + f.name} className="relative h-20 w-20 overflow-hidden rounded-xl border border-line bg-subtle">
              {urls[i] ? (
                <img src={urls[i]!} alt="" className="h-full w-full object-cover" />
              ) : (
                <div className="flex h-full flex-col items-center justify-center gap-1 p-1 text-center text-[10px] text-muted">
                  <FileText className="h-6 w-6" />
                  <span className="line-clamp-2 break-all">{f.name}</span>
                </div>
              )}
              <button
                type="button"
                onClick={() => onChange(files.filter((_, j) => j !== i))}
                className="absolute end-1 top-1 flex h-7 w-7 items-center justify-center rounded-full bg-slate-900/70 text-white"
                aria-label="הסרה"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          ))}
        </div>
      )}
      {files.length < max && <AddFileButtons kind={kind} multiple={max - files.length > 1} cameraLabel={cameraLabel} fileLabel={fileLabel} onFiles={(add) => onChange([...files, ...add].slice(0, max))} />}
    </div>
  )
}

/** Images of an entity plus the buttons to add more (uploaded right away). */
export function PhotoUploader({ entity, entityId, title, hint, canAdd = true, kind = 'image' }: { entity: string; entityId: string; title?: string; hint?: string; canAdd?: boolean; kind?: FileKind }) {
  const qc = useQueryClient()
  const upload = useMutation({
    mutationFn: (files: File[]) => uploadPhotos(entity, entityId, files),
    onSuccess: (_, files) => toast.success(files.length > 1 ? `${files.length} קבצים נשמרו` : 'הקובץ נשמר'),
    onError: (e) => toast.error(errorMessage(e)),
    onSettled: () => qc.invalidateQueries({ queryKey: ['files', entity, entityId] }),
  })
  return (
    <div className="space-y-2">
      {title && <p className="text-sm font-medium">{title}</p>}
      {hint && <p className="-mt-1 text-xs text-muted">{hint}</p>}
      <PhotoGallery entity={entity} entityId={entityId} />
      {canAdd && <AddFileButtons kind={kind} busy={upload.isPending} onFiles={(f) => upload.mutate(f)} />}
    </div>
  )
}

/** Thumbnails of the images attached to an entity (fetched with the auth header). */
export function PhotoGallery({ entity, entityId, title }: { entity: string; entityId: string; title?: string }) {
  const { data = [], isLoading } = useQuery({ queryKey: ['files', entity, entityId], queryFn: () => api.get<Attachment[]>('/files', { entity, entityId }) })
  const shown = data.filter((f) => f.contentType.startsWith('image/') || f.contentType === 'application/pdf')
  if (isLoading) return <Spinner />
  if (shown.length === 0) return null
  return (
    <div>
      {title && <p className="mb-1.5 text-xs font-medium text-muted">{title}</p>}
      <div className="flex flex-wrap gap-2">
        {shown.map((f) => (f.contentType.startsWith('image/') ? <Thumb key={f.id} id={f.id} /> : <DocThumb key={f.id} file={f} />))}
      </div>
    </div>
  )
}

function DocThumb({ file }: { file: Attachment }) {
  return (
    <button
      type="button"
      onClick={() => openFile(file.id).catch((e) => toast.error(errorMessage(e)))}
      className="flex h-20 w-20 flex-col items-center justify-center gap-1 overflow-hidden rounded-xl border border-line bg-subtle p-1 text-center text-[10px] text-muted"
      aria-label={`פתיחת ${file.filename}`}
    >
      <FileText className="h-6 w-6" />
      <span className="line-clamp-2 break-all">{file.filename}</span>
    </button>
  )
}

export function Thumb({ id }: { id: string }) {
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
