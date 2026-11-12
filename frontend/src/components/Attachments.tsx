import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { FileText, Image as ImageIcon, Paperclip, Trash2 } from 'lucide-react'
import { api, errorMessage, openFile } from '@/lib/api'
import { bytes } from '@/lib/format'
import type { Attachment } from '@/lib/types'
import { AddFileButtons, uploadPhotos, type FileKind } from './Photos'
import { toast } from './toast'
import { IconButton, Spinner } from './ui'

/** Upload/list files stored in Garage S3 for any entity: take a photo or attach a file (image, PDF…). */
export function Attachments({
  entity,
  entityId,
  canDelete = true,
  canAdd = true,
  kind = 'any',
  title = 'קבצים מצורפים',
  empty = 'אין קבצים. ניתן לצלם או לצרף חוזים, תמונות, אישורים (עד 15MB)',
}: {
  entity: string
  entityId: string
  canDelete?: boolean
  canAdd?: boolean
  kind?: FileKind
  title?: string
  empty?: string
}) {
  const qc = useQueryClient()
  const key = ['files', entity, entityId]
  const { data = [], isLoading } = useQuery({ queryKey: key, queryFn: () => api.get<Attachment[]>('/files', { entity, entityId }) })

  const upload = useMutation({
    mutationFn: (files: File[]) => uploadPhotos(entity, entityId, files),
    onSuccess: (_, files) => toast.success(files.length > 1 ? `${files.length} קבצים הועלו` : 'הקובץ הועלה'),
    onError: (e) => toast.error(errorMessage(e)),
    onSettled: () => qc.invalidateQueries({ queryKey: key }),
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.del(`/files/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
    onError: (e) => toast.error(errorMessage(e)),
  })

  return (
    <div className="space-y-2">
      <p className="flex items-center gap-1.5 text-sm font-medium">
        <Paperclip className="h-4 w-4 text-muted" /> {title}
      </p>
      {isLoading ? (
        <Spinner />
      ) : data.length === 0 ? (
        !canAdd && <p className="rounded-xl border border-dashed border-line px-3 py-4 text-center text-xs text-muted">{empty}</p>
      ) : (
        <ul className="divide-y divide-line rounded-xl border border-line">
          {data.map((f) => (
            <li key={f.id} className="flex min-h-12 items-center gap-3 px-3 py-1.5">
              {f.contentType.startsWith('image/') ? <ImageIcon className="h-4 w-4 shrink-0 text-muted" /> : <FileText className="h-4 w-4 shrink-0 text-muted" />}
              <button className="min-w-0 flex-1 truncate py-2 text-start text-sm hover:text-brand" onClick={() => openFile(f.id).catch((e) => toast.error(errorMessage(e)))}>
                {f.filename}
              </button>
              <span className="shrink-0 text-xs text-muted">{bytes(f.sizeBytes)}</span>
              {canDelete && (
                <IconButton label="מחיקה" onClick={() => remove.mutate(f.id)}>
                  <Trash2 className="h-4 w-4" />
                </IconButton>
              )}
            </li>
          ))}
        </ul>
      )}
      {canAdd && (
        <>
          <AddFileButtons kind={kind} busy={upload.isPending} onFiles={(f) => upload.mutate(f)} />
          {data.length === 0 && <p className="text-xs text-muted">{empty}</p>}
        </>
      )}
    </div>
  )
}
