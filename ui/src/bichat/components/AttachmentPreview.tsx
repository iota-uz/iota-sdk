/**
 * AttachmentPreview Component
 * Displays thumbnail preview of an image attachment
 * Shows filename, size, remove button, and supports click-to-enlarge
 */

import { memo, useState } from 'react'
import { X } from '@phosphor-icons/react'
import { ImageAttachment } from '../types'
import { formatFileSize, createDataUrl } from '../utils/fileUtils'

interface AttachmentPreviewProps {
  /** The attachment to display */
  attachment: ImageAttachment
  /** Optional callback when remove button is clicked */
  onRemove?: () => void
  /** Optional callback when thumbnail is clicked (for enlargement) */
  onClick?: () => void
  /** If true, hide remove button and disable click interactions */
  readonly?: boolean
}

const AttachmentPreview = memo<AttachmentPreviewProps>(({ attachment, onRemove, onClick, readonly = false }) => {
  const [isImageLoaded, setIsImageLoaded] = useState(false)
  const [imageError, setImageError] = useState(false)

  const previewUrl = attachment.preview || createDataUrl(attachment.base64Data, attachment.mimeType)
  const isClickable = onClick !== undefined && !readonly
  const showRemoveButton = onRemove !== undefined && !readonly

  return (
    <div
      className={`
        relative
        rounded-lg
        border border-gray-200 dark:border-gray-700
        bg-white dark:bg-gray-800
        p-2
        transition-all
        duration-200
        ${isClickable ? 'cursor-pointer hover:shadow-md hover:border-primary-400 dark:hover:border-primary-500' : ''}
        ${!isClickable ? 'hover:shadow-sm' : ''}
      `}
      onClick={isClickable ? onClick : undefined}
      role={isClickable ? 'button' : undefined}
      tabIndex={isClickable ? 0 : undefined}
      onKeyDown={
        isClickable
          ? (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                onClick?.()
              }
            }
          : undefined
      }
    >
      {/* Thumbnail Container */}
      <div className="relative mb-2 overflow-hidden rounded-md bg-gray-100 dark:bg-gray-700 aspect-square">
        {/* Loading Skeleton */}
        {!isImageLoaded && !imageError && (
          <div className="absolute inset-0 animate-pulse bg-gray-200 dark:bg-gray-600" />
        )}

        {/* Error State */}
        {imageError && (
          <div className="absolute inset-0 flex items-center justify-center bg-gray-100 dark:bg-gray-700">
            <span className="text-xs text-gray-500 dark:text-gray-400">Preview unavailable</span>
          </div>
        )}

        {/* Image */}
        <img
          src={previewUrl}
          alt={attachment.filename}
          className={`
            w-full h-full object-cover
            transition-opacity duration-200
            ${isImageLoaded ? 'opacity-100' : 'opacity-0'}
            ${isClickable ? 'group-hover:scale-105' : ''}
          `}
          onLoad={() => setIsImageLoaded(true)}
          onError={() => setImageError(true)}
        />
      </div>

      {/* Filename */}
      {!isImageLoaded && !imageError ? (
        <div className="h-3 w-3/4 bg-gray-200 dark:bg-gray-600 rounded animate-pulse mb-1" />
      ) : (
        <p
          className="text-xs font-medium text-gray-700 dark:text-gray-300 truncate"
          title={attachment.filename}
        >
          {attachment.filename}
        </p>
      )}

      {/* File Size */}
      {!isImageLoaded && !imageError ? (
        <div className="h-3 w-1/2 bg-gray-200 dark:bg-gray-600 rounded animate-pulse mb-1" />
      ) : (
        <p className="text-xs text-gray-500 dark:text-gray-400 mb-1">
          {formatFileSize(attachment.sizeBytes)}
        </p>
      )}

      {/* Remove Button */}
      {showRemoveButton && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation()
            onRemove?.()
          }}
          className="absolute top-1 right-1 flex items-center justify-center bg-red-500 hover:bg-red-600 dark:bg-red-600 dark:hover:bg-red-700 text-white rounded-full transition-all duration-200 shadow-sm hover:shadow-md active:scale-90 w-6 h-6"
          aria-label={`Remove ${attachment.filename}`}
          title="Remove attachment"
        >
          <X size={14} className="w-3.5 h-3.5" weight="bold" />
        </button>
      )}
    </div>
  )
})

AttachmentPreview.displayName = 'AttachmentPreview'

export default AttachmentPreview
