import { useCallback, useEffect, useMemo, useRef, type FocusEventHandler, type MouseEventHandler, type PointerEventHandler } from 'react'
import type { Action, Frame, Panel } from '../contract'
import { recordForRow, resolveActionURL, variablesFromLocation } from '../explore/actions'
import { navigateTo } from '../runtime/navigate'
import { useDrawer } from '../runtime'

/** How long a pointer/focus must dwell before a drawer document is prefetched. */
const prefetchIntentDelayMs = 65

export interface PrefetchHandlers {
  onPointerEnter: PointerEventHandler
  onPointerLeave: PointerEventHandler
  onFocus: FocusEventHandler
  onBlur: FocusEventHandler
}

/**
 * Hover/focus prefetch for a drawer-opening target. After a short intent delay
 * (cancelled if the pointer leaves first) the drawer document is warmed into
 * the shared cache, so a subsequent click opens against a document in hand.
 * Returns undefined when the action does not open a drawer or has no URL.
 */
export function usePrefetch(url: string | undefined, action: Action | undefined): PrefetchHandlers | undefined {
  const drawer = useDrawer()
  const enabled = action?.kind === 'open_drawer' && drawer.depth === 0 && Boolean(url)
  const timer = useRef<ReturnType<typeof setTimeout>>()
  const cancel = useCallback(() => {
    if (timer.current !== undefined) {
      clearTimeout(timer.current)
      timer.current = undefined
    }
  }, [])
  useEffect(() => cancel, [cancel])
  return useMemo(() => {
    if (!enabled || !url) return undefined
    const schedule = () => {
      cancel()
      timer.current = setTimeout(() => drawer.prefetch(url), prefetchIntentDelayMs)
    }
    return { onPointerEnter: schedule, onFocus: schedule, onPointerLeave: cancel, onBlur: cancel }
  }, [cancel, drawer, enabled, url])
}

/**
 * Panel-level navigation.
 *
 * The legacy renderer made a whole stat / segment-bar card a link, and made a
 * chart's data points navigate, whenever the panel spec carried a navigate
 * action. The wire keeps that action in `panel.actions` as kind `navigate`;
 * without this layer those panels render as inert cards, which is how the
 * «Ключевые коэффициенты» strip lost its drill-down.
 */

export function panelNavigateAction(panel: Panel): Action | undefined {
  // One panel, one click behaviour. A panel that owns a drill tree explores on
  // click and keeps its links inside the drill overlay; only a panel without a
  // tree turns its navigate action into a click target. Without this rule a
  // segment click both opened the overlay and left the page.
  if (panel.drillRoot) return undefined
  return panel.actions.find((action) => action.kind === 'navigate' || action.kind === 'open_drawer')
}

/**
 * True when the action's URL depends on the row it is resolved against — the
 * same rule the legacy renderer used to decide between one card-wide link and
 * one link per segment.
 */
export function isRowScoped(action: Action): boolean {
  if (action.urlSource) return action.urlSource.kind === 'field'
  return action.params.some((param) => param.source.kind === 'field')
    || Object.values(action.payload).some((source) => source.kind === 'field')
}

export interface PanelNavigation {
  action?: Action
  rowScoped: boolean
  /** URL for one row of the panel's frame, or undefined when it cannot resolve. */
  urlForRow: (frame: Frame | undefined, row: Array<unknown> | undefined) => string | undefined
  /** URL for the panel as a whole: the first row's, when the action is not row-scoped. */
  cardURL: (frame: Frame | undefined) => string | undefined
  onClick: (url: string | undefined) => MouseEventHandler<HTMLAnchorElement> | undefined
  activate: (url: string | undefined, opener?: HTMLElement) => void
}

export function useActionActivation(action: Action | undefined) {
  const drawer = useDrawer()
  const opensDrawer = action?.kind === 'open_drawer'
  const available = Boolean(action) && (!opensDrawer || drawer.depth === 0)
  const activate = useCallback((url: string | undefined, opener?: HTMLElement) => {
    if (!url || !available) return
    if (opensDrawer) drawer.open(url, opener)
    else navigateTo(url)
  }, [available, drawer, opensDrawer])
  const onClick = useCallback((url: string | undefined): MouseEventHandler<HTMLAnchorElement> | undefined => {
    if (!url || !opensDrawer || !available) return undefined
    return (event) => {
      event.preventDefault()
      drawer.open(url, event.currentTarget)
    }
  }, [available, drawer, opensDrawer])
  return { activate, available, onClick }
}

export function usePanelNavigation(panel: Panel): PanelNavigation {
  const candidate = useMemo(() => panelNavigateAction(panel), [panel])
  const activation = useActionActivation(candidate)
  const action = activation.available ? candidate : undefined

  const urlForRow = useCallback((frame: Frame | undefined, row: Array<unknown> | undefined) => {
    if (!action) return undefined
    const location = new URL(globalThis.location.href)
    return resolveActionURL(action, {
      fields: frame && row ? recordForRow(frame, row) : {},
      variables: variablesFromLocation(location),
      location,
    })
  }, [action])

  const cardURL = useCallback((frame: Frame | undefined) => {
    if (!action) return undefined
    // A row-scoped action belongs to the individual segments, not the card:
    // turning the whole card into the first segment's link would send every
    // click to the wrong place.
    if (isRowScoped(action) && (frame?.rows.length ?? 0) > 1) return undefined
    return urlForRow(frame, frame?.rows[0])
  }, [action, urlForRow])

  return useMemo(() => ({
    action,
    rowScoped: action ? isRowScoped(action) : false,
    urlForRow,
    cardURL,
    onClick: activation.onClick,
    activate: activation.activate,
  }), [action, activation.activate, activation.onClick, cardURL, urlForRow])
}
