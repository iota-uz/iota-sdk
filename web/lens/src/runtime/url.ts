import type { NodePath } from '../contract'
import type { NavigationView } from './navigation'

const pathParameter = 'path'
const perspectiveParameter = 'perspective'
const drawerParameter = 'drawer'
const drawerPathParameter = 'drawerPath'
const drawerPerspectiveParameter = 'drawerPerspective'
const drawerPanelParameter = 'drawerPanel'
const hiddenSeriesParameter = 'lensHidden'

function hiddenSeriesValue(panelId: string, key: string): string {
  return JSON.stringify([panelId, key])
}

function parseHiddenSeriesValue(value: string): [string, string] | undefined {
  try {
    const parsed: unknown = JSON.parse(value)
    if (!Array.isArray(parsed) || parsed.length !== 2 || parsed.some((item) => typeof item !== 'string')) return undefined
    return parsed as [string, string]
  } catch {
    return undefined
  }
}

export function hiddenSeriesFromURL(url: URL, panelId: string): ReadonlySet<string> {
  const hidden = new Set<string>()
  for (const value of url.searchParams.getAll(hiddenSeriesParameter)) {
    const parsed = parseHiddenSeriesValue(value)
    if (parsed?.[0] === panelId) hidden.add(parsed[1])
  }
  return hidden
}

export function hiddenSeriesToURL(current: URL, panelId: string, hidden: ReadonlySet<string>): URL {
  const next = new URL(current)
  const retained = next.searchParams.getAll(hiddenSeriesParameter).filter((value) => parseHiddenSeriesValue(value)?.[0] !== panelId)
  next.searchParams.delete(hiddenSeriesParameter)
  for (const value of retained) next.searchParams.append(hiddenSeriesParameter, value)
  for (const key of [...hidden].sort()) next.searchParams.append(hiddenSeriesParameter, hiddenSeriesValue(panelId, key))
  return next
}

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

/**
 * A drawer document may carry its desired initial Lens view in its own
 * `path` / `perspective` query parameters. Keeping that intent on the source
 * URL makes a normal open-drawer action deep-linkable without teaching the
 * action contract about a particular explorer or business domain.
 */
export function drawerNavigationFromSource(src: string, current: URL): Omit<NonNullable<NavigationView['drawer']>, 'src'> {
  const view = navigationFromURL(new URL(src, current))
  return {
    path: [...view.path],
    perspectiveId: view.perspectiveId,
  }
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
