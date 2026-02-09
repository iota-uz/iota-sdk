import { useEffect, useState } from 'react'

interface ScreenReaderAnnouncerProps {
  message: string
  politeness?: 'polite' | 'assertive'
  clearAfter?: number
}

/**
 * Screen reader announcer component for live region updates
 * Uses ARIA live regions to announce dynamic content changes
 *
 * @param message - The message to announce
 * @param politeness - 'polite' (wait for pause) or 'assertive' (immediate)
 * @param clearAfter - Optional milliseconds to clear message after announcement
 *
 * @example
 * <ScreenReaderAnnouncer
 *   message="New message received"
 *   politeness="polite"
 * />
 */
export default function ScreenReaderAnnouncer({
  message,
  politeness = 'polite',
  clearAfter,
}: ScreenReaderAnnouncerProps) {
  const [announcement, setAnnouncement] = useState(message)

  useEffect(() => {
    setAnnouncement(message)

    if (clearAfter && message) {
      const timer = setTimeout(() => {
        setAnnouncement('')
      }, clearAfter)
      return () => clearTimeout(timer)
    }
    return undefined
  }, [message, clearAfter])

  return (
    <div
      role="status"
      aria-live={politeness}
      aria-atomic="true"
      className="sr-only"
    >
      {announcement}
    </div>
  )
}
