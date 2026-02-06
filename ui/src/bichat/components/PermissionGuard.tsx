/**
 * PermissionGuard Component
 * Conditionally renders children based on permission checks
 *
 * @example
 * // Single permission
 * <PermissionGuard permissions={['chat.read']} hasPermission={hasPermission}>
 *   <ChatList />
 * </PermissionGuard>
 *
 * // Multiple permissions (AND logic - all required)
 * <PermissionGuard permissions={['chat.read', 'chat.write']} mode="all">
 *   <AdminPanel />
 * </PermissionGuard>
 *
 * // Multiple permissions (OR logic - any required)
 * <PermissionGuard permissions={['chat.read', 'chat.readOwn']} mode="any">
 *   <ChatList />
 * </PermissionGuard>
 *
 * // With custom fallback
 * <PermissionGuard
 *   permissions={['chat.write']}
 *   fallback={<div>You don't have permission</div>}
 * >
 *   <CreateChatButton />
 * </PermissionGuard>
 */

import type { ReactNode } from 'react'

export interface PermissionGuardProps {
  /** Permission names to check */
  permissions: string[]
  /** Check mode: 'all' requires all permissions (AND), 'any' requires at least one (OR) */
  mode?: 'all' | 'any'
  /** Function to check if user has a specific permission */
  hasPermission: (permission: string) => boolean
  /** Fallback to render when permissions are not satisfied */
  fallback?: ReactNode
  /** Children to render when permissions are satisfied */
  children: ReactNode
}

/**
 * Permission guard component.
 * Conditionally renders children based on permission checks.
 */
export function PermissionGuard({
  permissions,
  mode = 'all',
  hasPermission,
  fallback = null,
  children,
}: PermissionGuardProps) {
  // Handle empty permissions array (no permissions required, always render)
  if (permissions.length === 0) {
    return <>{children}</>
  }

  // Check permissions based on mode
  const permitted =
    mode === 'all'
      ? permissions.every((p) => hasPermission(p))
      : permissions.some((p) => hasPermission(p))

  return permitted ? <>{children}</> : <>{fallback}</>
}

export default PermissionGuard
