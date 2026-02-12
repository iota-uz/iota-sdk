export function normalizePathname(pathname: string): string {
  if (!pathname) return '/'
  let p = pathname.trim()
  if (!p.startsWith('/')) p = `/${p}`
  // collapse repeated slashes
  p = p.replace(/\/{2,}/g, '/')
  // strip trailing slash (except root)
  if (p.length > 1 && p.endsWith('/')) {
    p = p.slice(0, -1)
  }
  return p
}

export function isInScopePath(pathname: string): boolean {
  const p = normalizePathname(pathname)

  // Basic traversal guard
  if (p.includes('..')) return false
  if (p.includes('\\')) return false

  if (p === '/') return true
  if (p === '/archived') return true
  if (/^\/session\/[^/]+$/.test(p)) return true

  return false
}

