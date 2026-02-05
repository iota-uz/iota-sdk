/**
 * ImageModal Component
 * Full-screen image viewer with navigation for multiple images
 */

import { useState, useEffect } from 'react'
import { X, CaretLeft, CaretRight } from '@phosphor-icons/react'
import type { ImageAttachment } from '../types'

interface ImageModalProps {
  images: ImageAttachment[]
  initialIndex: number
  onClose: () => void
}

export default function ImageModal({ images, initialIndex, onClose }: ImageModalProps) {
  const [currentIndex, setCurrentIndex] = useState(initialIndex)

  const handlePrevious = () => {
    setCurrentIndex((prev) => (prev > 0 ? prev - 1 : images.length - 1))
  }

  const handleNext = () => {
    setCurrentIndex((prev) => (prev < images.length - 1 ? prev + 1 : 0))
  }

  // Keyboard navigation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      } else if (e.key === 'ArrowLeft') {
        handlePrevious()
      } else if (e.key === 'ArrowRight') {
        handleNext()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [onClose])

  // Prevent body scroll when modal is open
  useEffect(() => {
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = ''
    }
  }, [])

  return (
    <div
      className="fixed inset-0 z-50 bg-black/90 flex items-center justify-center"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label="Image viewer"
    >
      {/* Close button */}
      <button
        onClick={onClose}
        className="absolute top-4 right-4 p-2 text-white hover:bg-white/10 rounded-lg transition-colors"
        aria-label="Close image viewer"
      >
        <X size={24} weight="bold" />
      </button>

      {/* Navigation buttons (only show if multiple images) */}
      {images.length > 1 && (
        <>
          <button
            onClick={(e) => {
              e.stopPropagation()
              handlePrevious()
            }}
            className="absolute left-4 p-2 text-white hover:bg-white/10 rounded-lg transition-colors"
            aria-label="Previous image"
          >
            <CaretLeft size={32} weight="bold" />
          </button>
          <button
            onClick={(e) => {
              e.stopPropagation()
              handleNext()
            }}
            className="absolute right-4 p-2 text-white hover:bg-white/10 rounded-lg transition-colors"
            aria-label="Next image"
          >
            <CaretRight size={32} weight="bold" />
          </button>
        </>
      )}

      {/* Image */}
      <img
        src={images[currentIndex].preview}
        alt={images[currentIndex].filename}
        className="max-w-[90vw] max-h-[90vh] object-contain"
        onClick={(e) => e.stopPropagation()}
      />

      {/* Image counter and filename */}
      <div
        className="absolute bottom-4 left-1/2 transform -translate-x-1/2 text-white text-sm bg-black/50 px-4 py-2 rounded-lg backdrop-blur"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="text-center font-medium mb-1">
          {images[currentIndex].filename}
        </div>
        {images.length > 1 && (
          <div className="text-center text-xs opacity-80">
            {currentIndex + 1} / {images.length}
          </div>
        )}
      </div>
    </div>
  )
}
