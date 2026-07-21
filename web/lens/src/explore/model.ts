import type {
  DashboardDocument,
  Frame,
  Level,
  Node,
  Panel,
  PanelKind,
  Perspective,
  Semantics,
} from '../contract'
import { isPerspectiveFork, levelForPath, type NavigationView } from '../runtime'

export type ExploreViewKind = Extract<PanelKind, 'bar' | 'cascade' | 'donut' | 'hbar' | 'line' | 'pie' | 'table'>

const partitionViews: ReadonlySet<PanelKind> = new Set(['pie', 'donut', 'bar', 'hbar'])

export function viewForSemantics(semantics: Semantics, preferred?: PanelKind): ExploreViewKind {
  switch (semantics) {
    case 'partition':
      return preferred && partitionViews.has(preferred) ? preferred as ExploreViewKind : 'donut'
    case 'reconciliation':
      return 'cascade'
    case 'series':
      return 'line'
    case 'evidence':
      return 'table'
  }
}

export function perspectivesForLevel(document: DashboardDocument, level: Level | undefined): Array<Perspective> {
  if (!level) return []
  const ids = new Set(level.perspectives.map(({ id }) => id))
  return document.perspectives.filter(({ id }) => ids.has(id))
}

export interface ExploreBreadcrumb {
  label: string
  pathIndex: number
  current: boolean
  perspective?: Perspective
  perspectiveCount: number
}

export function breadcrumbsForNavigation(
  document: DashboardDocument,
  panel: Panel,
  navigation: NavigationView,
): Array<ExploreBreadcrumb> {
  const active = navigation.panelId === panel.id && navigation.path.length > 0
  const path = active ? navigation.path : (panel.drillRoot ? document.drill.edges[panel.drillRoot]?.path ?? [] : [])
  const crumbs: Array<ExploreBreadcrumb> = []

  for (let pathIndex = 0; pathIndex < path.length; pathIndex += 1) {
    const level = levelForPath(document, path.slice(0, pathIndex + 1))
    if (!level) continue
    const perspectives = perspectivesForLevel(document, level)
    const perspective = perspectives.find(({ id }) => id === navigation.perspectiveId)
    const label = level.label.trim() || (pathIndex === 0 ? panel.title : '')
    if (!label) continue
    crumbs.push({ label, pathIndex, current: false, perspective, perspectiveCount: perspectives.length })
  }

  const current = crumbs.at(-1)
  if (current) current.current = true
  return crumbs
}

export function childForSelection(level: Level | undefined, selectedKey: string): Node | undefined {
  return level?.children.find(({ key }) => key === selectedKey || key.endsWith(`/${selectedKey}`))
}

function rowValue(frame: Frame | undefined, row: Array<unknown> | undefined, field: string | undefined): unknown {
  if (!frame || !row || !field) return undefined
  const index = frame.columns.findIndex(({ name }) => name === field)
  return index < 0 ? undefined : row[index]
}

export function rowForNode(node: Node, level: Level, frame: Frame | undefined): Array<unknown> | undefined {
  if (!frame || !level.encoding?.id) return undefined
  return frame.rows.find((row) => {
    const id = rowValue(frame, row, level.encoding?.id)
    if (typeof id !== 'string' && typeof id !== 'number' && typeof id !== 'bigint' && typeof id !== 'boolean') return false
    const value = String(id)
    return node.key === value || node.key.endsWith(`/${value}`)
  })
}

export function labelForNode(node: Node, level: Level, document: DashboardDocument, frame: Frame | undefined): string {
  if (node.label.trim()) return node.label
  const id = node.key.split('/').at(-1)
  const row = rowForNode(node, level, frame)
  const label = rowValue(frame, row, level.encoding?.label)
  if (typeof label === 'string' && label.trim()) return label
  const targetLabel = node.target ? document.drill.edges[node.target]?.label.trim() : ''
  return targetLabel || id || node.key
}

export interface DrillBreakdownRow {
  node: Node
  label: string
  value?: number
  share?: number
  /** Resolved leaf URL when the child is a record rather than a level. */
  href?: string
}

export interface DrillTarget {
  /** The node the overlay describes; absent when it describes the level itself. */
  node?: Node
  label: string
  value?: number
  share?: number
  /** Level the overlay can drill into, i.e. what the segment expands to. */
  target?: Level
  /**
   * True when what the segment expands to is a perspective fork — a level that
   * owns no data and whose only content is the choice between its perspectives.
   * The overlay already offers that choice, so expanding into it would land the
   * user on a card that asks the same question again.
   */
  expandsToFork?: boolean
  breakdown: Array<DrillBreakdownRow>
  perspectives: Array<Perspective>
  leafHref?: string
}

function numeric(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

function valueForNode(node: Node, level: Level, frame: Frame | undefined): number | undefined {
  const row = rowForNode(node, level, frame)
  return numeric(rowValue(frame, row, level.encoding?.value))
}

/**
 * Describes what a user can do with one mark: how big it is relative to its
 * siblings, what it expands into, which perspectives its target level offers,
 * and whether it has a leaf route. The chart shows the level; this describes
 * the segment, which is the unit the overlay acts on.
 */
export function drillTargetForNode(
  document: DashboardDocument,
  level: Level,
  node: Node,
  frame: Frame | undefined,
  targetFrame: Frame | undefined,
): DrillTarget {
  const target = node.target ? document.drill.edges[node.target] : undefined
  const value = valueForNode(node, level, frame)
  const siblingTotal = level.children.reduce((sum, child) => sum + (valueForNode(child, level, frame) ?? 0), 0)
  const breakdownValues = (target?.children ?? []).map((child) => ({
    node: child,
    label: labelForNode(child, target!, document, targetFrame),
    value: target ? valueForNode(child, target, targetFrame) : undefined,
  }))
  const breakdownTotal = breakdownValues.reduce((sum, row) => sum + (row.value ?? 0), 0)
  const breakdown = breakdownValues
    .map((row) => ({ ...row, share: row.value !== undefined && breakdownTotal > 0 ? row.value / breakdownTotal : undefined }))
    .sort((left, right) => (right.value ?? 0) - (left.value ?? 0))

  return {
    node,
    label: labelForNode(node, level, document, frame),
    value,
    share: value !== undefined && siblingTotal > 0 ? value / siblingTotal : undefined,
    target,
    expandsToFork: target ? isPerspectiveFork(document, target) : false,
    breakdown,
    perspectives: perspectivesForLevel(document, target),
  }
}

/** The overlay opened from the panel header describes the current level. */
export function drillTargetForLevel(
  document: DashboardDocument,
  panel: Panel,
  level: Level,
  frame: Frame | undefined,
): DrillTarget {
  const values = level.children.map((child) => ({
    node: child,
    label: labelForNode(child, level, document, frame),
    value: valueForNode(child, level, frame),
  }))
  const total = values.reduce((sum, row) => sum + (row.value ?? 0), 0)
  return {
    label: level.label.trim() || panel.title,
    target: level,
    breakdown: values
      .map((row) => ({ ...row, share: row.value !== undefined && total > 0 ? row.value / total : undefined }))
      .sort((left, right) => (right.value ?? 0) - (left.value ?? 0)),
    perspectives: perspectivesForLevel(document, level),
  }
}
