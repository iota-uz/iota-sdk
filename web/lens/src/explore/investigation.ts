import type {
  ContextConstraint, ContextFilters, ContextOrigin, ContextRange, ContextSource, DashboardDocument, EffectiveContext,
  LayoutGroup, Level, NodePath, Panel,
} from '../contract'
import type { Capabilities } from '../reports/contract/types'
import type { ReportDestinationChoice, ReportDestinationHost } from '../reports/picker'
import { reportPlacementActions } from '../reports/version'
import { levelForPath } from '../runtime/drill'
import type { NavigationView } from '../runtime/navigation'

/**
 * Saving an exploration as a report.
 *
 * A drill position inside a report placement is a slice of that report's
 * source: the placement's effective context (period and restrictions, each
 * with its origin) plus one constraint per drill step, read from the level's
 * declared dimension and the selected node's value. The slice is saveable
 * only when the source can save under that context and every drill dimension
 * is one the source accepts for saving. Nothing here stores anything: the
 * host persists the proposal, and the parent dashboard is never touched.
 */

/** A drill step as a constraint: the level's dimension and the selected value. */
export interface InvestigationStep {
  dimension: string
  value: string
  /** The selected node's label. */
  label: string
  /** The label of the level the selection was made on. */
  levelLabel: string
}

/** One constraint of the slice with where it came from. */
export interface InvestigationConstraint {
  dimension: string
  values: Array<string>
  /** Display labels of the values, when known. */
  labels: Array<string>
  /** A board layer, or `{ scope: 'drill' }` for a step taken in this exploration. */
  origin: ContextOrigin
}

export interface InvestigationSlice {
  placementId: string
  /** The report (placement) title the exploration started from. */
  reportTitle: string
  measure: { panelId: string; title: string }
  source: ContextSource
  calendar: EffectiveContext['calendar']
  period: ContextRange
  constraints: Array<InvestigationConstraint>
  steps: Array<InvestigationStep>
  navigation: NavigationView
  /** The opened placement's effective context, unchanged. */
  context: EffectiveContext
}

export type InvestigationUnsupportedReason =
  /** The panel does not belong to a report placement with a source. */
  | 'not-a-report'
  /** The source does not declare saving, or the host did not say it can. */
  | 'source-cannot-save'
  /** The effective context conflicts or carries filters the source cannot save. */
  | 'context-blocked'
  /** A drill level does not declare which dimension its selection constrains. */
  | 'level-undeclared'
  /** A drill dimension is not one the source can save. */
  | 'dimension-unsupported'
  /** A drill selection contradicts a restriction already applied. */
  | 'drill-conflict'

export type InvestigationSaveability =
  | { saveable: true; slice: InvestigationSlice }
  | { saveable: false; reason: InvestigationUnsupportedReason; detail?: string }

/** The definition the host stores: a source, a view and filters; never a snapshot. */
export interface InvestigationProposal {
  title: string
  description?: string
  source: { id: string; version: number }
  view: string
  filters: ContextFilters
  /** Where it was explored, for the host's own audit; the parent is not modified. */
  origin: {
    dashboardId: string
    placementId: string
    panelId: string
    path: NodePath
    perspectiveId?: string
    trail: Array<string>
  }
}

export type InvestigationSaveIntent = 'draft' | 'publish'

/**
 * One save attempt. The id stays the same across retries of the same
 * proposal and intent, so the host can make the save idempotent and a retry
 * after an uncertain failure never creates a second report.
 */
export interface InvestigationSaveRequest {
  id: string
  intent: InvestigationSaveIntent
  proposal: InvestigationProposal
}

/** The report the host created. */
export interface SavedInvestigation {
  reportId: string
  title: string
  /** Where the report opens; shown as a link after saving. */
  href?: string
}

/** Adding a published investigation to a dashboard draft. */
export interface InvestigationPlaceRequest {
  /** Stable across retries of the same destination choice. */
  id: string
  report: SavedInvestigation
  choice: ReportDestinationChoice
  context?: EffectiveContext
}

/**
 * The host side of "Save investigation". The SDK never stores the report:
 * save persists the proposal, place adds a published report to a dashboard
 * draft (the host opens that draft). Without destinations and place the
 * "Publish and add…" action is not offered.
 */
export interface InvestigationHost {
  save: (request: InvestigationSaveRequest, options: { signal: AbortSignal }) => Promise<SavedInvestigation>
  destinations?: ReportDestinationHost
  place?: (request: InvestigationPlaceRequest) => Promise<void>
  /**
   * The placement's source capabilities. Falls back to the report placement
   * host's capabilities; without either the slice is not saveable.
   */
  capabilities?: (placementId: string) => Pick<Capabilities, 'save' | 'drill' | 'export'> & Partial<Pick<Capabilities, 'dimensions' | 'operationDimensions'>> | undefined
}

/** A detached copy of plain data; store proxies cannot be structured-cloned. */
function plain<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

/** The innermost report group around a panel whose context names a source. */
export function placementForPanel(document: DashboardDocument, panelId: string): LayoutGroup | undefined {
  for (const row of document.layout.rows) {
    for (const item of row.panels) {
      if (item.panelId !== panelId) continue
      const groups = item.groups ?? []
      for (let index = groups.length - 1; index >= 0; index -= 1) {
        if (groups[index]!.filterContext?.input.source) return groups[index]
      }
    }
  }
  return undefined
}

/** A node's raw selection: the declared value, else the last key segment unescaped. */
export function nodeValue(node: { key: string; value?: string }): string {
  if (node.value) return node.value
  const segment = node.key.split('/').at(-1) ?? node.key
  try { return decodeURIComponent(segment) } catch { return segment }
}

/** The drill steps of a path, or the first level that declares no dimension. */
export function drillSteps(
  document: DashboardDocument,
  panel: Panel,
  path: NodePath,
  labelFor: (node: Level['children'][number], level: Level) => string,
): { steps: Array<InvestigationStep> } | { undeclared: Level } {
  const root = panel.drillRoot ? document.drill.edges[panel.drillRoot]?.path ?? [] : []
  const steps: Array<InvestigationStep> = []
  for (let index = Math.max(root.length, 1); index < path.length; index += 1) {
    const owner = levelForPath(document, path.slice(0, index))
    if (!owner) continue
    const key = path[index]!
    const node = owner.children.find((child) => child.key === key)
    // A branch child opens a perspective or a sub-tree; it selects nothing.
    if (!node || node.key === node.target) continue
    if (!owner.dimension) return { undeclared: owner }
    steps.push({ dimension: owner.dimension, value: nodeValue(node), label: labelFor(node, owner), levelLabel: owner.label.trim() || panel.title })
  }
  return { steps }
}

/** Whether a slice can be saved, and the slice itself when it can. */
export function investigationSaveability(options: {
  document: DashboardDocument
  panel: Panel
  navigation: NavigationView
  capabilities?: Pick<Capabilities, 'save' | 'drill' | 'export'> & Partial<Pick<Capabilities, 'dimensions' | 'operationDimensions'>>
  labelFor: (node: Level['children'][number], level: Level) => string
}): InvestigationSaveability {
  const { document, panel, navigation } = options
  const group = placementForPanel(document, panel.id)
  const context = group?.filterContext
  const source = context?.input.source
  if (!group || !context || !source) return { saveable: false, reason: 'not-a-report' }
  if (!options.capabilities?.save) return { saveable: false, reason: 'source-cannot-save' }
  if (!reportPlacementActions(options.capabilities, context).save) return { saveable: false, reason: 'context-blocked' }
  const walked = drillSteps(document, panel, navigation.path, options.labelFor)
  if ('undeclared' in walked) return { saveable: false, reason: 'level-undeclared', detail: walked.undeclared.label }
  const accepted = context.input.operationDimensions?.save ?? options.capabilities.operationDimensions?.save
    ?? options.capabilities.dimensions ?? context.input.supportedDimensions
  const refused = walked.steps.find((step) => !accepted.includes(step.dimension))
  if (refused) return { saveable: false, reason: 'dimension-unsupported', detail: refused.dimension }

  const constraints: Array<InvestigationConstraint> = context.restrictions.map((restriction) => ({
    dimension: restriction.dimension,
    values: [...restriction.values],
    labels: restriction.values.map((value) => document.contextEditor?.valueLabels?.[restriction.dimension]?.[value] ?? value),
    origin: { ...restriction.origin },
  }))
  for (const step of walked.steps) {
    const applied = context.intersection.find((constraint) => constraint.dimension === step.dimension)
    if (applied && !applied.values.includes(step.value)) {
      return { saveable: false, reason: 'drill-conflict', detail: step.dimension }
    }
    constraints.push({ dimension: step.dimension, values: [step.value], labels: [step.label], origin: { scope: 'drill', id: step.levelLabel } })
  }
  return {
    saveable: true,
    slice: {
      placementId: group.id,
      reportTitle: group.label?.trim() || group.caption?.trim() || panel.title,
      measure: { panelId: panel.id, title: panel.title },
      source: { ...source },
      calendar: { ...context.calendar },
      period: plain(context.period),
      constraints,
      steps: walked.steps,
      navigation: { panelId: navigation.panelId, path: [...navigation.path], perspectiveId: navigation.perspectiveId },
      context: plain(context),
    },
  }
}

/** The constraints a saved definition carries: every dimension once, values intersected. */
export function savedConstraints(slice: InvestigationSlice): Array<ContextConstraint> {
  const byDimension = new Map<string, Array<string>>()
  for (const constraint of slice.constraints) {
    const current = byDimension.get(constraint.dimension)
    byDimension.set(constraint.dimension, current ? current.filter((value) => constraint.values.includes(value)) : [...constraint.values])
  }
  return [...byDimension].map(([dimension, values]) => ({ dimension, values }))
}

/** The suggested name: the report, then the selections that narrowed it. */
export function suggestedInvestigationTitle(slice: InvestigationSlice): string {
  const labels = slice.steps.map((step) => step.label).filter(Boolean)
  return labels.length ? `${slice.reportTitle} — ${labels.join(' › ')}` : slice.reportTitle
}

export function investigationProposal(slice: InvestigationSlice, options: { title: string; dashboardId: string }): InvestigationProposal {
  return {
    title: options.title.trim(),
    source: { id: slice.source.id, version: slice.source.version },
    view: slice.source.view,
    filters: { period: plain(slice.period.selection), constraints: savedConstraints(slice) },
    origin: {
      dashboardId: options.dashboardId,
      placementId: slice.placementId,
      panelId: slice.measure.panelId,
      path: [...slice.navigation.path],
      ...(slice.navigation.perspectiveId ? { perspectiveId: slice.navigation.perspectiveId } : {}),
      trail: slice.steps.map((step) => step.label),
    },
  }
}

/**
 * What the overlay states about a position, saveable or not: the effective
 * period and every constraint with its origin, including the drill steps
 * whose levels declare a dimension.
 */
export function positionProvenance(
  document: DashboardDocument,
  panel: Panel,
  navigation: NavigationView,
  labelFor: (node: Level['children'][number], level: Level) => string,
): { context?: EffectiveContext; constraints: Array<InvestigationConstraint> } {
  const context = placementForPanel(document, panel.id)?.filterContext
  const constraints: Array<InvestigationConstraint> = (context?.restrictions ?? []).map((restriction) => ({
    dimension: restriction.dimension,
    values: [...restriction.values],
    labels: restriction.values.map((value) => document.contextEditor?.valueLabels?.[restriction.dimension]?.[value] ?? value),
    origin: { ...restriction.origin },
  }))
  const root = panel.drillRoot ? document.drill.edges[panel.drillRoot]?.path ?? [] : []
  for (let index = Math.max(root.length, 1); index < navigation.path.length; index += 1) {
    const owner = levelForPath(document, navigation.path.slice(0, index))
    const node = owner?.children.find((child) => child.key === navigation.path[index])
    if (!owner || !node || node.key === node.target || !owner.dimension) continue
    constraints.push({
      dimension: owner.dimension, values: [nodeValue(node)], labels: [labelFor(node, owner)],
      origin: { scope: 'drill', id: owner.label.trim() || panel.title },
    })
  }
  return { context, constraints }
}
