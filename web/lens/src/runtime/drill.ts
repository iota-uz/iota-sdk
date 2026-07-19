import type { DashboardDocument, Level, NodePath, Panel } from '../contract'
import type { NavigationView } from './navigation'

export function sameNodePath(left: NodePath, right: NodePath): boolean {
  return left.length === right.length && left.every((key, index) => key === right[index])
}

export function levelForPath(document: DashboardDocument, path: NodePath): Level | undefined {
  for (const level of Object.values(document.drill.edges)) {
    if (sameNodePath(level.path, path)) return level
    const child = level.children.find((candidate) => sameNodePath(candidate.path, path))
    if (child?.target) return document.drill.edges[child.target]
  }
  return undefined
}

export function pathResolves(document: DashboardDocument, path: NodePath, perspectiveId?: string): boolean {
  if (path.length === 0) return true
  const level = levelForPath(document, path)
  if (!level) return false
  if (!perspectiveId) return true
  return level.perspectives.some((perspective) => perspective.id === perspectiveId)
}

export function panelForNavigation(document: DashboardDocument, view: NavigationView): Panel | undefined {
  if (view.panelId) return document.panels.find((panel) => panel.id === view.panelId)
  if (view.path.length === 0) return undefined
  return document.panels.find((panel) => {
    if (!panel.drillRoot) return false
    const root = document.drill.edges[panel.drillRoot]?.path
    return root ? root.every((key, index) => view.path[index] === key) : false
  })
}

export function rootNavigation(document: DashboardDocument, panelId?: string): NavigationView {
  const panel = document.panels.find((candidate) => candidate.id === panelId)
  const path = panel?.drillRoot ? document.drill.edges[panel.drillRoot]?.path ?? [] : []
  return { panelId: panel?.id, path: [...path] }
}

export function replayNavigation(document: DashboardDocument, view: NavigationView): NavigationView | undefined {
  if (!pathResolves(document, view.path, view.perspectiveId)) return undefined
  const perspectiveId = view.perspectiveId && document.perspectives.some(({ id }) => id === view.perspectiveId)
    ? view.perspectiveId
    : undefined
  const panel = panelForNavigation(document, view)
  return { panelId: panel?.id, path: [...view.path], perspectiveId }
}
