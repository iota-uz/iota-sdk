import { useEffect } from 'react'

/**
 * Hook to prevent body scroll when modal is open
 * Restores scroll on cleanup or when modal closes
 *
 * @param isOpen - Whether the modal is currently open
 *
 * @example
 * const [isModalOpen, setIsModalOpen] = useState(false)
 * useModalLock(isModalOpen)
 */
export function useModalLock(isOpen: boolean) {
  useEffect(() => {
    if (!isOpen) return

    // Store original scroll position
    const originalOverflow = document.body.style.overflow

    // Prevent scroll
    document.body.style.overflow = 'hidden'

    // Cleanup: restore scroll
    return () => {
      document.body.style.overflow = originalOverflow
    }
  }, [isOpen])
}
