/**
 * useImageGallery Hook
 * Manages image modal/gallery state and navigation
 */

import { useState, useCallback, useMemo } from 'react'
import type { ImageAttachment } from '../types'

export interface UseImageGalleryOptions {
  /** Initial images to display */
  images?: ImageAttachment[]
  /** Wrap navigation at boundaries (default: false) */
  wrap?: boolean
  /** Callback when modal opens */
  onOpen?: (index: number) => void
  /** Callback when modal closes */
  onClose?: () => void
  /** Callback when navigation occurs */
  onNavigate?: (index: number, direction: 'prev' | 'next') => void
}

export interface UseImageGalleryReturn {
  /** Whether the gallery modal is open */
  isOpen: boolean
  /** Current image index */
  currentIndex: number
  /** Current image (or undefined if none) */
  currentImage: ImageAttachment | undefined
  /** All images in the gallery */
  images: ImageAttachment[]
  /** Whether there's a previous image */
  hasPrev: boolean
  /** Whether there's a next image */
  hasNext: boolean
  /** Open gallery at specific index */
  open: (index: number, newImages?: ImageAttachment[]) => void
  /** Close the gallery */
  close: () => void
  /** Navigate to previous image */
  prev: () => void
  /** Navigate to next image */
  next: () => void
  /** Navigate to specific index */
  goTo: (index: number) => void
  /** Set images without opening */
  setImages: (images: ImageAttachment[]) => void
}

/**
 * Hook for managing image gallery/modal state
 *
 * @example
 * ```tsx
 * const gallery = useImageGallery({ images: attachments })
 *
 * // Open gallery
 * <button onClick={() => gallery.open(0)}>View Images</button>
 *
 * // Render gallery
 * {gallery.isOpen && (
 *   <ImageModal
 *     image={gallery.currentImage}
 *     onClose={gallery.close}
 *     onPrev={gallery.prev}
 *     onNext={gallery.next}
 *     hasPrev={gallery.hasPrev}
 *     hasNext={gallery.hasNext}
 *   />
 * )}
 * ```
 */
export function useImageGallery(options: UseImageGalleryOptions = {}): UseImageGalleryReturn {
  const { images: initialImages = [], wrap = false, onOpen, onClose, onNavigate } = options

  const [isOpen, setIsOpen] = useState(false)
  const [currentIndex, setCurrentIndex] = useState(0)
  const [images, setImages] = useState<ImageAttachment[]>(initialImages)

  const currentImage = useMemo(() => images[currentIndex], [images, currentIndex])

  const hasPrev = useMemo(() => {
    if (wrap) return images.length > 1
    return currentIndex > 0
  }, [currentIndex, images.length, wrap])

  const hasNext = useMemo(() => {
    if (wrap) return images.length > 1
    return currentIndex < images.length - 1
  }, [currentIndex, images.length, wrap])

  const open = useCallback(
    (index: number, newImages?: ImageAttachment[]) => {
      if (newImages) {
        setImages(newImages)
      }
      const targetImages = newImages || images
      const safeIndex = Math.max(0, Math.min(index, targetImages.length - 1))
      setCurrentIndex(safeIndex)
      setIsOpen(true)
      onOpen?.(safeIndex)
    },
    [images, onOpen]
  )

  const close = useCallback(() => {
    setIsOpen(false)
    onClose?.()
  }, [onClose])

  const prev = useCallback(() => {
    if (images.length < 2) return
    if (!hasPrev && !wrap) return

    setCurrentIndex((current) => {
      const newIndex = wrap
        ? (current - 1 + images.length) % images.length
        : Math.max(0, current - 1)
      onNavigate?.(newIndex, 'prev')
      return newIndex
    })
  }, [hasPrev, wrap, images.length, onNavigate])

  const next = useCallback(() => {
    if (images.length < 2) return
    if (!hasNext && !wrap) return

    setCurrentIndex((current) => {
      const newIndex = wrap ? (current + 1) % images.length : Math.min(images.length - 1, current + 1)
      onNavigate?.(newIndex, 'next')
      return newIndex
    })
  }, [hasNext, wrap, images.length, onNavigate])

  const goTo = useCallback(
    (index: number) => {
      const safeIndex = Math.max(0, Math.min(index, images.length - 1))
      setCurrentIndex(safeIndex)
    },
    [images.length]
  )

  const setImagesHandler = useCallback((newImages: ImageAttachment[]) => {
    setImages(newImages)
    // Reset index if it's out of bounds
    setCurrentIndex((current) => Math.min(current, Math.max(0, newImages.length - 1)))
  }, [])

  return {
    isOpen,
    currentIndex,
    currentImage,
    images,
    hasPrev,
    hasNext,
    open,
    close,
    prev,
    next,
    goTo,
    setImages: setImagesHandler,
  }
}
