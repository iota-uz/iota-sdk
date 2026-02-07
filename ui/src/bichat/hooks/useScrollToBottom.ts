/**
 * useScrollToBottom Hook
 * Manages scroll-to-bottom functionality with smart auto-scroll
 * Only scrolls if user is near the bottom (within threshold)
 */

import { useRef, useCallback, useState, useEffect } from 'react'

const SCROLL_THRESHOLD = 100 // pixels from bottom to consider "near bottom"

export interface UseScrollToBottomReturn {
  /**
   * Ref to attach to the messages container
   */
  containerRef: React.RefObject<HTMLDivElement>

  /**
   * Whether to show the scroll-to-bottom button
   */
  showScrollButton: boolean

  /**
   * Function to scroll to bottom
   */
  scrollToBottom: () => void
}

/**
 * Hook for managing scroll-to-bottom behavior
 * Automatically scrolls if user is near the bottom
 * Shows button only when scrolled up significantly
 */
export function useScrollToBottom(items: unknown[]): UseScrollToBottomReturn {
  const containerRef = useRef<HTMLDivElement>(null)
  const [showScrollButton, setShowScrollButton] = useState(false)
  const isNearBottomRef = useRef(true)

  /**
   * Scroll to bottom smoothly
   */
  const scrollToBottom = useCallback(() => {
    if (containerRef.current) {
      containerRef.current.scroll({
        top: containerRef.current.scrollHeight,
        behavior: 'smooth',
      })
      isNearBottomRef.current = true
      setShowScrollButton(false)
    }
  }, [])

  /**
   * Handle scroll events to detect when user is near bottom
   */
  const handleScroll = useCallback(() => {
    if (!containerRef.current) return

    const element = containerRef.current
    const { scrollTop, scrollHeight, clientHeight } = element

    // Calculate how far from bottom
    const distanceFromBottom = scrollHeight - scrollTop - clientHeight
    const isNearBottom = distanceFromBottom < SCROLL_THRESHOLD

    isNearBottomRef.current = isNearBottom
    setShowScrollButton(!isNearBottom && scrollHeight > clientHeight)
  }, [])

  /**
   * Auto-scroll when new items arrive (if near bottom)
   */
  useEffect(() => {
    if (isNearBottomRef.current) {
      // Use requestAnimationFrame for smooth scrolling
      const timer = setTimeout(() => {
        scrollToBottom()
      }, 0)
      return () => clearTimeout(timer)
    }
    return undefined
  }, [items, scrollToBottom])

  /**
   * Add scroll listener to container
   */
  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    container.addEventListener('scroll', handleScroll, { passive: true })

    // Initial check - defer to avoid synchronous setState in effect
    const timerId = setTimeout(handleScroll, 0)

    return () => {
      container.removeEventListener('scroll', handleScroll)
      clearTimeout(timerId)
    }
  }, [handleScroll])

  return {
    containerRef,
    showScrollButton,
    scrollToBottom,
  }
}
