/**
 * AttachmentGrid Component
 * Displays image and non-image attachments as compact chips/thumbnails.
 */

import React, { useMemo } from 'react'
import {
  File,
  FilePdf,
  FileDoc,
  FileXls,
  FileText,
  FileCode,
  X,
} from '@phosphor-icons/react'
import { formatFileSize } from '../utils/fileUtils'
import type { Attachment } from '../types'

interface AttachmentGridProps {
  attachments: Attachment[]
  onRemove?: (index: number) => void
  onView?: (index: number) => void
  className?: string
  readonly?: boolean
  maxDisplay?: number
  maxCapacity?: number
  emptyMessage?: string
  showCount?: boolean
}

function isImageAttachment(attachment: Attachment): boolean {
  return attachment.mimeType.toLowerCase().startsWith('image/')
}

function resolveImagePreview(attachment: Attachment): string {
  if (attachment.preview) {
    return attachment.preview
  }
  if (!isImageAttachment(attachment)) {
    return ''
  }
  if (attachment.base64Data) {
    if (attachment.base64Data.startsWith('data:')) {
      return attachment.base64Data
    }
    return `data:${attachment.mimeType};base64,${attachment.base64Data}`
  }
  return attachment.url || ''
}

interface FileTypeMeta {
  label: string
  icon: typeof File
  iconColor: string
}

function fileTypeMeta(mimeType: string): FileTypeMeta {
  const n = mimeType.toLowerCase()
  if (n.includes('pdf'))
    return { label: 'PDF', icon: FilePdf, iconColor: 'text-red-500 dark:text-red-400' }
  if (n.includes('excel') || n.includes('spreadsheet'))
    return { label: 'XLS', icon: FileXls, iconColor: 'text-emerald-600 dark:text-emerald-400' }
  if (n.includes('wordprocessingml') || n.includes('msword'))
    return { label: 'DOC', icon: FileDoc, iconColor: 'text-blue-600 dark:text-blue-400' }
  if (n.includes('json') || n.includes('xml') || n.includes('yaml') || n.includes('javascript') || n.includes('typescript'))
    return { label: n.includes('json') ? 'JSON' : n.includes('xml') ? 'XML' : n.includes('yaml') ? 'YAML' : 'CODE', icon: FileCode, iconColor: 'text-violet-500 dark:text-violet-400' }
  if (n.startsWith('text/') || n.includes('csv'))
    return { label: n.includes('csv') ? 'CSV' : 'TEXT', icon: FileText, iconColor: 'text-gray-500 dark:text-gray-400' }
  return { label: (n.split('/')[1] || 'FILE').toUpperCase().slice(0, 4), icon: File, iconColor: 'text-gray-500 dark:text-gray-400' }
}

function AttachmentGrid({
  attachments,
  onRemove,
  onView,
  className = '',
  readonly = false,
  maxDisplay,
  maxCapacity = 10,
  emptyMessage = 'No files attached',
  showCount = false,
}: AttachmentGridProps) {
  const displayedAttachments = useMemo(
    () =>
      maxDisplay && attachments.length > maxDisplay
        ? attachments.slice(0, maxDisplay)
        : attachments,
    [attachments, maxDisplay]
  )

  const isAtMaxCapacity = attachments.length >= maxCapacity

  if (displayedAttachments.length === 0) {
    if (!showCount) return null
    return (
      <div className="text-center text-gray-500 dark:text-gray-400 py-4">{emptyMessage}</div>
    )
  }

  const isEditable = !readonly && !!onRemove

  return (
    <div className={`space-y-2 ${className}`}>
      {showCount && (
        <div className="text-sm text-gray-600 dark:text-gray-400">
          {displayedAttachments.length} file{displayedAttachments.length !== 1 ? 's' : ''} attached
        </div>
      )}

      <div className="flex flex-wrap gap-2">
        {displayedAttachments.map((attachment, index) => {
          const isImage = isImageAttachment(attachment) && resolveImagePreview(attachment)
          return isImage ? (
            <MemoizedImageItem
              key={`${attachment.id || attachment.filename}-${index}`}
              attachment={attachment}
              index={index}
              onRemove={isEditable ? onRemove : undefined}
              onView={onView}
            />
          ) : (
            <MemoizedFileCard
              key={`${attachment.id || attachment.filename}-${index}`}
              attachment={attachment}
              index={index}
              onRemove={isEditable ? onRemove : undefined}
            />
          )
        })}
      </div>

      {maxDisplay && attachments.length > maxDisplay && (
        <div className="text-xs text-gray-500 dark:text-gray-400">
          +{attachments.length - maxDisplay} more
        </div>
      )}

      {isAtMaxCapacity && isEditable && (
        <div className="text-xs text-amber-600 dark:text-amber-400">
          Maximum {maxCapacity} files
        </div>
      )}
    </div>
  )
}

/* ── Image thumbnail ────────────────────────────────── */

interface ImageItemProps {
  attachment: Attachment
  index: number
  onRemove?: (index: number) => void
  onView?: (index: number) => void
}

function ImageItem({ attachment, index, onRemove, onView }: ImageItemProps) {
  const previewSrc = resolveImagePreview(attachment)
  const isViewable = previewSrc !== '' && !!onView

  const img = (
    <img
      src={previewSrc}
      alt={attachment.filename}
      className="w-16 h-16 object-cover rounded-lg border border-gray-200 dark:border-gray-700"
    />
  )

  return (
    <div className="w-16">
      <div className="relative group" title={`${attachment.filename} (${formatFileSize(attachment.sizeBytes)})`}>
        {isViewable ? (
          <button
            type="button"
            onClick={() => onView?.(index)}
            className="cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 rounded-lg hover:opacity-80 transition-opacity duration-150"
            aria-label={`View ${attachment.filename}`}
          >
            {img}
          </button>
        ) : (
          img
        )}
        {onRemove && (
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); onRemove(index) }}
            className="absolute -top-1.5 -right-1.5 p-0.5 bg-gray-700/80 hover:bg-red-500 text-white rounded-full opacity-0 group-hover:opacity-100 transition-all duration-150 shadow-sm focus-visible:opacity-100 focus-visible:outline-none cursor-pointer"
            aria-label={`Remove ${attachment.filename}`}
          >
            <X size={12} weight="bold" />
          </button>
        )}
      </div>
      <p className="mt-1 text-[10px] text-gray-500 dark:text-gray-400 truncate" title={attachment.filename}>
        {attachment.filename}
      </p>
    </div>
  )
}

/* ── File card (non-image, same size as image thumbnails) ── */

interface FileCardProps {
  attachment: Attachment
  index: number
  onRemove?: (index: number) => void
}

function FileCard({ attachment, index, onRemove }: FileCardProps) {
  const meta = fileTypeMeta(attachment.mimeType)
  const Icon = meta.icon

  return (
    <div className="w-16">
      <div
        className="relative group w-16 h-16 rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 flex flex-col items-center justify-center gap-1"
        title={`${attachment.filename} (${formatFileSize(attachment.sizeBytes)})`}
      >
        <Icon size={20} weight="duotone" className={meta.iconColor} />
        <span className={`text-[9px] font-bold tracking-wider ${meta.iconColor}`}>
          {meta.label}
        </span>
        {onRemove && (
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); onRemove(index) }}
            className="absolute -top-1.5 -right-1.5 p-0.5 bg-gray-700/80 hover:bg-red-500 text-white rounded-full opacity-0 group-hover:opacity-100 transition-all duration-150 shadow-sm focus-visible:opacity-100 focus-visible:outline-none cursor-pointer"
            aria-label={`Remove ${attachment.filename}`}
          >
            <X size={12} weight="bold" />
          </button>
        )}
      </div>
      <p className="mt-1 text-[10px] text-gray-500 dark:text-gray-400 truncate" title={attachment.filename}>
        {attachment.filename}
      </p>
    </div>
  )
}

const attachmentEq = (a: Attachment, b: Attachment) =>
  a.id === b.id &&
  a.filename === b.filename &&
  a.preview === b.preview &&
  a.base64Data === b.base64Data &&
  a.url === b.url

const MemoizedImageItem = React.memo(ImageItem, (prev, next) =>
  attachmentEq(prev.attachment, next.attachment) &&
  prev.index === next.index &&
  prev.onRemove === next.onRemove &&
  prev.onView === next.onView
)

const MemoizedFileCard = React.memo(FileCard, (prev, next) =>
  attachmentEq(prev.attachment, next.attachment) &&
  prev.index === next.index &&
  prev.onRemove === next.onRemove
)

const MemoizedAttachmentGrid = React.memo(AttachmentGrid)
MemoizedAttachmentGrid.displayName = 'AttachmentGrid'

export { MemoizedAttachmentGrid as AttachmentGrid }
export default MemoizedAttachmentGrid
