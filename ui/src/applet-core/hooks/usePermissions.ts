import { useAppletContext } from '../context/AppletContext'
import type { PermissionsHook } from '../types'

/**
 * usePermissions provides permission checking utilities.
 * All user permissions are automatically passed from backend.
 *
 * Usage:
 * const { hasPermission, hasAnyPermission } = usePermissions()
 *
 * if (hasPermission('bichat.access')) {
 *   // User has bichat access
 * }
 *
 * if (hasAnyPermission('finance.view', 'finance.edit')) {
 *   // User has at least one of these permissions
 * }
 */
export function usePermissions(): PermissionsHook {
  const { user } = useAppletContext()

  const hasPermission = (permission: string): boolean => {
    return user.permissions.includes(permission)
  }

  const hasAnyPermission = (...permissions: string[]): boolean => {
    return permissions.some(p => user.permissions.includes(p))
  }

  return {
    hasPermission,
    hasAnyPermission,
    permissions: user.permissions
  }
}
