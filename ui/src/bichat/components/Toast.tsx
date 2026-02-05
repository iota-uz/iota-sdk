/**
 * Toast Component
 * Individual toast notification with auto-dismiss and accessibility support
 */

import { useEffect } from 'react'
import { motion } from 'framer-motion'
import { CheckCircle, XCircle, Info, Warning, X } from '@phosphor-icons/react'
import type { ToastType } from '../hooks/useToast'

export interface ToastProps {
  id: string
  type: ToastType
  message: string
  duration?: number
  onDismiss: (id: string) => void
  /** Label for dismiss button (defaults to "Dismiss") */
  dismissLabel?: string
}

const typeConfig = {
  success: {
    bgColor: 'bg-green-500',
    darkBgColor: 'dark:bg-green-600',
    icon: <CheckCircle size={20} className="w-5 h-5 flex-shrink-0" weight="fill" />,
  },
  error: {
    bgColor: 'bg-red-500',
    darkBgColor: 'dark:bg-red-600',
    icon: <XCircle size={20} className="w-5 h-5 flex-shrink-0" weight="fill" />,
  },
  info: {
    bgColor: 'bg-blue-500',
    darkBgColor: 'dark:bg-blue-600',
    icon: <Info size={20} className="w-5 h-5 flex-shrink-0" weight="fill" />,
  },
  warning: {
    bgColor: 'bg-yellow-500',
    darkBgColor: 'dark:bg-yellow-600',
    icon: <Warning size={20} className="w-5 h-5 flex-shrink-0" weight="fill" />,
  },
}

export function Toast({
  id,
  type,
  message,
  duration = 5000,
  onDismiss,
  dismissLabel = 'Dismiss',
}: ToastProps) {
  const config = typeConfig[type]

  // Use assertive for errors, polite for others
  const ariaLive = type === 'error' ? 'assertive' : 'polite'
  // Status for info/success, alert for errors/warnings
  const role = type === 'error' || type === 'warning' ? 'alert' : 'status'

  useEffect(() => {
    const timer = setTimeout(() => onDismiss(id), duration)
    return () => clearTimeout(timer)
  }, [id, duration, onDismiss])

  return (
    <motion.div
      initial={{ opacity: 0, y: -50, scale: 0.95 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, scale: 0.95 }}
      transition={{ duration: 0.2 }}
      className={`flex items-center gap-3 px-4 py-3 rounded-lg shadow-lg backdrop-blur-sm min-w-[300px] max-w-[400px] text-white ${config.bgColor} ${config.darkBgColor}`}
      role={role}
      aria-live={ariaLive}
      aria-atomic="true"
    >
      {config.icon}
      <p className="flex-1 text-sm font-medium">{message}</p>
      <button
        onClick={() => onDismiss(id)}
        className="ml-2 text-white hover:bg-white/20 p-1 rounded transition-colors flex-shrink-0"
        aria-label={dismissLabel}
      >
        <X size={16} className="w-4 h-4" weight="bold" />
      </button>
    </motion.div>
  )
}

export default Toast
