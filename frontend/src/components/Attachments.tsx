import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { FileText, Image as ImageIcon, Paperclip, Trash2, Upload } from 'lucide-react'
import { useRef } from 'react'
import { api, errorMessage, openFile } from '@/lib/api'
import { bytes } from '@/lib/format'
import type { Attachment } from '@/lib/types'
import { toast } from './toast'
import { Button, IconButton, Spinner } from './ui'

/** Upload/list files stored in Garage S3 for any entity. */
export function Attachments({ entity, entityId, canDelete = true }: { entity: string; entityId: string; canDelete?: boolean }) {
  const qc = useQueryClient()
  const input = useRef<HTMLInputElement>(null)
  const key = ['files', entity, entityId]
  const { data = [], isLoading } = useQuery({ queryKey: key, queryFn: () => api.get<Attachment[]>('/files', { entity, entityId }) })

  const upload = useMutation({
    mutationFn: (file: File) => {
      const fd = new FormData()
      fd.set('entity', entity)
      fd.set('entityId', entityId)
      fd.set('file', file)
      return api.post<Attachment>('/files', fd)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: key })
      toast.success('הקובץ הועלה')
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.del(`/files/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
    onError: (e) => toast.error(errorMessage(e)),
  })

  return (
    <div>
      <div className="mb-2 flex items-center justify-between">
        <p className="flex items-center gap-1.5 text-sm font-medium">
          <Paperclip className="h-4 w-4 text-muted" /> קבצים מצורפים
        </p>
        <Button size="sm" variant="secondary" icon={<Upload className="h-3.5 w-3.5" />} loading={upload.isPending} onClick={() => input.current?.click()}>
          העלאה
        </Button>
        <input
          ref={input}
          type="file"
          hidden
          accept="image/png,image/jpeg,image/webp,application/pdf,.docx,.xlsx"
          onChange={(e) => {
            const f = e.target.files?.[0]
            if (f) upload.mutate(f)
            e.target.value = ''
          }}
        />
      </div>
      {isLoading ? (
        <Spinner />
      ) : data.length === 0 ? (
        <p className="rounded-xl border border-dashed border-line px-3 py-4 text-center text-xs text-muted">אין קבצים. ניתן לצרף חוזים, תמונות, אישורים (עד 15MB)</p>
      ) : (
        <ul className="divide-y divide-line rounded-xl border border-line">
          {data.map((f) => (
            <li key={f.id} className="flex items-center gap-3 px-3 py-2">
              {f.contentType.startsWith('image/') ? <ImageIcon className="h-4 w-4 text-muted" /> : <FileText className="h-4 w-4 text-muted" />}
              <button className="min-w-0 flex-1 truncate text-start text-sm hover:text-brand" onClick={() => openFile(f.id).catch((e) => toast.error(errorMessage(e)))}>
                {f.filename}
              </button>
              <span className="text-xs text-muted">{bytes(f.sizeBytes)}</span>
              {canDelete && (
                <IconButton label="מחיקה" onClick={() => remove.mutate(f.id)}>
                  <Trash2 className="h-4 w-4" />
                </IconButton>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
