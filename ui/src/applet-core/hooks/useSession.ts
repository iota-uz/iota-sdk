import { useMemo } from 'react'
import { useAppletContext } from '../context/AppletContext'
import type { SessionHook } from '../types'

/**
 * useSession provides session and authentication handling utilities.
 *
 * Usage:
 * const { isExpiringSoon, refreshSession, csrfToken } = useSession()
 *
 * // Check if session is expiring soon (5 min buffer)
 * if (isExpiringSoon) {
 *   await refreshSession()
 * }
 *
 * // Include CSRF token in requests
 * fetch('/api/endpoint', {
 *   headers: { 'X-CSRF-Token': csrfToken }
 * })
 */
export function useSession(): SessionHook {
  const { session } = useAppletContext()

  // Check if session is expiring soon (5 minute buffer)
  const isExpiringSoon = useMemo(() => {
    const bufferMs = 5 * 60 * 1000 // 5 minutes
    return session.expiresAt - Date.now() < bufferMs
  }, [session.expiresAt])

  const refreshSession = async (): Promise<void> => {
    const response = await fetch(session.refreshURL, {
      method: 'POST',
      headers: {
        'X-CSRF-Token': session.csrfToken
      }
    })

    if (!response.ok) {
      // Session refresh failed - redirect to login with return URL
      const returnUrl = encodeURIComponent(window.location.pathname)
      window.location.href = `/login?redirect=${returnUrl}`
      return
    }

    // Dispatch event for CSRF token update
    const newToken = response.headers.get('X-CSRF-Token')
    if (newToken) {
      window.dispatchEvent(
        new CustomEvent('iota:csrf-refresh', {
          detail: { token: newToken }
        })
      )
    }
  }

  return {
    isExpiringSoon,
    refreshSession,
    csrfToken: session.csrfToken,
    expiresAt: session.expiresAt
  }
}
