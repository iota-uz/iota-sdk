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
}

export default function ScrollToBottomButton({
  show,
  onClick,
  unreadCount = 0
}: ScrollToBottomButtonProps) {
  return (
    <AnimatePresence>
      {show && (
        <motion.button
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: 10 }}
          transition={{ duration: 0.2 }}
          onClick={onClick}
          className="absolute bottom-24 right-8 p-3 bg-white dark:bg-gray-800 rounded-full shadow-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors z-10"
          aria-label="Scroll to bottom"
        >
          <div className="relative">
            <ArrowDown size={20} weight="bold" className="text-gray-700 dark:text-gray-300" />

            {/* Unread count badge */}
            {unreadCount > 0 && (
              <span className="absolute -top-2 -right-2 min-w-[18px] h-[18px] bg-primary-600 dark:bg-primary-500 text-white text-xs font-semibold rounded-full flex items-center justify-center px-1">
                {unreadCount > 99 ? '99+' : unreadCount}
              </span>
            )}
          </div>
        </motion.button>
      )}
    </AnimatePresence>
  )
}
