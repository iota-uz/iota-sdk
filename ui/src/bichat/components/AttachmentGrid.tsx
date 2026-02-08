/**
 * AttachmentGrid Component
 * Displays image and non-image attachments as compact horizontal cards.
 */

import React, { useMemo, useState } from 'react'
import { X, Image as ImageIcon } from '@phosphor-icons/react'
import { formatFileSize, getFileVisual } from '../utils/fileUtils'
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
  /** Number of files currently being processed (shows shimmer placeholders) */
  pendingCount?: number
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

/* ── Shared card styles ─────────────────────────────── */

const CARD_CLS = [
  'group relative flex items-center gap-2.5 rounded-xl',
  'border border-gray-200/80 dark:border-gray-700/60',
  'bg-white dark:bg-gray-800/60',
  'px-2.5 py-2',
  'transition-all duration-150',
].join(' ')

function RemoveButton({ index, onRemove, filename }: { index: number; onRemove: (i: number) => void; filename: string }) {
  return (
    <button
      type="button"
      onClick={(e) => { e.stopPropagation(); onRemove(index) }}
      className="flex-shrink-0 p-1 text-gray-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-md opacity-0 group-hover:opacity-100 transition-all duration-150 cursor-pointer focus-visible:outline-none focus-visible:opacity-100"
      aria-label={`Remove ${filename}`}
    >
      <X size={14} weight="bold" />
    </button>
  )
}

/* ── Shimmer placeholder ───────────────────────────── */

function ShimmerCard() {
  const shimmerStyle = {
    background: 'linear-gradient(110deg, transparent 30%, rgba(255,255,255,0.4) 50%, transparent 70%)',
    backgroundSize: '250% 100%',
    animation: 'attachmentShimmer 1.5s ease-in-out infinite',
  }

  return (
    <div className={CARD_CLS} style={{ pointerEvents: 'none' }}>
      <div className="flex-shrink-0 w-10 h-10 rounded-lg bg-gray-100 dark:bg-gray-700 overflow-hidden">
        <div className="w-full h-full" style={shimmerStyle} />
      </div>
      <div className="flex-1 space-y-1.5">
        <div className="h-3.5 w-28 rounded bg-gray-100 dark:bg-gray-700 overflow-hidden">
          <div className="w-full h-full" style={shimmerStyle} />
        </div>
        <div className="h-3 w-16 rounded bg-gray-100 dark:bg-gray-700 overflow-hidden">
          <div className="w-full h-full" style={shimmerStyle} />
        </div>
      </div>
      <style>{`
        @keyframes attachmentShimmer {
          0% { background-position: 200% 0; }
          100% { background-position: -60% 0; }
        }
      `}</style>
    </div>
  )
}

/* ── Image card ──────────────────────────────────────── */

interface ImageItemProps {
  attachment: Attachment
  index: number
  onRemove?: (index: number) => void
  onView?: (index: number) => void
}

function ImageItem({ attachment, index, onRemove, onView }: ImageItemProps) {
  const previewSrc = resolveImagePreview(attachment)
  const hasPreview = previewSrc !== ''
  const [imgFailed, setImgFailed] = useState(false)

  const thumbnail =
    hasPreview && !imgFailed ? (
      <img
        src={previewSrc}
        alt={attachment.filename}
        onError={() => setImgFailed(true)}
        className="w-10 h-10 rounded-lg object-cover bg-gray-100 dark:bg-gray-700"
      />
    ) : (
      <div className="flex items-center justify-center w-10 h-10 rounded-lg bg-violet-100 dark:bg-violet-900/40">
        <ImageIcon size={20} weight="duotone" className="text-violet-600 dark:text-violet-400" />
      </div>
    )

  return (
    <div className={CARD_CLS}>
      {hasPreview && !imgFailed && onView ? (
        <button
          type="button"
          onClick={() => onView(index)}
          className="flex-shrink-0 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 rounded-lg hover:opacity-80 transition-opacity"
          aria-label={`View ${attachment.filename}`}
        >
          {thumbnail}
        </button>
      ) : (
        <div className="flex-shrink-0">{thumbnail}</div>
      )}

      <div className="flex-1 min-w-0">
        <span className="block text-[13px] font-medium text-gray-900 dark:text-gray-100 truncate">
          {attachment.filename}
        </span>
        <span className="text-[11px] text-gray-400 dark:text-gray-500">
          {formatFileSize(attachment.sizeBytes)}
        </span>
      </div>

      {onRemove && <RemoveButton index={index} onRemove={onRemove} filename={attachment.filename} />}
    </div>
  )
}

/* ── File card (non-image) ──────────────────────────── */

interface FileCardProps {
  attachment: Attachment
  index: number
  onRemove?: (index: number) => void
}

function FileCard({ attachment, index, onRemove }: FileCardProps) {
  const visual = getFileVisual(attachment.mimeType, attachment.filename)
  const Icon = visual.icon

  return (
    <div className={CARD_CLS}>
      <div className={`flex-shrink-0 flex items-center justify-center w-10 h-10 rounded-lg ${visual.bgColor}`}>
        <Icon size={20} weight="duotone" className={visual.iconColor} />
      </div>

      <div className="flex-1 min-w-0">
        <span className="block text-[13px] font-medium text-gray-900 dark:text-gray-100 truncate">
          {attachment.filename}
        </span>
        <span className="text-[11px] text-gray-400 dark:text-gray-500">
          {formatFileSize(attachment.sizeBytes)}
        </span>
      </div>

      {onRemove && <RemoveButton index={index} onRemove={onRemove} filename={attachment.filename} />}
    </div>
  )
}

/* ── Memoization ───────────────────────────────────── */

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

/* ── Grid ──────────────────────────────────────────── */

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
  pendingCount = 0,
}: AttachmentGridProps) {
  const displayedAttachments = useMemo(
    () =>
      maxDisplay && attachments.length > maxDisplay
        ? attachments.slice(0, maxDisplay)
        : attachments,
    [attachments, maxDisplay]
  )

  const isAtMaxCapacity = attachments.length >= maxCapacity
  const hasContent = displayedAttachments.length > 0 || pendingCount > 0

  if (!hasContent) {
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

      <div className="grid gap-2">
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

        {/* Shimmer placeholders for files being processed */}
        {Array.from({ length: pendingCount }).map((_, i) => (
          <ShimmerCard key={`pending-${i}`} />
        ))}
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

const MemoizedAttachmentGrid = React.memo(AttachmentGrid)
MemoizedAttachmentGrid.displayName = 'AttachmentGrid'

export { MemoizedAttachmentGrid as AttachmentGrid }
export default MemoizedAttachmentGrid
