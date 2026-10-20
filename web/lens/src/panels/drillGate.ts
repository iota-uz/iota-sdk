import { createContext, useContext } from 'solid-js'
import type { Action, Panel } from '../contract'

/**
 * Whether panels below may continue the analysis (drill, open records or
 * details). A report placement whose source or context does not allow drill
 * provides false, so its members render as terminal.
 */
export const DrillGateContext = createContext<() => boolean>(() => true)

export function useDrillAllowed(): () => boolean {
  return useContext(DrillGateContext)
}

/** Cross-filtering narrows the board in place; every other action continues the analysis elsewhere. */
function keep(action: Action | undefined): Action | undefined {
  return action?.kind === 'cross_filter' ? action : undefined
}

/**
 * The panel as a terminal result: no drill tree and no action that continues
 * the analysis, wherever the panel kind carries one, so no kind can render a
 * click target, pointer affordance or keyboard activation for it.
 */
export function withoutDrill(panel: Panel): Panel {
  const terminal = { ...panel, terminal: true, actions: panel.actions.filter((action) => keep(action)) } as Panel & Record<string, unknown>
  delete terminal.drillRoot
  if (panel.columns) terminal.columns = panel.columns.map(({ action, ...column }) => ({ ...column, ...(keep(action) ? { action } : {}) }))
  if (panel.metricFlow) {
    terminal.metricFlow = { ...panel.metricFlow, stages: panel.metricFlow.stages.map(({ action, ...stage }) => ({ ...stage, ...(keep(action) ? { action } : {}) })) }
  }
  if (panel.metricHierarchy) {
    terminal.metricHierarchy = { ...panel.metricHierarchy, rows: panel.metricHierarchy.rows.map(({ action, ...row }) => ({ ...row, ...(keep(action) ? { action } : {}) })) }
  }
  if (panel.metricRelationship) {
    const end = ({ action, ...rest }: NonNullable<Panel['metricRelationship']>['source']) => ({ ...rest, ...(keep(action) ? { action } : {}) })
    terminal.metricRelationship = { ...panel.metricRelationship, source: end(panel.metricRelationship.source), target: end(panel.metricRelationship.target) }
  }
  return terminal
}
