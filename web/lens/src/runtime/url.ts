import type { NodePath } from '../contract'
import type { NavigationView } from './navigation'

const pathParameter = 'path'
const perspectiveParameter = 'perspective'
const drawerParameter = 'drawer'
const drawerPathParameter = 'drawerPath'
const drawerPerspectiveParameter = 'drawerPerspective'
const drawerPanelParameter = 'drawerPanel'

export function navigationFromURL(url: URL): NavigationView {
  const path = url.searchParams.getAll(pathParameter).filter((key) => key.length > 0) as NodePath
  const perspectiveId = url.searchParams.get(perspectiveParameter)?.trim() || undefined
  const rawDrawerSrc = url.searchParams.get(drawerParameter)?.trim()
  let drawerSrc: string | undefined
  if (rawDrawerSrc) {
    try {
      if (new URL(rawDrawerSrc, url).origin === url.origin) drawerSrc = rawDrawerSrc
    } catch {
      drawerSrc = undefined
    }
  }
  const drawer = drawerSrc ? {
    src: drawerSrc,
    path: url.searchParams.getAll(drawerPathParameter).filter((key) => key.length > 0) as NodePath,
    perspectiveId: url.searchParams.get(drawerPerspectiveParameter)?.trim() || undefined,
    panelId: url.searchParams.get(drawerPanelParameter)?.trim() || undefined,
  } : undefined
  return { path, perspectiveId, ...(drawer ? { drawer } : {}) }
}

export function navigationToURL(view: NavigationView, current: URL): URL {
  const next = new URL(current)
  next.searchParams.delete(pathParameter)
  next.searchParams.delete(perspectiveParameter)
  next.searchParams.delete(drawerParameter)
  next.searchParams.delete(drawerPathParameter)
  next.searchParams.delete(drawerPerspectiveParameter)
  next.searchParams.delete(drawerPanelParameter)
  for (const key of view.path) next.searchParams.append(pathParameter, key)
  if (view.perspectiveId) next.searchParams.set(perspectiveParameter, view.perspectiveId)
  if (view.drawer) {
    next.searchParams.set(drawerParameter, view.drawer.src)
    for (const key of view.drawer.path) next.searchParams.append(drawerPathParameter, key)
    if (view.drawer.perspectiveId) next.searchParams.set(drawerPerspectiveParameter, view.drawer.perspectiveId)
    if (view.drawer.panelId) next.searchParams.set(drawerPanelParameter, view.drawer.panelId)
  }
  return next
}

export function sameNavigationURL(left: URL, right: URL): boolean {
  return left.pathname === right.pathname && left.search === right.search && left.hash === right.hash
}
