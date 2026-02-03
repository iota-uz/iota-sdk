import { useAppletContext } from '../context/AppletContext'
import type { RouteContext } from '../types'

/**
 * useRoute provides access to the current route context.
 * Route context is initialized from the backend and includes path, params, and query.
 *
 * Usage:
 * const { path, params, query } = useRoute()
 *
 * // Example values:
 * // path: "/sessions/123"
 * // params: { id: "123" }
 * // query: { tab: "history" }
 */
export function useRoute(): RouteContext {
  const { route } = useAppletContext()
  return route
}
