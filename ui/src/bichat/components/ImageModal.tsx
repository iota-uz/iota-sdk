/**
 * ImageModal Component
 * Full-screen image viewer with navigation, loading state, and accessibility
 */

import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { X, CaretLeft, CaretRight } from '@phosphor-icons/react'
import type { ImageAttachment } from '../types'
import { createDataUrl, formatFileSize } from '../utils/fileUtils'
import { useModalLock } from '../hooks/useModalLock'
import { useFocusTrap } from '../hooks/useFocusTrap'
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
 * - Keyboard support: Escape to close, Arrow keys to navigate
 * - Click backdrop to close
 * - Focus trap within modal
 * - Body scroll locked when open
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
  const modalRef = useRef<HTMLDivElement>(null)
  const [isImageLoaded, setIsImageLoaded] = useState(false)
  const [imageError, setImageError] = useState(false)
  const hasMultipleImages = allAttachments && allAttachments.length > 1
  const canNavigatePrev = hasMultipleImages && currentIndex > 0
  const canNavigateNext =
    hasMultipleImages && currentIndex < (allAttachments?.length || 1) - 1

  // Lock body scroll when modal is open
  useModalLock(isOpen)

  // Trap focus within modal
  useFocusTrap(modalRef, isOpen)

  // Handle keyboard events
  useEffect(() => {
    if (!isOpen) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      } else if (e.key === 'ArrowLeft' && onNavigate && canNavigatePrev) {
        onNavigate('prev')
      } else if (e.key === 'ArrowRight' && onNavigate && canNavigateNext) {
        onNavigate('next')
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onClose, onNavigate, canNavigatePrev, canNavigateNext])

  // Reset image loading state when attachment changes
  useEffect(() => {
    setIsImageLoaded(false)
    setImageError(false)
  }, [attachment])

  if (!isOpen) return null

  const previewUrl =
    attachment.preview || createDataUrl(attachment.base64Data, attachment.mimeType)
  return createPortal(
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-black/90 transition-opacity duration-200 z-40"
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Modal Container */}
      <div
        className="fixed inset-0 flex items-center justify-center z-50 p-4"
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-image-title"
        aria-describedby="modal-image-description"
      >
        <div
          ref={modalRef}
          className="relative flex flex-col items-center justify-center w-full h-full"
        >
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
          <div className="flex flex-col items-center justify-center w-full h-full max-w-4xl">
            {/* Image Loading State */}
            {!isImageLoaded && !imageError && (
              <div className="absolute inset-0 flex items-center justify-center">
                <LoadingSpinner />
              </div>
            )}

            {/* Error State */}
            {imageError && (
              <div className="flex flex-col items-center justify-center text-white">
                <p className="text-lg font-medium mb-2">Failed to load image</p>
                <p className="text-sm text-gray-300">{attachment.filename}</p>
              </div>
            )}

            {/* Image */}
            <img
              src={previewUrl}
              alt={attachment.filename}
              className={`
                max-w-4xl max-h-screen object-contain
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
              <p id="modal-image-title" className="text-lg font-medium">
                {attachment.filename}
              </p>
              <div id="modal-image-description" className="text-sm text-gray-300 space-x-2">
                <span>{formatFileSize(attachment.sizeBytes)}</span>
                <span>•</span>
                <span>{attachment.mimeType}</span>
              </div>
            </div>
          )}

          {/* Navigation Arrows - Previous */}
          {hasMultipleImages && (
            <button
              onClick={() => onNavigate?.('prev')}
              disabled={!canNavigatePrev}
              className={`
                absolute left-4 z-40 flex items-center justify-center w-12 h-12
                rounded-full transition-all duration-200
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
              disabled={!canNavigateNext}
              className={`
                absolute right-4 z-40 flex items-center justify-center w-12 h-12
                rounded-full transition-all duration-200
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
      </div>
    </>,
    document.body
  )
}

export { ImageModal }
export default ImageModal
