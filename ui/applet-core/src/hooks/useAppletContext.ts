import type { InitialContext } from '../types'

/**
 * useAppletContext provides direct access to the window global context.
 * This is a standalone version that doesn't require AppletProvider.
 *
 * Usage:
 * const context = useAppletContext('__BICHAT_CONTEXT__')
 *
 * Note: Prefer using AppletProvider + context hooks for better type safety
 * and testability. Use this hook only when provider setup is not possible.
 */
export function useAppletContext<T = InitialContext>(windowKey: string): T {
  const context = (window as any)[windowKey]

  if (!context) {
    throw new Error(`${windowKey} not found on window. Ensure backend context injection is working.`)
  }

  return context as T
}
