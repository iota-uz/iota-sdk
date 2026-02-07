/**
 * Alert Component
 * Standardized error/success/warning/info messages with retry capability
 */

import { memo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { X, Warning, CheckCircle, Info, XCircle } from '@phosphor-icons/react'
import { errorMessageVariants } from '../animations/variants'
import { useTranslation } from '../hooks/useTranslation'

export type AlertVariant = 'error' | 'success' | 'warning' | 'info'

interface AlertProps {
  variant?: AlertVariant
  message: string
  title?: string
  onDismiss?: () => void
  onRetry?: () => void
  show?: boolean
  dismissible?: boolean
}

const variantStyles = {
  error: {
    container: 'border-red-200 bg-red-50 dark:bg-red-900/20',
    title: 'text-red-800 dark:text-red-300',
    message: 'text-red-700 dark:text-red-400',
    icon: 'text-red-600 dark:text-red-400',
    button: 'text-red-400 hover:text-red-600 dark:hover:text-red-300',
    retryButton: 'bg-red-600 dark:bg-red-700 hover:bg-red-700 dark:hover:bg-red-800 text-white',
    Icon: XCircle,
  },
  success: {
    container: 'border-green-200 bg-green-50 dark:bg-green-900/20',
    title: 'text-green-800 dark:text-green-300',
    message: 'text-green-700 dark:text-green-400',
    icon: 'text-green-600 dark:text-green-400',
    button: 'text-green-400 hover:text-green-600 dark:hover:text-green-300',
    retryButton: 'bg-green-600 dark:bg-green-700 hover:bg-green-700 dark:hover:bg-green-800 text-white',
    Icon: CheckCircle,
  },
  warning: {
    container: 'border-yellow-200 bg-yellow-50 dark:bg-yellow-900/20',
    title: 'text-yellow-800 dark:text-yellow-300',
    message: 'text-yellow-700 dark:text-yellow-400',
    icon: 'text-yellow-600 dark:text-yellow-400',
    button: 'text-yellow-400 hover:text-yellow-600 dark:hover:text-yellow-300',
    retryButton: 'bg-yellow-600 dark:bg-yellow-700 hover:bg-yellow-700 dark:hover:bg-yellow-800 text-white',
    Icon: Warning,
  },
  info: {
    container: 'border-blue-200 bg-blue-50 dark:bg-blue-900/20',
    title: 'text-blue-800 dark:text-blue-300',
    message: 'text-blue-700 dark:text-blue-400',
    icon: 'text-blue-600 dark:text-blue-400',
    button: 'text-blue-400 hover:text-blue-600 dark:hover:text-blue-300',
    retryButton: 'bg-blue-600 dark:bg-blue-700 hover:bg-blue-700 dark:hover:bg-blue-800 text-white',
    Icon: Info,
  },
}

function Alert({
  variant = 'info',
  message,
  title,
  onDismiss,
  onRetry,
  show = true,
  dismissible = true,
}: AlertProps) {
  const { t } = useTranslation()
  const styles = variantStyles[variant]
  const IconComponent = styles.Icon

  return (
    <AnimatePresence>
      {show && (
        <motion.div
          variants={errorMessageVariants}
          initial="initial"
          animate="animate"
          exit="exit"
          className={`border-t border ${styles.container} px-4 py-3`}
          role="alert"
          aria-live="assertive"
        >
          <div className="w-full flex items-start justify-between px-4">
            <div className="flex items-start gap-3 flex-1">
              {/* Icon */}
              <IconComponent size={20} className={`w-5 h-5 ${styles.icon} flex-shrink-0 mt-0.5`} />

              {/* Content */}
              <div className="flex-1">
                {title && <p className={`text-sm ${styles.title} font-medium`}>{title}</p>}
                <p className={`text-sm ${styles.message} ${title ? 'mt-1' : ''}`}>{message}</p>

                {/* Retry Button */}
                {onRetry && (
                  <button
                    onClick={onRetry}
                    className={`mt-2 text-xs px-3 py-1.5 rounded ${styles.retryButton} transition-colors font-medium`}
                  >
                    {t('chat.retry')}
                  </button>
                )}
              </div>
            </div>

            {/* Dismiss Button */}
            {dismissible && onDismiss && (
              <button
                onClick={onDismiss}
                className={`${styles.button} transition-colors flex-shrink-0`}
                aria-label={t('chat.dismissNotification')}
              >
                <X size={20} className="w-5 h-5" />
              </button>
            )}
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}

export default memo(Alert)
