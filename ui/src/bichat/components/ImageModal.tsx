/**
 * ImageModal Component
 * Full-screen image viewer with navigation, loading state, and accessibility
 * Uses @headlessui/react Dialog for accessible modal behavior
 */

import { useCallback, useEffect, useState } from 'react'
import { Dialog, DialogBackdrop, DialogPanel } from '@headlessui/react'
import { X, CaretLeft, CaretRight, ArrowClockwise } from '@phosphor-icons/react'
import type { ImageAttachment } from '../types'
import { createDataUrl, formatFileSize } from '../utils/fileUtils'
import LoadingSpinner from './LoadingSpinner'

interface ImageModalProps {
  /** Whether the modal is open */
  isOpen: boolean

  /** Callback to close the modal */
  onClose: () => void

  /** The current attachment to display */
  attachment: ImageAttachment

  /** Optional: all attachments for navigation */
  allAttachments?: ImageAttachment[]

  /** Optional: current index for navigation state */
  currentIndex?: number

  /** Optional: callback for navigation (prev/next) */
  onNavigate?: (direction: 'prev' | 'next') => void
}

/**
 * Full-screen image modal component for viewing attachments
 *
 * Features:
 * - Full-screen overlay with dark backdrop (90% opacity)
 * - Large image display centered on screen
 * - Close button (X) in top-right corner
 * - Image metadata: filename, file size, MIME type
 * - Navigation arrows (left/right) for image carousel
 * - Keyboard support: Escape to close (Dialog), Arrow keys to navigate
 * - Click backdrop to close
 * - Focus trap within modal (Dialog)
 * - Body scroll locked when open (Dialog)
 * - Image loading state with spinner
 * - Navigation disabled at boundaries (not circular)
 */
function ImageModal({
  isOpen,
  onClose,
  attachment,
  allAttachments,
  currentIndex = 0,
  onNavigate,
}: ImageModalProps) {
  const [isImageLoaded, setIsImageLoaded] = useState(false)
  const [imageError, setImageError] = useState(false)
  const [retryKey, setRetryKey] = useState(0)
  const hasMultipleImages = allAttachments && allAttachments.length > 1
  const canNavigatePrev = hasMultipleImages && currentIndex > 0
  const canNavigateNext =
    hasMultipleImages && currentIndex < (allAttachments?.length || 1) - 1

  // Handle arrow key navigation (domain-specific, not handled by Dialog)
  useEffect(() => {
    if (!isOpen) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'ArrowLeft' && onNavigate && canNavigatePrev) {
        onNavigate('prev')
      } else if (e.key === 'ArrowRight' && onNavigate && canNavigateNext) {
        onNavigate('next')
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onNavigate, canNavigatePrev, canNavigateNext])

  // Reset image loading state when attachment changes
  useEffect(() => {
    setIsImageLoaded(false)
    setImageError(false)
  }, [attachment])

  const handleRetry = useCallback(() => {
    setImageError(false)
    setIsImageLoaded(false)
    setRetryKey((k) => k + 1)
  }, [])

  const previewUrl =
    attachment.preview || createDataUrl(attachment.base64Data, attachment.mimeType)

  return (
    <Dialog open={isOpen} onClose={onClose} className="relative z-40">
      {/* Backdrop */}
      <DialogBackdrop className="fixed inset-0 bg-black/90 transition-opacity duration-200" />

      {/* Modal Container */}
      <DialogPanel className="fixed inset-0 flex items-center justify-center z-50 p-4">
        <div className="relative flex flex-col items-center justify-center w-full h-full">
          {/* Close Button */}
          <button
            onClick={onClose}
            className="absolute top-4 right-4 z-50 flex items-center justify-center w-10 h-10 bg-white/10 hover:bg-white/20 text-white rounded-full transition-colors duration-200"
            aria-label="Close modal"
            type="button"
          >
            <X size={24} weight="bold" />
          </button>

          {/* Image Container */}
          <div className="flex flex-col items-center justify-center w-full h-full max-w-[95vw]">
            {/* Image Loading State */}
            {!isImageLoaded && !imageError && (
              <div className="absolute inset-0 flex items-center justify-center" aria-label="Loading image">
                <LoadingSpinner />
              </div>
            )}

            {/* Error State */}
            {imageError && (
              <div role="alert" className="flex flex-col items-center justify-center text-white">
                <p className="text-lg font-medium mb-2">Failed to load image</p>
                <p className="text-sm text-gray-300 mb-4">{attachment.filename}</p>
                <button
                  type="button"
                  onClick={handleRetry}
                  className="flex items-center gap-2 px-4 py-2 bg-white/10 hover:bg-white/20 text-white rounded-lg transition-colors duration-200"
                  aria-label="Retry loading image"
                >
                  <ArrowClockwise size={18} />
                  <span>Retry</span>
                </button>
              </div>
            )}

            {/* Image */}
            <img
              key={retryKey}
              src={previewUrl}
              alt={attachment.filename}
              className={`
                max-w-[95vw] max-h-[85vh] object-contain
                transition-opacity duration-200
                ${isImageLoaded ? 'opacity-100' : 'opacity-0'}
              `}
              onLoad={() => setIsImageLoaded(true)}
              onError={() => setImageError(true)}
              loading="lazy"
            />
          </div>

          {/* Metadata */}
          {isImageLoaded && !imageError && (
            <div className="absolute bottom-0 left-0 right-0 flex flex-col items-center text-white text-center pb-4 bg-gradient-to-t from-black/50 to-transparent pt-8">
              <p className="text-lg font-medium">
                {attachment.filename}
              </p>
              <div className="text-sm text-gray-300 space-x-2">
                <span>{formatFileSize(attachment.sizeBytes)}</span>
                <span>&bull;</span>
                <span>{attachment.mimeType}</span>
              </div>
            </div>
          )}

          {/* Navigation Arrows - Previous */}
          {hasMultipleImages && (
            <button
              onClick={() => onNavigate?.('prev')}
              disabled={!canNavigatePrev || !isImageLoaded || imageError}
              className={`
                absolute left-4 z-40 flex items-center justify-center w-12 h-12
                rounded-full transition-all duration-200
                disabled:opacity-40 disabled:cursor-not-allowed
                ${
                  canNavigatePrev
                    ? 'bg-white/10 hover:bg-white/20 text-white cursor-pointer'
                    : 'bg-white/5 text-white/30 cursor-not-allowed'
                }
              `}
              aria-label="Previous image"
              type="button"
            >
              <CaretLeft size={28} weight="bold" />
            </button>
          )}

          {/* Navigation Arrows - Next */}
          {hasMultipleImages && (
            <button
              onClick={() => onNavigate?.('next')}
              disabled={!canNavigateNext || !isImageLoaded || imageError}
              className={`
                absolute right-4 z-40 flex items-center justify-center w-12 h-12
                rounded-full transition-all duration-200
                disabled:opacity-40 disabled:cursor-not-allowed
                ${
                  canNavigateNext
                    ? 'bg-white/10 hover:bg-white/20 text-white cursor-pointer'
                    : 'bg-white/5 text-white/30 cursor-not-allowed'
                }
              `}
              aria-label="Next image"
              type="button"
            >
              <CaretRight size={28} weight="bold" />
            </button>
          )}

          {/* Image Counter */}
          {hasMultipleImages && (
            <div className="absolute top-4 left-4 bg-white/10 text-white px-3 py-1 rounded-full text-sm">
              {currentIndex + 1} / {allAttachments?.length}
            </div>
          )}
        </div>
      </DialogPanel>
    </Dialog>
  )
}

export { ImageModal }
export default ImageModal
