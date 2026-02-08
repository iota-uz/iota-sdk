/**
 * useToast Hook
 * Manages toast notification state
 */

import { useState, useCallback } from 'react'

export type ToastType = 'success' | 'error' | 'info' | 'warning'

export interface ToastItem {
  id: string
  type: ToastType
  message: string
  duration?: number
}

export interface UseToastReturn {
  toasts: ToastItem[]
  success: (msg: string, duration?: number) => void
  error: (msg: string, duration?: number) => void
  info: (msg: string, duration?: number) => void
  warning: (msg: string, duration?: number) => void
  dismiss: (id: string) => void
  dismissAll: () => void
}

/**
 * Generate a unique ID for a toast
 */
function generateId(): string {
  return Math.random().toString(36).substring(7)
}

/**
 * Hook for managing toast notifications
 *
 * @example
 * ```tsx
 * const { toasts, success, error, dismiss } = useToast()
 *
 * // Show a success toast
 * success('Operation completed!')
 *
 * // Show an error toast with custom duration
 * error('Something went wrong', 10000)
 *
 * // Render toasts
 * <ToastContainer toasts={toasts} onDismiss={dismiss} />
 * ```
 */
export function useToast(): UseToastReturn {
  const [toasts, setToasts] = useState<ToastItem[]>([])

  const showToast = useCallback(
    (type: ToastType, message: string, duration?: number) => {
      const id = generateId()
      setToasts((prev) => [...prev, { id, type, message, duration }])
    },
    []
  )

  const dismiss = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  const dismissAll = useCallback(() => {
    setToasts([])
  }, [])

  return {
    toasts,
    success: (msg: string, duration?: number) => showToast('success', msg, duration),
    error: (msg: string, duration?: number) => showToast('error', msg, duration),
    info: (msg: string, duration?: number) => showToast('info', msg, duration),
    warning: (msg: string, duration?: number) => showToast('warning', msg, duration),
    dismiss,
    dismissAll,
  }
}
