/**
 * ImageModal Component
 * Full-screen image viewer with gallery navigation.
 * Uses @headlessui/react Dialog for accessible modal behavior.
 */

import { useCallback, useEffect, useState } from 'react'
import { Dialog, DialogBackdrop, DialogPanel } from '@headlessui/react'
import { X, CaretLeft, CaretRight, ArrowClockwise, ImageBroken } from '@phosphor-icons/react'
import type { ImageAttachment } from '../types'
import { createDataUrl, formatFileSize } from '../utils/fileUtils'

interface ImageModalProps {
  isOpen: boolean
  onClose: () => void
  attachment: ImageAttachment
  allAttachments?: ImageAttachment[]
  currentIndex?: number
  onNavigate?: (direction: 'prev' | 'next') => void
}

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
    <Dialog open={isOpen} onClose={onClose} className="relative" style={{ zIndex: 99999 }}>
      <DialogBackdrop
        className="fixed inset-0"
        style={{ zIndex: 99999, backgroundColor: 'rgba(0, 0, 0, 0.85)' }}
      />

      <DialogPanel className="fixed inset-0 flex flex-col" style={{ zIndex: 100000 }}>
        {/* ── Top bar ── */}
        <div className="flex items-center justify-between px-4 py-3 shrink-0 bg-white dark:bg-gray-900 border-b border-gray-200 dark:border-gray-800">
          <div className="flex items-center gap-3 min-w-0">
            {hasMultipleImages && (
              <span className="text-xs text-gray-500 dark:text-gray-400 tabular-nums whitespace-nowrap">
                {currentIndex + 1} / {allAttachments?.length}
              </span>
            )}
            <span className="text-sm text-gray-900 dark:text-gray-200 truncate">{attachment.filename}</span>
            <span className="text-xs text-gray-400 dark:text-gray-500 whitespace-nowrap">
              {formatFileSize(attachment.sizeBytes)}
            </span>
          </div>

          <button
            onClick={onClose}
            className="cursor-pointer flex items-center justify-center w-8 h-8 rounded-md bg-gray-100 hover:bg-gray-200 dark:bg-gray-800 dark:hover:bg-gray-700 text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-gray-400"
            aria-label="Close modal"
            type="button"
          >
            <X size={18} weight="bold" />
          </button>
        </div>

        {/* ── Image area (click outside image to close) ── */}
        <div
          className="relative flex-1 flex items-center justify-center min-h-0"
          onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
        >
          {/* Loading spinner */}
          {!isImageLoaded && !imageError && (
            <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
              <div className="flex flex-col items-center gap-3">
                <div className="w-8 h-8 border-2 border-gray-300 dark:border-gray-700 border-t-gray-500 dark:border-t-gray-400 rounded-full animate-spin" />
                <span className="text-xs text-gray-400 dark:text-gray-500">Loading</span>
              </div>
            </div>
          )}

          {/* Error state */}
          {imageError && (
            <div role="alert" className="flex flex-col items-center justify-center text-center max-w-xs">
              <div className="flex items-center justify-center w-16 h-16 rounded-2xl bg-gray-100 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 mb-5">
                <ImageBroken size={28} className="text-gray-400 dark:text-gray-500" weight="duotone" />
              </div>
              <p className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Failed to load image</p>
              <p className="text-xs text-gray-400 dark:text-gray-500 mb-5 truncate max-w-full">{attachment.filename}</p>
              <button
                type="button"
                onClick={handleRetry}
                className="cursor-pointer inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700 border border-gray-200 dark:border-gray-700 rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-gray-400"
                aria-label="Retry loading image"
              >
                <ArrowClockwise size={16} weight="bold" />
                Retry
              </button>
            </div>
          )}

          {/* Image */}
          <img
            key={retryKey}
            src={previewUrl}
            alt={attachment.filename}
            className={[
              'max-w-[90vw] max-h-[calc(100vh-120px)] object-contain select-none',
              'transition-opacity duration-300 ease-out',
              isImageLoaded ? 'opacity-100' : 'opacity-0',
            ].join(' ')}
            onLoad={() => setIsImageLoaded(true)}
            onError={() => setImageError(true)}
            loading="lazy"
            draggable={false}
          />

          {/* ── Navigation arrows ── */}
          {hasMultipleImages && (
            <>
              <button
                onClick={() => onNavigate?.('prev')}
                disabled={!canNavigatePrev || !isImageLoaded || imageError}
                className={[
                  'absolute left-3 top-1/2 -translate-y-1/2',
                  'flex items-center justify-center w-10 h-10 rounded-md',
                  'transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-gray-400',
                  canNavigatePrev && isImageLoaded && !imageError
                    ? 'cursor-pointer bg-white/90 hover:bg-white dark:bg-gray-800/90 dark:hover:bg-gray-700 text-gray-700 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white shadow-sm'
                    : 'bg-white/40 dark:bg-gray-800/40 text-gray-300 dark:text-gray-700 cursor-not-allowed',
                ].join(' ')}
                aria-label="Previous image"
                type="button"
              >
                <CaretLeft size={20} weight="bold" />
              </button>

              <button
                onClick={() => onNavigate?.('next')}
                disabled={!canNavigateNext || !isImageLoaded || imageError}
                className={[
                  'absolute right-3 top-1/2 -translate-y-1/2',
                  'flex items-center justify-center w-10 h-10 rounded-md',
                  'transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-gray-400',
                  canNavigateNext && isImageLoaded && !imageError
                    ? 'cursor-pointer bg-white/90 hover:bg-white dark:bg-gray-800/90 dark:hover:bg-gray-700 text-gray-700 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white shadow-sm'
                    : 'bg-white/40 dark:bg-gray-800/40 text-gray-300 dark:text-gray-700 cursor-not-allowed',
                ].join(' ')}
                aria-label="Next image"
                type="button"
              >
                <CaretRight size={20} weight="bold" />
              </button>
            </>
          )}
        </div>
      </DialogPanel>
    </Dialog>
  )
}

export { ImageModal }
export default ImageModal
