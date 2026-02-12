import { createContext, useContext, type ReactNode } from 'react'
import { ToastContainer, useToast, type UseToastReturn } from '@iota-uz/sdk/bichat'

const ToastContext = createContext<UseToastReturn | null>(null)

export function ToastProvider({ children }: { children: ReactNode }) {
  const toast = useToast()

  return (
    <ToastContext.Provider value={toast}>
      {children}
      <ToastContainer toasts={toast.toasts} onDismiss={toast.dismiss} />
    </ToastContext.Provider>
  )
}

export function useAppToast(): UseToastReturn {
  const ctx = useContext(ToastContext)
  if (!ctx) {
    throw new Error('useAppToast must be used within ToastProvider')
  }
  return ctx
}

