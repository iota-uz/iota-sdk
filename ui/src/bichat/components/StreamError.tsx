/**
 * StreamError Component
 * Error recovery UI for streaming failures
 */

import { motion } from 'framer-motion'
import { Warning, ArrowClockwise, ArrowsCounterClockwise } from '@phosphor-icons/react'
import { useTranslation } from '../hooks/useTranslation'

interface StreamErrorProps {
  /** Error message to display */
  error: string
  /** Callback to retry the failed operation */
  onRetry?: () => void
  /** Callback to regenerate the message */
  onRegenerate?: () => void
  /** Whether to show compact mode (less padding) */
  compact?: boolean
}

export function StreamError({
  error,
  onRetry,
  onRegenerate,
  compact = false,
}: StreamErrorProps) {
  const { t } = useTranslation()

  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -10 }}
      className={`flex items-center gap-3 ${compact ? 'p-3' : 'p-4'} bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg`}
      role="alert"
    >
      <Warning
        className="w-5 h-5 text-red-500 dark:text-red-400 flex-shrink-0"
        weight="fill"
      />
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-red-800 dark:text-red-200">
          {t('error.generic')}
        </p>
        <p className="text-sm text-red-600 dark:text-red-300 break-words">
          {error}
        </p>
      </div>
      <div className="flex items-center gap-2 flex-shrink-0">
        {onRetry && (
          <button
            onClick={onRetry}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-red-700 dark:text-red-200 bg-red-100 dark:bg-red-800/50 hover:bg-red-200 dark:hover:bg-red-800 rounded-md transition-colors"
            type="button"
          >
            <ArrowClockwise className="w-4 h-4" />
            {t('streamError.retry')}
          </button>
        )}
        {onRegenerate && (
          <button
            onClick={onRegenerate}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-200 bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-md transition-colors"
            type="button"
          >
            <ArrowsCounterClockwise className="w-4 h-4" />
            {t('streamError.regenerate')}
          </button>
        )}
      </div>
    </motion.div>
  )
}
