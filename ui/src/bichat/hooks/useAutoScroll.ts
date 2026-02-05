/**
 * useAutoScroll Hook
 * Manages auto-scroll behavior for chat containers
 */

import { useRef, useState, useCallback, useEffect } from 'react'

export interface UseAutoScrollOptions {
  /** Threshold in pixels from bottom to consider "at bottom" (default: 100) */
  threshold?: number
  /** Smooth scroll behavior (default: true) */
  smooth?: boolean
  /** Callback when scroll position changes */
  onScroll?: (isAtBottom: boolean) => void
}

export interface UseAutoScrollReturn {
  /** Ref to attach to the scrollable container */
  containerRef: React.RefObject<HTMLDivElement>
  /** Whether the container is scrolled to the bottom */
  isAtBottom: boolean
  /** Whether auto-scroll should be active */
  shouldAutoScroll: boolean
  /** Manually scroll to bottom */
  scrollToBottom: (smooth?: boolean) => void
  /** Enable/disable auto-scroll */
  setAutoScroll: (enabled: boolean) => void
  /** Handle scroll event (attach to container if not using ref) */
  handleScroll: (e: React.UIEvent<HTMLDivElement>) => void
}

/**
 * Hook for managing auto-scroll behavior in chat containers
 *
 * @example
 * ```tsx
 * const scroll = useAutoScroll({ threshold: 50 })
 *
 * // Attach to container
 * <div ref={scroll.containerRef} onScroll={scroll.handleScroll}>
 *   {messages.map(msg => <Message key={msg.id} />)}
 * </div>
 *
 * // Scroll button
 * {!scroll.isAtBottom && (
 *   <button onClick={() => scroll.scrollToBottom()}>
 *     Scroll to bottom
 *   </button>
 * )}
 * ```
 */
export function useAutoScroll(options: UseAutoScrollOptions = {}): UseAutoScrollReturn {
  const { threshold = 100, smooth = true, onScroll } = options

  const containerRef = useRef<HTMLDivElement>(null)
  const [isAtBottom, setIsAtBottom] = useState(true)
  const [shouldAutoScroll, setShouldAutoScroll] = useState(true)

  const checkIsAtBottom = useCallback(
    (container: HTMLElement): boolean => {
      const { scrollTop, scrollHeight, clientHeight } = container
      return scrollHeight - scrollTop - clientHeight <= threshold
    },
    [threshold]
  )

  const handleScroll = useCallback(
    (e: React.UIEvent<HTMLDivElement>) => {
      const container = e.currentTarget
      const atBottom = checkIsAtBottom(container)
      setIsAtBottom(atBottom)
      setShouldAutoScroll(atBottom)
      onScroll?.(atBottom)
    },
    [checkIsAtBottom, onScroll]
  )

  const scrollToBottom = useCallback(
    (useSmooth?: boolean) => {
      const container = containerRef.current
      if (!container) return

      const shouldSmooth = useSmooth ?? smooth
      container.scrollTo({
        top: container.scrollHeight,
        behavior: shouldSmooth ? 'smooth' : 'instant',
      })
      setIsAtBottom(true)
      setShouldAutoScroll(true)
    },
    [smooth]
  )

  const setAutoScroll = useCallback((enabled: boolean) => {
    setShouldAutoScroll(enabled)
    if (enabled) {
      // Scroll to bottom immediately when enabling
      const container = containerRef.current
      if (container) {
        container.scrollTo({
          top: container.scrollHeight,
          behavior: 'instant',
        })
        setIsAtBottom(true)
      }
    }
  }, [])

  // Auto-scroll when content changes (using MutationObserver)
  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    const observer = new MutationObserver(() => {
      if (shouldAutoScroll) {
        container.scrollTo({
          top: container.scrollHeight,
          behavior: 'instant',
        })
      }
    })

    observer.observe(container, {
      childList: true,
      subtree: true,
      characterData: true,
    })

    return () => observer.disconnect()
  }, [shouldAutoScroll])

  return {
    containerRef,
    isAtBottom,
    shouldAutoScroll,
    scrollToBottom,
    setAutoScroll,
    handleScroll,
  }
}
