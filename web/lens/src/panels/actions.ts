import { useCallback, useMemo } from 'react'
import type { Action, Frame, Panel } from '../contract'
import { recordForRow, resolveActionURL, variablesFromLocation } from '../explore/actions'

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
  return panel.actions.find((action) => action.kind === 'navigate')
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
}

export function usePanelNavigation(panel: Panel): PanelNavigation {
  const action = useMemo(() => panelNavigateAction(panel), [panel])

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
  }), [action, cardURL, urlForRow])
}
