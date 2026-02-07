/**
 * AttachmentGrid Component
 * Displays image and non-image attachments in a responsive grid.
 */

import React, { useMemo } from 'react'
import { File, X } from '@phosphor-icons/react'
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

function fileTypeLabel(mimeType: string): string {
  const normalized = mimeType.toLowerCase()
  if (normalized.startsWith('text/')) return 'TEXT'
  if (normalized.includes('pdf')) return 'PDF'
  if (normalized.includes('excel') || normalized.includes('spreadsheet')) return 'XLS'
  if (normalized.includes('wordprocessingml') || normalized.includes('msword')) return 'DOC'
  if (normalized.includes('json')) return 'JSON'
  if (normalized.includes('xml')) return 'XML'
  if (normalized.includes('yaml')) return 'YAML'
  return (normalized.split('/')[1] || 'FILE').toUpperCase()
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

      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2">
        {displayedAttachments.map((attachment, index) => (
          <MemoizedAttachmentItem
            key={`${attachment.id || attachment.filename}-${index}`}
            attachment={attachment}
            index={index}
            onRemove={isEditable ? onRemove : undefined}
            onView={onView}
          />
        ))}
      </div>

      {maxDisplay && attachments.length > maxDisplay && (
        <div className="text-sm text-gray-500 dark:text-gray-400">
          +{attachments.length - maxDisplay} more
        </div>
      )}

      {isAtMaxCapacity && isEditable && (
        <div className="text-sm text-amber-600 dark:text-amber-400">
          Maximum {maxCapacity} files
        </div>
      )}
    </div>
  )
}

interface AttachmentItemProps {
  attachment: Attachment
  index: number
  onRemove?: (index: number) => void
  onView?: (index: number) => void
}

function AttachmentItem({ attachment, index, onRemove, onView }: AttachmentItemProps) {
  const isEditable = !!onRemove
  const isImage = isImageAttachment(attachment)
  const previewSrc = resolveImagePreview(attachment)
  const isImageViewable = isImage && previewSrc !== '' && !!onView

  return (
    <div className="relative group">
      {isImage && previewSrc !== '' ? (
        isImageViewable ? (
          <button
            type="button"
            onClick={() => onView?.(index)}
            className="w-full cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-900 rounded-lg"
            aria-label={`View ${attachment.filename}`}
          >
            <img
              src={previewSrc}
              alt={attachment.filename}
              className="w-full h-24 object-cover rounded-lg border border-gray-200 dark:border-gray-700 hover:opacity-80 transition-opacity duration-150"
            />
          </button>
        ) : (
          <img
            src={previewSrc}
            alt={attachment.filename}
            className="w-full h-24 object-cover rounded-lg border border-gray-200 dark:border-gray-700"
          />
        )
      ) : (
        <div className="w-full h-24 rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 p-2 flex flex-col justify-between">
          <div className="flex items-center gap-1.5 text-gray-600 dark:text-gray-300">
            <File size={16} />
            <span className="text-[10px] font-semibold tracking-wide">
              {fileTypeLabel(attachment.mimeType)}
            </span>
          </div>
          <div className="text-[11px] text-gray-500 dark:text-gray-400 truncate" title={attachment.filename}>
            {attachment.filename}
          </div>
        </div>
      )}

      {isEditable && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation()
            onRemove?.(index)
          }}
          className="absolute top-1 right-1 p-1.5 bg-red-500 hover:bg-red-600 active:bg-red-700 text-white rounded-full opacity-0 group-hover:opacity-100 transition-all duration-150 shadow-md focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/50"
          aria-label={`Remove ${attachment.filename}`}
        >
          <X size={16} weight="bold" />
        </button>
      )}

      <div className="mt-1 px-1">
        <div
          className="text-xs text-gray-600 dark:text-gray-400 truncate"
          title={attachment.filename}
        >
          {attachment.filename.length > 20
            ? `${attachment.filename.substring(0, 20)}...`
            : attachment.filename}
        </div>
        <div className="text-xs text-gray-500 dark:text-gray-500">
          {formatFileSize(attachment.sizeBytes)}
        </div>
      </div>
    </div>
  )
}

const MemoizedAttachmentItem = React.memo(
  AttachmentItem,
  (prevProps, nextProps) => {
    return (
      prevProps.attachment.id === nextProps.attachment.id &&
      prevProps.attachment.filename === nextProps.attachment.filename &&
      prevProps.attachment.preview === nextProps.attachment.preview &&
      prevProps.attachment.base64Data === nextProps.attachment.base64Data &&
      prevProps.attachment.url === nextProps.attachment.url &&
      prevProps.index === nextProps.index &&
      prevProps.onRemove === nextProps.onRemove &&
      prevProps.onView === nextProps.onView
    )
  }
)

const MemoizedAttachmentGrid = React.memo(AttachmentGrid)
MemoizedAttachmentGrid.displayName = 'AttachmentGrid'

export { MemoizedAttachmentGrid as AttachmentGrid }
export default MemoizedAttachmentGrid
