/**
 * RetryActionArea Component
 * Displays a retry action area inline where the assistant message would appear
 * (typically after an interrupted request or connection loss)
 *
 * Styled to match assistant message positioning (left-aligned) so users see
 * the retry button contextually in the conversation flow.
 */

import { memo } from 'react'
import { motion } from 'framer-motion'
import { ArrowClockwise, Warning } from '@phosphor-icons/react'
import { useTranslation } from '../hooks/useTranslation'

interface RetryActionAreaProps {
  /** Callback when retry button is clicked */
  onRetry: () => void
}

export const RetryActionArea = memo(function RetryActionArea({
  onRetry,
}: RetryActionAreaProps) {
  const { t } = useTranslation()

  return (
    // Wrapper matches TurnBubble layout for assistant messages (justify-start = left-aligned)
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -10 }}
      transition={{ duration: 0.2 }}
      className="flex justify-start"
    >
      {/* Bubble styled like AssistantTurnView message bubble */}
      <div
        className="flex flex-col gap-3 max-w-2xl rounded-2xl px-5 py-3 shadow-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700"
        role="status"
        aria-live="polite"
      >
        <div className="flex items-center gap-3">
          <Warning
            className="w-5 h-5 text-amber-500 dark:text-amber-400 flex-shrink-0"
            weight="fill"
          />
          <span className="text-sm text-gray-700 dark:text-gray-300">
            {t('retry.description')}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={onRetry}
            className="cursor-pointer inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium text-white bg-primary-600 hover:bg-primary-700 dark:bg-primary-700 dark:hover:bg-primary-600 rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-800"
            aria-label={t('retry.title')}
          >
            <ArrowClockwise size={16} className="w-4 h-4" />
            {t('retry.button')}
          </button>
        </div>
      </div>
    </motion.div>
  )
})

export default RetryActionArea
