import { createEffect, createMemo, onCleanup, untrack, type Accessor } from 'solid-js'
import type { Action, Frame, Panel } from '../contract'
import { drawerKeyFromActionURL, recordForRow, resolveActionURL, variablesFromLocation } from '../explore/actions'
import { navigateTo } from '../runtime/navigate'
import { useDrawer } from '../runtime'
import { filterActionURL, useFilters } from '../runtime'

/** How long a pointer/focus must dwell before a drawer document is prefetched. */
const prefetchIntentDelayMs = 65

/** Normalizes a reactive-or-static value into an accessor. */
function of<T>(value: T | Accessor<T>): Accessor<T> {
  return typeof value === 'function' ? value as Accessor<T> : () => value
}

export interface PrefetchHandlers {
  onPointerEnter: (event: PointerEvent) => void
  onPointerLeave: (event: PointerEvent) => void
  onFocus: (event: FocusEvent) => void
  onBlur: (event: FocusEvent) => void
}

/**
 * Bounded idle plus hover/focus prefetch for a stat drawer target. The idle
 * registration supplies the automatic first-level warm-up; concrete intent
 * promotes the same target after a short cancellable dwell.
 *
 * The handlers are stable and guard on the reactive state at call time, so the
 * descriptor stays valid across frames loading and drawers opening.
 */
export function usePrefetch(
  url: string | Accessor<string | undefined>,
  action: Action | Accessor<Action | undefined>,
  prefetchIdle?: (urls: ReadonlyArray<string>) => () => void,
): PrefetchHandlers {
  const drawer = useDrawer()
  const resolvedURL = of(url)
  const resolvedAction = of(action)
  let timer: ReturnType<typeof setTimeout> | undefined
  let cancelActive: (() => void) | undefined
  const cancel = () => {
    if (timer !== undefined) {
      clearTimeout(timer)
      timer = undefined
    }
    cancelActive?.()
    cancelActive = undefined
  }
  onCleanup(cancel)
  createEffect(() => {
    const current = resolvedURL()
    return prefetchIdle?.(current ? [current] : [])
  })
  const eligible = () => untrack(resolvedAction)?.kind === 'open_drawer'
    && drawer.depth === 0
    && Boolean(untrack(resolvedURL))
  const schedule = () => {
    if (!eligible()) return
    cancel()
    timer = setTimeout(() => {
      timer = undefined
      const target = untrack(resolvedURL)
      const key = drawerKeyFromActionURL(target ?? '')
      cancelActive = key && target ? drawer.prefetchKey(key) : drawer.prefetch(target ?? '')
    }, prefetchIntentDelayMs)
  }
  return { onPointerEnter: schedule, onFocus: schedule, onPointerLeave: cancel, onBlur: cancel }
}

/**
 * Panel-level navigation.
 *
 * A panel-level navigate action makes a whole stat / segment-bar card a link
 * and makes chart data points navigate. The wire keeps that action in
 * `panel.actions` as kind `navigate`.
 */

export function panelNavigateAction(panel: Panel): Action | undefined {
  // One panel, one click behaviour. A panel that owns a drill tree explores on
  // click and keeps its links inside the drill overlay; only a panel without a
  // tree turns its navigate action into a click target. Without this rule a
  // segment click both opened the overlay and left the page.
  if (panel.drillRoot) return undefined
  return panel.actions.find((action) => action.kind === 'navigate' || action.kind === 'open_drawer' || action.kind === 'cross_filter' || action.kind === 'cube_drill')
}

/**
 * True when the action's URL depends on the row it is resolved against — the
 * rule used to decide between one card-wide link and one link per segment.
 */
export function isRowScoped(action: Action): boolean {
  if (action.urlSource) return action.urlSource.kind === 'field'
  return action.params.some((param) => param.source.kind === 'field')
    || Object.values(action.payload).some((source) => source.kind === 'field')
}

export interface PanelNavigation {
  action: Action | undefined
  rowScoped: boolean
  /** URL for one row of the panel's frame, or undefined when it cannot resolve. */
  urlForRow: (frame: Frame | undefined, row: Array<unknown> | undefined) => string | undefined
  /** URL for the panel as a whole: the first row's, when the action is not row-scoped. */
  cardURL: (frame: Frame | undefined) => string | undefined
  onClick: (url: string | undefined) => ((event: MouseEvent) => void) | undefined
  activate: (url: string | undefined, opener?: HTMLElement, options?: { newTab?: boolean }) => void
  /** Promote a concrete drawer target on pointer/focus intent. */
  prefetch: (url: string | undefined) => () => void
  /** Register low-cardinality targets in the runtime's one-slot idle queue. */
  prefetchIdle: (urls: ReadonlyArray<string>) => () => void
}

/**
 * An element-level action resolved to an interactive descriptor.
 *
 * Unlike {@link usePanelNavigation}, which owns the single panel-wide navigate
 * action, metric panels attach an action to each stage / row / end. The resolver
 * is a stable function (one drawer subscription) so it can be applied across a
 * list of elements without a hook per element.
 */
export interface ElementActionTarget {
  href: string
  onClick?: (event: MouseEvent) => void
  opensDrawer: boolean
}

/**
 * Returns a resolver that turns an element's `Action` into an anchor descriptor,
 * or `undefined` when the action carries no navigable URL (e.g. an emit-only
 * action) or a drawer action is unavailable at the current depth. Element
 * actions resolve literal/variable params only in v1; `fields` is accepted for
 * forward compatibility with per-element field resolution.
 */
export function useElementActionResolver(): (action: Action | undefined, fields?: Readonly<Record<string, unknown>>) => ElementActionTarget | undefined {
  const drawer = useDrawer()
  return (action, fields = {}) => {
    if (!action) return undefined
    const opensDrawer = action.kind === 'open_drawer'
    if (opensDrawer && !(drawer.depth === 0 || drawer.canOpen)) return undefined
    const location = new URL(globalThis.location.href)
    const href = resolveActionURL(action, {
      fields,
      variables: variablesFromLocation(location),
      location,
    })
    if (!href) return undefined
    const onClick: ((event: MouseEvent) => void) | undefined = opensDrawer
      ? (event) => {
        event.preventDefault()
        const key = drawerKeyFromActionURL(href)
        if (key) drawer.openKey(key, event.currentTarget as HTMLElement)
        else drawer.open(href, event.currentTarget as HTMLElement)
      }
      : undefined
    return { href, onClick, opensDrawer }
  }
}

export function useActionActivation(action: Action | undefined | Accessor<Action | undefined>) {
  const drawer = useDrawer()
  const filters = useFilters()
  const resolvedAction = of(action)
  const opensDrawer = () => resolvedAction()?.kind === 'open_drawer'
  const filtersData = () => resolvedAction()?.kind === 'cross_filter' || resolvedAction()?.kind === 'cube_drill'
  const available = () => Boolean(resolvedAction()) && (!opensDrawer() || drawer.depth === 0 || drawer.canOpen === true)
  let intentTimer: ReturnType<typeof setTimeout> | undefined
  let cancelIntent: (() => void) | undefined
  const cancelPrefetch = () => {
    if (intentTimer !== undefined) clearTimeout(intentTimer)
    intentTimer = undefined
    cancelIntent?.()
    cancelIntent = undefined
  }
  onCleanup(cancelPrefetch)
  const prefetch = (url: string | undefined) => {
    cancelPrefetch()
    if (!url || !untrack(opensDrawer) || !untrack(available)) return cancelPrefetch
    intentTimer = setTimeout(() => {
      intentTimer = undefined
      const key = drawerKeyFromActionURL(url)
      cancelIntent = key ? drawer.prefetchKey(key) : drawer.prefetch(url)
    }, prefetchIntentDelayMs)
    return cancelPrefetch
  }
  const prefetchIdle = (urls: ReadonlyArray<string>) => {
    if (!untrack(opensDrawer) || !untrack(available)) return () => undefined
    const cancels = urls.map((url) => {
      const key = drawerKeyFromActionURL(url)
      return key ? drawer.prefetchIdleKey(key) : drawer.prefetchIdle(url)
    })
    return () => cancels.forEach((cancel) => cancel())
  }
  const activate = (url: string | undefined, opener?: HTMLElement, options?: { newTab?: boolean }) => {
    if (!url || !untrack(available)) return
    if (untrack(opensDrawer)) {
      const key = drawerKeyFromActionURL(url)
      if (key) drawer.openKey(key, opener)
      else drawer.open(url, opener)
    }
    else if (untrack(filtersData)) filters.applyURL(url, options)
    else navigateTo(url)
  }
  const onClick = (url: string | undefined): ((event: MouseEvent) => void) | undefined => {
    if (!url || !untrack(opensDrawer) || !untrack(available)) return undefined
    return (event) => {
      event.preventDefault()
      const key = drawerKeyFromActionURL(url)
      if (key) drawer.openKey(key, event.currentTarget as HTMLElement)
      else drawer.open(url, event.currentTarget as HTMLElement)
    }
  }
  return { activate, available, onClick, prefetch, prefetchIdle }
}

export function usePanelNavigation(panel: Panel | Accessor<Panel>): PanelNavigation {
  const resolvedPanel = of(panel)
  const candidate = createMemo(() => panelNavigateAction(resolvedPanel()))
  const activation = useActionActivation(candidate)
  const action = createMemo<Action | undefined>(() => activation.available() ? candidate() : undefined)

  const urlForRow = (frame: Frame | undefined, row: Array<unknown> | undefined) => {
    const current = untrack(action)
    if (!current) return undefined
    const location = new URL(globalThis.location.href)
    if ((current.kind === 'cross_filter' || current.kind === 'cube_drill') && current.filter) {
      return filterActionURL(current, frame && row ? recordForRow(frame, row) : {}, location)?.href
    }
    return resolveActionURL(current, {
      fields: frame && row ? recordForRow(frame, row) : {},
      variables: variablesFromLocation(location),
      location,
    })
  }

  const cardURL = (frame: Frame | undefined) => {
    const current = untrack(action)
    if (!current) return undefined
    // A row-scoped action belongs to the individual segments, not the card:
    // turning the whole card into the first segment's link would send every
    // click to the wrong place.
    if (isRowScoped(current) && (frame?.rows.length ?? 0) > 1) return undefined
    return urlForRow(frame, frame?.rows[0])
  }

  return {
    get action() { return action() },
    get rowScoped() { return action() ? isRowScoped(action()!) : false },
    urlForRow,
    cardURL,
    get onClick() { return activation.onClick },
    get activate() { return activation.activate },
    get prefetch() { return activation.prefetch },
    get prefetchIdle() { return activation.prefetchIdle },
  }
}
