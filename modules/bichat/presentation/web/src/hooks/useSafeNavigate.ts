import { useCallback } from 'react'
import { useLocation, useNavigate, type NavigateOptions, type To } from 'react-router-dom'
import { isInScopePath, normalizePathname } from '../utils/navigation'

function isStringTo(to: To): to is string {
  return typeof to === 'string'
}

function resolveTargetPathname(currentPathname: string, to: string): string | null {
  const raw = to.trim()
  if (!raw) return null

  // Allow staying on current route
  if (raw === '.') return normalizePathname(currentPathname)

  // Disallow explicit traversal
  if (raw === '..' || raw.startsWith('../') || raw.includes('/../')) return null

  // We only support absolute paths for safety.
  if (!raw.startsWith('/')) return null

  return normalizePathname(raw)
}

export function useSafeNavigate() {
  const navigate = useNavigate()
  const location = useLocation()

  return useCallback(
    (to: To | number, options?: NavigateOptions) => {
      if (typeof to === 'number') {
        navigate(to)
        return
      }

      if (!isStringTo(to)) {
        // Non-string targets (like { pathname, search }) are intentionally disallowed here.
        navigate('/', { replace: true })
        return
      }

      const target = resolveTargetPathname(location.pathname || '/', to)
      if (!target || !isInScopePath(target)) {
        navigate('/', { replace: true })
        return
      }

      navigate(target, options)
    },
    [location.pathname, navigate]
  )
}

