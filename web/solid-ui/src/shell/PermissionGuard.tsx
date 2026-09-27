import { createMemo, type JSX } from 'solid-js'

export type PermissionPredicate<P = string> = (permission: P) => boolean
export interface PermissionGuardProps<P = string> { permissions: readonly P[]; can: PermissionPredicate<P>; mode?: 'all' | 'any'; fallback?: JSX.Element; children: JSX.Element }

export function PermissionGuard<P = string>(props: PermissionGuardProps<P>) {
  const allowed = createMemo(() => (props.mode ?? 'all') === 'all' ? props.permissions.every(props.can) : props.permissions.some(props.can))
  return <>{allowed() ? props.children : props.fallback}</>
}
