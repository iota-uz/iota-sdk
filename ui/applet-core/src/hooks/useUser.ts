import { useAppletContext } from '../context/AppletContext'
import type { UserContext } from '../types'

/**
 * useUser provides access to current user information.
 *
 * Usage:
 * const { id, email, firstName, lastName, permissions } = useUser()
 */
export function useUser(): UserContext {
  const { user } = useAppletContext()
  return user
}
