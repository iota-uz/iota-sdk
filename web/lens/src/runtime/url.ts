import type { NodePath } from '../contract'
import type { NavigationView } from './navigation'

const pathParameter = 'path'
const perspectiveParameter = 'perspective'
const drawerParameter = 'drawer'
const drawerPathParameter = 'drawerPath'
const drawerPerspectiveParameter = 'drawerPerspective'
const drawerPanelParameter = 'drawerPanel'
const rendererStateParameter = 'lens-state'
const legacyHiddenSeriesParameter = 'lensHidden'
const legacyTemporalStateParameter = 'lensTemporal'

export function siteRelativeURL(value: string, current: URL): string | undefined {
  try {
    const target = new URL(value, current)
    return target.origin === current.origin ? `${target.pathname}${target.search}${target.hash}` : undefined
  } catch { return undefined }
}

interface RendererURLState {
  v: 1
  h?: Record<string, string[]>
  t?: Record<string, [boolean, number?]>
}

function rendererState(url: URL): RendererURLState {
  try {
    const parsed = JSON.parse(url.searchParams.get(rendererStateParameter) ?? '') as Partial<RendererURLState>
    if (parsed.v !== 1) return { v: 1 }
    return { v: 1, ...(parsed.h && typeof parsed.h === 'object' ? { h: parsed.h } : {}), ...(parsed.t && typeof parsed.t === 'object' ? { t: parsed.t } : {}) }
  } catch { return { v: 1 } }
}

function writeRendererState(url: URL, state: RendererURLState): URL {
  const next = new URL(url)
  const empty = Object.keys(state.h ?? {}).length === 0 && Object.keys(state.t ?? {}).length === 0
  if (empty) next.searchParams.delete(rendererStateParameter)
  else next.searchParams.set(rendererStateParameter, JSON.stringify(state))
  next.searchParams.delete(legacyHiddenSeriesParameter)
  next.searchParams.delete(legacyTemporalStateParameter)
  return next
}

/** A server-provided reset link cannot know about renderer-owned state. */
export function clearRendererStateFromURL(target: string, current: URL): string {
  const next = new URL(target, current)
  next.searchParams.delete(rendererStateParameter)
  next.searchParams.delete(legacyHiddenSeriesParameter)
  next.searchParams.delete(legacyTemporalStateParameter)
  return next.origin === current.origin ? `${next.pathname}${next.search}${next.hash}` : next.href
}

export function hiddenSeriesFromURL(url: URL, panelId: string): ReadonlySet<string> {
  const keys = rendererState(url).h?.[panelId]
  return new Set(Array.isArray(keys) ? keys.filter((key): key is string => typeof key === 'string') : [])
}

export function hiddenSeriesToURL(current: URL, panelId: string, hidden: ReadonlySet<string>): URL {
  const state = rendererState(current)
  const h = { ...state.h }
  if (hidden.size > 0) h[panelId] = [...hidden].sort()
  else delete h[panelId]
  return writeRendererState(current, { ...state, h })
}

export interface TemporalURLState {
  regression: boolean
  movingAverage?: number
}

export function temporalStateFromURL(url: URL, panelId: string): TemporalURLState {
  const parsed = rendererState(url).t?.[panelId]
  if (Array.isArray(parsed) && typeof parsed[0] === 'boolean') return { regression: parsed[0], ...(typeof parsed[1] === 'number' ? { movingAverage: parsed[1] } : {}) }
  return { regression: false }
}

export function temporalStateToURL(current: URL, panelId: string, state: TemporalURLState): URL {
  const compact = rendererState(current)
  const t = { ...compact.t }
  if (state.regression || state.movingAverage !== undefined) t[panelId] = [state.regression, state.movingAverage]
  else delete t[panelId]
  return writeRendererState(current, { ...compact, t })
}

export function navigationFromURL(url: URL): NavigationView {
  const path = url.searchParams.getAll(pathParameter).filter((key) => key.length > 0) as NodePath
  const perspectiveId = url.searchParams.get(perspectiveParameter)?.trim() || undefined
  const rawDrawerSrc = url.searchParams.get(drawerParameter)?.trim()
  let drawerSrc: string | undefined
  if (rawDrawerSrc) {
    try {
      drawerSrc = siteRelativeURL(rawDrawerSrc, url)
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
    const drawerSrc = siteRelativeURL(view.drawer.src, current)
    if (drawerSrc) next.searchParams.set(drawerParameter, drawerSrc)
    for (const key of view.drawer.path) next.searchParams.append(drawerPathParameter, key)
    if (view.drawer.perspectiveId) next.searchParams.set(drawerPerspectiveParameter, view.drawer.perspectiveId)
    if (view.drawer.panelId) next.searchParams.set(drawerPanelParameter, view.drawer.panelId)
  }
  return next
}

export function sameNavigationURL(left: URL, right: URL): boolean {
  return left.pathname === right.pathname && left.search === right.search && left.hash === right.hash
}
