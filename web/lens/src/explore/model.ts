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
import { levelForPath, type NavigationView } from '../runtime'

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
