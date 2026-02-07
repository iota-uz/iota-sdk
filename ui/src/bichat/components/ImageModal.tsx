/**
 * ImageModal Component
 * Cinema-grade full-screen image viewer with gallery navigation,
 * loading shimmer, glassmorphic controls, and elegant transitions.
 * Uses @headlessui/react Dialog for accessible modal behavior.
 */

import { useCallback, useEffect, useState } from 'react'
import { Dialog, DialogBackdrop, DialogPanel } from '@headlessui/react'
import { X, CaretLeft, CaretRight, ArrowClockwise, ImageBroken } from '@phosphor-icons/react'
import type { ImageAttachment } from '../types'
import { createDataUrl, formatFileSize } from '../utils/fileUtils'

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
 * - Full-screen overlay with cinematic dark backdrop
 * - Glassmorphic controls with backdrop-blur
 * - Large image display centered on screen with subtle drop shadow
 * - Close button (X) in top-right corner
 * - Image metadata: filename, file size, MIME type
 * - Navigation arrows (left/right) for image carousel
 * - Keyboard support: Escape to close (Dialog), Arrow keys to navigate
 * - Click backdrop to close
 * - Focus trap within modal (Dialog)
 * - Body scroll locked when open (Dialog)
 * - Loading shimmer animation
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
      {/* Backdrop — deep black with grain-like noise via radial gradient layers */}
      <DialogBackdrop
        className="fixed inset-0 bg-black/[0.92] transition-opacity duration-300"
        style={{
          backgroundImage:
            'radial-gradient(ellipse at 20% 50%, rgba(30,58,138,0.06) 0%, transparent 70%), radial-gradient(ellipse at 80% 50%, rgba(30,58,138,0.04) 0%, transparent 70%)',
        }}
      />

      {/* Modal Container */}
      <DialogPanel className="fixed inset-0 flex items-center justify-center z-50 p-4 sm:p-8">
        <div className="relative flex flex-col items-center justify-center w-full h-full">

          {/* ── Top Bar: counter (left) + close (right) ── */}
          <div className="absolute top-3 sm:top-5 left-3 sm:left-5 right-3 sm:right-5 flex items-center justify-between z-50 pointer-events-none">
            {/* Image Counter — glassmorphic pill */}
            {hasMultipleImages ? (
              <div className="pointer-events-auto inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-white/[0.08] backdrop-blur-xl border border-white/[0.1] text-white/80 text-xs font-medium tabular-nums select-none shadow-lg">
                <span className="text-white">{currentIndex + 1}</span>
                <span className="text-white/40">/</span>
                <span>{allAttachments?.length}</span>
              </div>
            ) : (
              <div />
            )}

            {/* Close Button — glassmorphic circle */}
            <button
              onClick={onClose}
              className="pointer-events-auto cursor-pointer flex items-center justify-center w-9 h-9 rounded-full bg-white/[0.08] backdrop-blur-xl border border-white/[0.1] text-white/70 hover:text-white hover:bg-white/[0.15] transition-all duration-200 shadow-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/30"
              aria-label="Close modal"
              type="button"
            >
              <X size={18} weight="bold" />
            </button>
          </div>

          {/* ── Image Container ── */}
          <div className="flex flex-col items-center justify-center w-full h-full max-w-[95vw]">
            {/* Loading shimmer skeleton */}
            {!isImageLoaded && !imageError && (
              <div className="absolute inset-0 flex items-center justify-center" aria-label="Loading image">
                <div className="relative w-64 h-48 sm:w-80 sm:h-56 rounded-2xl overflow-hidden">
                  <div className="absolute inset-0 bg-white/[0.04] border border-white/[0.06] rounded-2xl" />
                  <div
                    className="absolute inset-0 rounded-2xl"
                    style={{
                      background: 'linear-gradient(110deg, transparent 25%, rgba(255,255,255,0.04) 37%, rgba(255,255,255,0.08) 50%, rgba(255,255,255,0.04) 63%, transparent 75%)',
                      backgroundSize: '250% 100%',
                      animation: 'imageModalShimmer 2s ease-in-out infinite',
                    }}
                  />
                  <div className="absolute inset-0 flex flex-col items-center justify-center gap-3">
                    <div className="w-8 h-8 border-2 border-white/20 border-t-white/60 rounded-full animate-spin" />
                    <span className="text-xs text-white/30 font-medium tracking-wide">Loading</span>
                  </div>
                </div>
              </div>
            )}

            {/* Error State */}
            {imageError && (
              <div role="alert" className="flex flex-col items-center justify-center text-center max-w-xs">
                <div className="flex items-center justify-center w-16 h-16 rounded-2xl bg-white/[0.06] border border-white/[0.08] mb-5">
                  <ImageBroken size={28} className="text-white/40" weight="duotone" />
                </div>
                <p className="text-sm font-medium text-white/80 mb-1">Failed to load image</p>
                <p className="text-xs text-white/35 mb-5 truncate max-w-full">{attachment.filename}</p>
                <button
                  type="button"
                  onClick={handleRetry}
                  className="cursor-pointer inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white/80 bg-white/[0.08] hover:bg-white/[0.14] border border-white/[0.1] rounded-xl backdrop-blur-xl transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/30"
                  aria-label="Retry loading image"
                >
                  <ArrowClockwise size={16} weight="bold" />
                  Retry
                </button>
              </div>
            )}

            {/* Image — with subtle drop shadow for floating effect */}
            <img
              key={retryKey}
              src={previewUrl}
              alt={attachment.filename}
              className={[
                'max-w-[95vw] max-h-[82vh] object-contain rounded-sm select-none',
                'transition-all duration-300 ease-out',
                isImageLoaded ? 'opacity-100 scale-100' : 'opacity-0 scale-[0.97]',
              ].join(' ')}
              style={isImageLoaded ? { filter: 'drop-shadow(0 8px 40px rgba(0,0,0,0.5))' } : undefined}
              onLoad={() => setIsImageLoaded(true)}
              onError={() => setImageError(true)}
              loading="lazy"
              draggable={false}
            />
          </div>

          {/* ── Bottom Metadata Bar — gradient fade with refined type ── */}
          {isImageLoaded && !imageError && (
            <div className="absolute bottom-0 left-0 right-0 flex flex-col items-center text-center pb-5 pt-16 bg-gradient-to-t from-black/60 via-black/25 to-transparent pointer-events-none select-none">
              <p className="text-sm font-medium text-white/90 tracking-tight max-w-md truncate px-4">
                {attachment.filename}
              </p>
              <div className="flex items-center gap-1.5 mt-1 text-[11px] text-white/40 font-medium">
                <span>{formatFileSize(attachment.sizeBytes)}</span>
                <span className="w-0.5 h-0.5 rounded-full bg-white/25" />
                <span>{attachment.mimeType.replace('image/', '').toUpperCase()}</span>
              </div>
            </div>
          )}

          {/* ── Navigation Arrows — glassmorphic, vertically centered ── */}
          {hasMultipleImages && (
            <>
              {/* Previous */}
              <button
                onClick={() => onNavigate?.('prev')}
                disabled={!canNavigatePrev || !isImageLoaded || imageError}
                className={[
                  'absolute left-3 sm:left-5 top-1/2 -translate-y-1/2 z-40',
                  'flex items-center justify-center w-10 h-10 sm:w-11 sm:h-11 rounded-full',
                  'backdrop-blur-xl border transition-all duration-200',
                  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/30',
                  canNavigatePrev && isImageLoaded && !imageError
                    ? 'cursor-pointer bg-white/[0.08] border-white/[0.1] text-white/70 hover:bg-white/[0.16] hover:text-white hover:scale-105 hover:shadow-lg active:scale-95'
                    : 'bg-white/[0.03] border-white/[0.05] text-white/15 cursor-not-allowed',
                ].join(' ')}
                aria-label="Previous image"
                type="button"
              >
                <CaretLeft size={20} weight="bold" />
              </button>

              {/* Next */}
              <button
                onClick={() => onNavigate?.('next')}
                disabled={!canNavigateNext || !isImageLoaded || imageError}
                className={[
                  'absolute right-3 sm:right-5 top-1/2 -translate-y-1/2 z-40',
                  'flex items-center justify-center w-10 h-10 sm:w-11 sm:h-11 rounded-full',
                  'backdrop-blur-xl border transition-all duration-200',
                  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/30',
                  canNavigateNext && isImageLoaded && !imageError
                    ? 'cursor-pointer bg-white/[0.08] border-white/[0.1] text-white/70 hover:bg-white/[0.16] hover:text-white hover:scale-105 hover:shadow-lg active:scale-95'
                    : 'bg-white/[0.03] border-white/[0.05] text-white/15 cursor-not-allowed',
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

      {/* Shimmer keyframe — injected once */}
      <style>{`
        @keyframes imageModalShimmer {
          0% { background-position: 200% 0; }
          100% { background-position: -60% 0; }
        }
      `}</style>
    </Dialog>
  )
}

export { ImageModal }
export default ImageModal
