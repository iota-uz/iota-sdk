import type { NodePath } from '../contract'
import type { NavigationView } from './navigation'

const pathParameter = 'path'
const perspectiveParameter = 'perspective'

export function navigationFromURL(url: URL): NavigationView {
  const path = url.searchParams.getAll(pathParameter).filter((key) => key.length > 0) as NodePath
  const perspectiveId = url.searchParams.get(perspectiveParameter)?.trim() || undefined
  return { path, perspectiveId }
}

export function navigationToURL(view: NavigationView, current: URL): URL {
  const next = new URL(current)
  next.searchParams.delete(pathParameter)
  next.searchParams.delete(perspectiveParameter)
  for (const key of view.path) next.searchParams.append(pathParameter, key)
  if (view.perspectiveId) next.searchParams.set(perspectiveParameter, view.perspectiveId)
  return next
}

export function sameNavigationURL(left: URL, right: URL): boolean {
  return left.pathname === right.pathname && left.search === right.search && left.hash === right.hash
}
