/**
 * ScrollToBottomButton Component
 * Floating button to scroll chat to bottom, shown when user scrolls up
 */

import { ArrowDown } from '@phosphor-icons/react'
import { motion, AnimatePresence } from 'framer-motion'

interface ScrollToBottomButtonProps {
  show: boolean
  onClick: () => void
  unreadCount?: number
  disabled?: boolean
}

function ScrollToBottomButton({
  show,
  onClick,
  unreadCount = 0,
  disabled = false,
}: ScrollToBottomButtonProps) {
  return (
    <AnimatePresence>
      {show && (
        <div
          className="absolute bottom-8 z-10 pointer-events-none"
          style={{ left: '50%', transform: 'translateX(-50%)' }}
        >
          <motion.button
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 10 }}
            transition={{ duration: 0.2 }}
            onClick={disabled ? undefined : onClick}
            disabled={disabled}
            className="pointer-events-auto cursor-pointer p-2.5 bg-white dark:bg-gray-800 rounded-full shadow-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 active:bg-gray-100 dark:active:bg-gray-600 transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-900 disabled:opacity-40 disabled:cursor-not-allowed"
            aria-label="Scroll to bottom"
          >
            <div className="relative">
              <ArrowDown size={18} weight="bold" className="text-gray-700 dark:text-gray-300" />

              {/* Unread count badge */}
              {unreadCount > 0 && (
                <span className="absolute -top-2 -right-2 min-w-[18px] h-[18px] bg-primary-600 dark:bg-primary-500 text-white text-xs font-semibold rounded-full flex items-center justify-center px-1">
                  {unreadCount > 99 ? '99+' : unreadCount}
                </span>
              )}
            </div>
          </motion.button>
        </div>
      )}
    </AnimatePresence>
  )
}

export { ScrollToBottomButton }
export default ScrollToBottomButton
