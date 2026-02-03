/**
 * AttachmentGrid Component
 * Displays image attachments in a responsive grid
 * Supports both view-only mode and edit mode (with remove buttons)
 */

import { X } from '@phosphor-icons/react'
import { formatFileSize } from '../utils/fileUtils'
import type { ImageAttachment } from '../types'

interface AttachmentGridProps {
  attachments: ImageAttachment[]
  onRemove?: (index: number) => void
  onView?: (index: number) => void
  className?: string
}

export default function AttachmentGrid({
  attachments,
  onRemove,
  onView,
  className = ''
}: AttachmentGridProps) {
  if (attachments.length === 0) return null

  const isEditable = !!onRemove
  const isViewable = !!onView

  return (
    <div className={`grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2 ${className}`}>
      {attachments.map((attachment, index) => (
        <div key={index} className="relative group">
          <img
            src={attachment.preview}
            alt={attachment.filename}
            className={`w-full h-24 object-cover rounded-lg border border-gray-200 dark:border-gray-700 ${
              isViewable ? 'cursor-pointer hover:opacity-80 transition-opacity' : ''
            }`}
            onClick={() => isViewable && onView(index)}
            role={isViewable ? 'button' : undefined}
            tabIndex={isViewable ? 0 : undefined}
            onKeyDown={(e) => {
              if (isViewable && (e.key === 'Enter' || e.key === ' ')) {
                e.preventDefault()
                onView(index)
              }
            }}
          />

          {isEditable && (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation()
                onRemove(index)
              }}
              className="absolute top-1 right-1 p-1 bg-red-500 hover:bg-red-600 text-white rounded-full opacity-0 group-hover:opacity-100 transition-opacity shadow-md"
              aria-label={`Remove ${attachment.filename}`}
            >
              <X size={16} weight="bold" />
            </button>
          )}

          <div className="mt-1 px-1">
            <div className="text-xs text-gray-600 dark:text-gray-400 truncate" title={attachment.filename}>
              {attachment.filename.length > 20
                ? `${attachment.filename.substring(0, 20)}...`
                : attachment.filename}
            </div>
            <div className="text-xs text-gray-500 dark:text-gray-500">
              {formatFileSize(attachment.sizeBytes)}
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
