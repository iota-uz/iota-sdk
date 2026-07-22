import type { DashboardDocument, Frame, Level, NodePath, Panel } from '../contract'
import type { NavigationView } from './navigation'

export function sameNodePath(left: NodePath, right: NodePath): boolean {
  return left.length === right.length && left.every((key, index) => key === right[index])
}

function levelByOwnPath(document: DashboardDocument, path: NodePath): Level | undefined {
  for (const level of Object.values(document.drill.edges)) {
    if (sameNodePath(level.path, path)) return level
  }
  return undefined
}

interface ResolvedDrillPath {
  level: Level
  queryPath: NodePath
}

/**
 * Resolves a navigation path to the level it addresses. A path is an owning
 * level's canonical path followed by zero or more child keys, each a concrete
 * selection (a point, or the "other" bucket) whose edge leads to the next
 * level. Levels reached through a point are parameterised by it — the same
 * node aggregates 2025 or 2024 depending on which slice was entered — so the
 * selection has to survive in the path instead of collapsing onto the target
 * node's canonical ancestry.
 *
 * `queryPath` is the same walk in the wire shape the snapshot query endpoint
 * parses: each selection interleaved with the node it selects into
 * (`[…, root, "2026", detail, "direct", detail-2]`).
 */
export function resolveDrillPath(document: DashboardDocument, path: NodePath): ResolvedDrillPath | undefined {
  for (let prefix = path.length; prefix > 0; prefix -= 1) {
    const base = levelByOwnPath(document, path.slice(0, prefix))
    if (!base) continue
    const queryPath: NodePath = [...path.slice(0, prefix)]
    let level: Level | undefined = base
    for (let index = prefix; index < path.length && level; index += 1) {
      const key = path[index]!
      const child: Level['children'][number] | undefined =
        level.children.find((candidate) => candidate.key === key)
      const target: Level | undefined = child?.target ? document.drill.edges[child.target] : undefined
      if (!child?.target || !target) {
        level = undefined
        break
      }
      queryPath.push(key)
      // A branch child is keyed by the node it opens; repeating it would say
      // the same step twice. A point child is keyed by the selection, so the
      // node it selects into follows it.
      if (child.key !== child.target) queryPath.push(child.target)
      level = target
    }
    if (level) return { level, queryPath }
  }
  return undefined
}

export function levelForPath(document: DashboardDocument, path: NodePath): Level | undefined {
  return resolveDrillPath(document, path)?.level
}

/**
 * The wire path for a navigation path: selections interleaved with the nodes
 * they select into. An unresolvable path passes through unchanged so the
 * server, not the client, owns the rejection.
 */
export function queryPathForNavigation(document: DashboardDocument, path: NodePath): NodePath {
  return resolveDrillPath(document, path)?.queryPath ?? [...path]
}

/**
 * A fork in the drill path: a node that carries no data of its own because its
 * perspectives (which branch off it) own it. Panels must not draw anything at
 * such a node until a perspective is chosen — the parent's numbers under the
 * child's title read as fact and are wrong.
 */
export function isPerspectiveFork(document: DashboardDocument, level: Level): boolean {
  if (level.frame) return false
  const levelKey = level.path.at(-1)
  return level.perspectives.some(({ id }) => (
    document.perspectives.find((candidate) => candidate.id === id)?.branchKey === levelKey
  ))
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

export function dynamicParentPath(document: DashboardDocument, path: NodePath): NodePath | undefined {
  if (pathResolves(document, path)) return undefined
  for (let length = path.length - 1; length > 0; length -= 1) {
    const prefix = path.slice(0, length)
    const level = levelForPath(document, prefix)
    if (level?.dynamicChildren) return prefix
  }
  return undefined
}

export function withFrameChildren(document: DashboardDocument, path: NodePath, frame: Frame): DashboardDocument {
  if (!frame.children) return document
  const resolved = resolveDrillPath(document, path)
  if (!resolved?.level.dynamicChildren) return document
  const entry = Object.entries(document.drill.edges).find(([, level]) => level === resolved.level)
  if (!entry) return document
  const [key, level] = entry
  return {
    ...document,
    drill: {
      ...document.drill,
      edges: { ...document.drill.edges, [key]: { ...level, children: frame.children } },
    },
  }
}

export function withInlineFrameChildren(document: DashboardDocument): DashboardDocument {
  let resolved = document
  for (const level of Object.values(document.drill.edges)) {
    const frame = level.frame ? document.frames[level.frame] : undefined
    if (frame?.children) resolved = withFrameChildren(resolved, level.path, frame)
  }
  return resolved
}
