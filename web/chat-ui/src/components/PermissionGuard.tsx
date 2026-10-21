import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
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
export interface PermissionGuardProps {
    /** Permission names to check */
    permissions: string[];
    /** Check mode: 'all' requires all permissions (AND), 'any' requires at least one (OR) */
    mode?: 'all' | 'any';
    /** Function to check if user has a specific permission */
    hasPermission: (permission: string) => boolean;
    /** Fallback to render when permissions are not satisfied */
    fallback?: JSX.Element;
    /** Children to render when permissions are satisfied */
    children: JSX.Element;
}
/**
 * Permission guard component.
 * Conditionally renders children based on permission checks.
 */
export function PermissionGuard(solidProps1Input: PermissionGuardProps) {
    const solidProps1 = mergeProps({ mode: 'all', fallback: null } as const, solidProps1Input);
    return <>{createMemo(() => {
            // Handle empty permissions array (no permissions required, always render)
            if (solidProps1.permissions.length === 0) {
                return <>{solidProps1.children}</>;
            }
            // Check permissions based on mode
            const permitted = createMemo(() => solidProps1.mode === 'all'
                ? solidProps1.permissions.every((p) => solidProps1.hasPermission(p))
                : solidProps1.permissions.some((p) => solidProps1.hasPermission(p)));
            return permitted() ? <>{solidProps1.children}</> : <>{solidProps1.fallback}</>;
        })}</>;
}
export default PermissionGuard;
