/**
 * Archive Banner Component
 * Displays when a chat session is archived and provides a restore button
 */

import { memo, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Archive, Spinner } from '@phosphor-icons/react'
import { errorMessageVariants } from '../animations/variants'
import Alert from './Alert'

interface ArchiveBannerProps {
  show?: boolean
  onRestore?: () => Promise<void>
  restoring?: boolean
  onRestoreComplete?: () => void
}

function ArchiveBanner({
  show = true,
  onRestore,
  restoring = false,
  onRestoreComplete,
}: ArchiveBannerProps) {
  const [error, setError] = useState<string | null>(null)

  const handleRestore = async () => {
    try {
      setError(null)
      if (onRestore) {
        await onRestore()
      }
      if (onRestoreComplete) {
        onRestoreComplete()
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to restore session'
      setError(message)
    }
  }

  return (
    <>
      <AnimatePresence>
        {show && (
          <motion.div
            variants={errorMessageVariants}
            initial="initial"
            animate="animate"
            exit="exit"
            className="border-t border border-blue-200 bg-blue-50 dark:bg-blue-900/20 px-4 py-3"
            role="region"
            aria-label="Archive banner"
          >
            <div className="w-full flex items-start justify-between px-4">
              <div className="flex items-start gap-3 flex-1">
                {/* Icon */}
                <Archive size={20} className="w-5 h-5 text-blue-600 dark:text-blue-400 flex-shrink-0 mt-0.5" />

                {/* Content */}
                <div className="flex-1">
                  <p className="text-sm text-blue-700 dark:text-blue-400">
                    This chat session has been archived
                  </p>
                </div>
              </div>

              {/* Restore Button */}
              <button
                onClick={handleRestore}
                disabled={restoring}
                className="ml-2 flex-shrink-0 px-3 py-1.5 text-xs font-medium bg-blue-600 dark:bg-blue-700 hover:bg-blue-700 dark:hover:bg-blue-800 text-white rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1.5"
                aria-label="Restore session"
              >
                {restoring ? (
                  <>
                    <Spinner size={16} className="w-4 h-4 animate-spin" />
                    Restoring...
                  </>
                ) : (
                  'Restore'
                )}
              </button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Error Alert */}
      {error && (
        <Alert
          variant="error"
          message={error}
          title="Restore Failed"
          onDismiss={() => setError(null)}
          dismissible
        />
      )}
    </>
  )
}

export default memo(ArchiveBanner)
