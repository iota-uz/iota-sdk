/**
 * AttachmentGrid Component
 * Displays image attachments in a responsive grid
 *
 * Features:
 * - Responsive grid (2 cols mobile, 3 tablet, 4 desktop)
 * - View-only or edit mode (with remove buttons)
 * - Optional maxDisplay limit
 * - Max capacity indicator (10 images)
 * - Memoized items for performance
 * - Readonly mode support
 */

import React, { useMemo } from 'react'
import { X } from '@phosphor-icons/react'
import { formatFileSize } from '../utils/fileUtils'
import type { ImageAttachment } from '../types'

interface AttachmentGridProps {
  /** Array of image attachments to display */
  attachments: ImageAttachment[]
  /** Optional callback when remove button is clicked */
  onRemove?: (index: number) => void
  /** Optional callback when thumbnail is clicked for preview */
  onView?: (index: number) => void
  /** Additional CSS class */
  className?: string
  /** If true, disable all interactions */
  readonly?: boolean
  /** Maximum number of attachments to display (default: all) */
  maxDisplay?: number
  /** Maximum total capacity (for warning display, default: 10) */
  maxCapacity?: number
  /** Empty state message */
  emptyMessage?: string
  /** Show count label above grid */
  showCount?: boolean
}

/**
 * Responsive grid component for displaying image attachments
 *
 * Layout:
 * - Mobile: 2 columns (grid-cols-2)
 * - Tablet: 3 columns (sm:grid-cols-3)
 * - Desktop: 4 columns (md:grid-cols-4)
 */
function AttachmentGrid({
  attachments,
  onRemove,
  onView,
  className = '',
  readonly = false,
  maxDisplay,
  maxCapacity = 10,
  emptyMessage = 'No images attached',
  showCount = false,
}: AttachmentGridProps) {
  // Limit attachments to maxDisplay if specified
  const displayedAttachments = useMemo(
    () =>
      maxDisplay && attachments.length > maxDisplay
        ? attachments.slice(0, maxDisplay)
        : attachments,
    [attachments, maxDisplay]
  )

  // Determine if we're at maximum capacity
  const isAtMaxCapacity = attachments.length >= maxCapacity

  // Return null for truly empty state (no empty message needed in most cases)
  if (displayedAttachments.length === 0) {
    if (!showCount) return null
    return (
      <div className="text-center text-gray-500 dark:text-gray-400 py-4">{emptyMessage}</div>
    )
  }

  const isEditable = !readonly && !!onRemove
  const isViewable = !readonly && !!onView

  return (
    <div className={`space-y-2 ${className}`}>
      {/* Count label */}
      {showCount && (
        <div className="text-sm text-gray-600 dark:text-gray-400">
          {displayedAttachments.length} image{displayedAttachments.length !== 1 ? 's' : ''} attached
        </div>
      )}

      {/* Grid container */}
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2">
        {displayedAttachments.map((attachment, index) => (
          <MemoizedAttachmentItem
            key={`${attachment.filename}-${index}`}
            attachment={attachment}
            index={index}
            onRemove={isEditable ? onRemove : undefined}
            onView={isViewable ? onView : undefined}
          />
        ))}
      </div>

      {/* Overflow indicator when maxDisplay is set */}
      {maxDisplay && attachments.length > maxDisplay && (
        <div className="text-sm text-gray-500 dark:text-gray-400">
          +{attachments.length - maxDisplay} more
        </div>
      )}

      {/* Maximum capacity indicator */}
      {isAtMaxCapacity && isEditable && (
        <div className="text-sm text-amber-600 dark:text-amber-400">
          Maximum {maxCapacity} images
        </div>
      )}
    </div>
  )
}

/**
 * Individual attachment preview item
 */
interface AttachmentItemProps {
  attachment: ImageAttachment
  index: number
  onRemove?: (index: number) => void
  onView?: (index: number) => void
}

function AttachmentItem({ attachment, index, onRemove, onView }: AttachmentItemProps) {
  const isEditable = !!onRemove
  const isViewable = !!onView

  return (
    <div className="relative group">
      {isViewable ? (
        <button
          type="button"
          onClick={() => onView?.(index)}
          className="w-full cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-900 rounded-lg"
          aria-label={`View ${attachment.filename}`}
        >
          <img
            src={attachment.preview}
            alt={attachment.filename}
            className="w-full h-24 object-cover rounded-lg border border-gray-200 dark:border-gray-700 hover:opacity-80 transition-opacity duration-150"
          />
        </button>
      ) : (
        <img
          src={attachment.preview}
          alt={attachment.filename}
          className="w-full h-24 object-cover rounded-lg border border-gray-200 dark:border-gray-700"
        />
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

/**
 * Memoized AttachmentItem to prevent unnecessary re-renders
 * Only re-renders when the attachment or callbacks actually change
 */
const MemoizedAttachmentItem = React.memo(
  AttachmentItem,
  (prevProps, nextProps) => {
    // Custom equality check: only re-render if attachment content or callbacks change
    return (
      prevProps.attachment.base64Data === nextProps.attachment.base64Data &&
      prevProps.attachment.filename === nextProps.attachment.filename &&
      prevProps.attachment.preview === nextProps.attachment.preview &&
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
