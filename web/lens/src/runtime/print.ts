import type {
  Action,
  DashboardDocument,
  Frame,
  Level,
  NodePath,
  Panel,
  Perspective,
  QueryRequest,
  QueryResponse,
} from '../contract'
import {
  isPerspectiveFork,
  levelForPath,
  perspectivesForPosition,
  queryPathForNavigation,
  withFrameChildren,
  withInlineFrameChildren,
} from './drill'
import {
  recordForRow,
  resolveActionURL,
  variablesFromLocation,
} from '../explore/actions'

export interface PrintSection {
  id: string
  document: DashboardDocument
  panel: Panel
  level: Level
  frame?: Frame
  path: NodePath
  breadcrumb: Array<string>
  perspective?: Perspective
  depth: number
  root: boolean
}

export interface PrintReport {
  document: DashboardDocument
  sections: Array<PrintSection>
  warnings: Array<string>
  truncated: boolean
}

export type PrintQuery = (
  request: QueryRequest,
  document?: DashboardDocument,
) => Promise<QueryResponse>

export type PrintDocumentLoader = (src: string) => Promise<DashboardDocument>

function frameFromResponse(response: QueryResponse): Frame | undefined {
  return Object.values(response.frames)[0]
}

function pathKey(document: DashboardDocument, panel: Panel, path: NodePath, perspective?: Perspective): string {
  return JSON.stringify([document.snapshotId, panel.id, path, perspective?.id ?? ''])
}

interface ActionCandidate {
  action: Action
  label: string
  row?: Array<unknown>
}

function actionIsRowScoped(action: Action): boolean {
  if (action.urlSource) return action.urlSource.kind === 'field'
  return action.params.some((param) => param.source.kind === 'field')
    || Object.values(action.payload).some((source) => source.kind === 'field')
}

function actionCandidates(panel: Panel, level: Level, frame?: Frame): Array<ActionCandidate> {
  const candidates: Array<ActionCandidate> = []
  const add = (action: Action | undefined, label: string, row?: Array<unknown>) => {
    if (action?.kind === 'open_drawer') candidates.push({ action, label, ...(row ? { row } : {}) })
  }
  const addForRows = (action: Action | undefined, label: string) => {
    if (!action || action.kind !== 'open_drawer') return
    if (frame && actionIsRowScoped(action)) {
      // Every chart sector, flow stage, hierarchy row and table row is already
      // represented by the exact-value evidence table printed with the panel.
      // Opening the same external drawer once per row would turn a company
      // audit into hundreds of near-identical entity reports. The dashboard's
      // actual analytical drill graph is expanded separately by `walk`; only
      // unscoped drawers add another company-level lens here.
      return
    }
    add(action, label, frame?.rows[0])
  }

  for (const action of panel.actions) addForRows(action, panel.title)
  for (const column of panel.columns ?? []) addForRows(column.action, column.label)
  for (const stage of panel.metricFlow?.stages ?? []) addForRows(stage.action, stage.label)
  for (const row of panel.metricHierarchy?.rows ?? []) addForRows(row.action, row.label)
  if (panel.metricRelationship) {
    addForRows(panel.metricRelationship.source.action, panel.metricRelationship.source.label)
    addForRows(panel.metricRelationship.target.action, panel.metricRelationship.target.label)
  }
  for (const node of level.children) {
    if (!node.action || node.action.kind !== 'open_drawer') continue
    const matchingRow = frame?.rows.find((row) => {
      const idField = level.encoding?.id ?? panel.encoding.id
      const index = idField ? frame.columns.findIndex(({ name }) => name === idField) : -1
      return index >= 0 && String(row[index]) === node.key
    })
    add(node.action, node.label, matchingRow)
  }
  return candidates
}

/**
 * Materializes the dashboard's drill graph into a linear print report.
 *
 * The interactive runtime resolves one path at a time. Printing needs the
 * inverse: every branch must exist at once, including dynamic children only
 * known after their parent frame is queried. The walker uses the canonical
 * query contract and merges child descriptors into an isolated document copy,
 * leaving the live dashboard and its URL navigation untouched.
 */
export async function buildPrintReport(
  source: DashboardDocument,
  query?: PrintQuery,
  maxSections = 2_000,
  loadDocument?: PrintDocumentLoader,
  location = new URL(
    typeof globalThis.location === 'undefined'
      ? 'http://localhost/'
      : globalThis.location.href,
  ),
  signal?: AbortSignal,
): Promise<PrintReport> {
  const sections: Array<PrintSection> = []
  const warnings: Array<string> = []
  let truncated = false
  const visited = new Set<string>()
  const visitedDocuments = new Set<string>()
  const visitedDocumentIdentities = new Set<string>()
  const queriedFrames = new Map<string, Promise<Frame | undefined>>()
  const pendingDocuments: Array<{
    href: string
    candidate: ActionCandidate
    breadcrumb: Array<string>
    depth: number
  }> = []

  const documentIdentity = (document: DashboardDocument) => JSON.stringify([
    document.meta.dashboardId,
    document.drawer?.title ?? '',
    document.header?.title ?? '',
  ])
  visitedDocumentIdentities.add(documentIdentity(source))

  const shouldStop = () => {
    if (!signal?.aborted) return false
    truncated = true
    return true
  }

  const loadFrame = (
    document: DashboardDocument,
    level: Level,
    path: NodePath,
    perspective?: Perspective,
  ): Promise<Frame | undefined> => {
    if (!query || shouldStop()) return Promise.resolve(undefined)
    const key = JSON.stringify([document.snapshotId, path, perspective?.id ?? ''])
    const existing = queriedFrames.get(key)
    if (existing) return existing
    const pending = query({
      snapshotId: document.snapshotId,
      path: queryPathForNavigation(document, path),
      ...(perspective ? { perspective: perspective.id } : {}),
    }, document).then(frameFromResponse).catch((cause: unknown) => {
      const detail = cause instanceof Error ? cause.message : 'query request failed'
      warnings.push(`${document.meta.title} › ${level.label}: ${detail}`)
      return undefined
    })
    queriedFrames.set(key, pending)
    return pending
  }

  const walkDocument = async (
    input: DashboardDocument,
    prefix: Array<string>,
    documentDepth: number,
  ): Promise<void> => {
    if (shouldStop()) return
    if (documentDepth > 12) {
      warnings.push(`${input.meta.title}: nested detail depth exceeds 12 levels`)
      return
    }
    let document = withInlineFrameChildren(input)

    const collectActions = (
      panel: Panel,
      level: Level,
      frame: Frame | undefined,
      breadcrumb: Array<string>,
      depth: number,
    ) => {
      if (!loadDocument) return
      for (const candidate of actionCandidates(panel, level, frame)) {
        if (shouldStop()) break
        const href = resolveActionURL(candidate.action, {
          fields: candidate.row && frame ? recordForRow(frame, candidate.row) : {},
          variables: variablesFromLocation(location),
          location,
        })
        if (!href || visitedDocuments.has(href)) continue
        visitedDocuments.add(href)
        pendingDocuments.push({ href, candidate, breadcrumb, depth })
      }
    }

    const walk = async (
      panel: Panel,
      path: NodePath,
      perspective: Perspective | undefined,
      breadcrumb: Array<string>,
      depth: number,
      root: boolean,
    ): Promise<void> => {
      if (shouldStop()) return
      if (sections.length >= maxSections) {
        throw new Error(`print report exceeds ${maxSections} sections`)
      }
      const level = levelForPath(document, path)
      if (!level) return
      const visitKey = pathKey(document, panel, path, perspective)
      if (visited.has(visitKey)) return
      visited.add(visitKey)

      if (isPerspectiveFork(document, level)) {
        const candidates = perspectivesForPosition(document, level)
        // Perspective frames are independent queries. Warm them in parallel,
        // then walk them in authored order so the printed report stays stable.
        await Promise.all(candidates.map((candidate) => {
          const branch = document.drill.edges[candidate.root]
          if (!branch || (branch.frame && document.frames[branch.frame])) return undefined
          return loadFrame(document, branch, branch.path, candidate)
        }))
        for (const candidate of candidates) {
          if (shouldStop()) break
          const branch = document.drill.edges[candidate.root]
          if (!branch) continue
          await walk(panel, branch.path, candidate, [...breadcrumb, candidate.label], depth, false)
        }
        return
      }

      let frame = level.frame ? document.frames[level.frame] : undefined
      if (!frame) frame = await loadFrame(document, level, path, perspective)
      if (frame?.children) document = withFrameChildren(document, path, frame)

      // Resolve once more after dynamic children were merged.
      const resolved = levelForPath(document, path) ?? level
      const label = resolved.label.trim() || panel.title
      sections.push({
        id: `${panel.id}:${sections.length}`,
        document,
        panel,
        level: resolved,
        frame,
        path: [...path],
        breadcrumb: [...prefix, ...breadcrumb, label],
        perspective,
        depth: documentDepth + depth,
        root: root && documentDepth === 0,
      })
      collectActions(
        panel,
        resolved,
        frame,
        [...prefix, ...breadcrumb, label],
        documentDepth + depth + 1,
      )

      for (const child of resolved.children) {
        if (shouldStop()) break
        if (!child.target) continue
        await walk(
          panel,
          [...path, child.key],
          perspective,
          [...breadcrumb, label, child.label],
          depth + 1,
          false,
        )
      }
    }

    for (const panel of document.panels) {
      if (shouldStop()) break
      if (sections.length >= maxSections) {
        throw new Error(`print report exceeds ${maxSections} sections`)
      }
      if (!panel.drillRoot) {
        const frame = document.frames[panel.frame]
        const level: Level = {
          path: [],
          label: panel.title,
          children: [],
          perspectives: [],
          ...(frame ? { frame: panel.frame } : {}),
        }
        const breadcrumb = [...prefix, panel.title]
        sections.push({
          id: `${panel.id}:${sections.length}`,
          document,
          panel,
          level,
          frame,
          path: [],
          breadcrumb,
          depth: documentDepth,
          root: documentDepth === 0,
        })
        collectActions(panel, level, frame, breadcrumb, documentDepth + 1)
        continue
      }
      const rootLevel = document.drill.edges[panel.drillRoot]
      if (rootLevel) await walk(panel, rootLevel.path, undefined, [], 0, true)
    }
  }

  await walkDocument(source, [], 0)
  while (pendingDocuments.length > 0 && !shouldStop()) {
    const batch = pendingDocuments.splice(0, 4)
    const loaded = await Promise.all(batch.map(async (entry) => {
      try {
        return { entry, document: await loadDocument!(entry.href) }
      } catch (cause: unknown) {
        const detail = cause instanceof Error ? cause.message : 'document request failed'
        warnings.push(`${entry.candidate.label}: ${detail}`)
        return { entry, document: undefined }
      }
    }))
    for (const { entry, document: nested } of loaded) {
      if (!nested || shouldStop()) continue
      const identity = documentIdentity(nested)
      if (visitedDocumentIdentities.has(identity)) continue
      visitedDocumentIdentities.add(identity)
      const nestedTitle = nested.drawer?.title || nested.header?.title || nested.meta.title
      const actionTrail = entry.candidate.label && entry.candidate.label !== entry.breadcrumb.at(-1)
        ? [...entry.breadcrumb, entry.candidate.label]
        : entry.breadcrumb
      await walkDocument(
        nested,
        nestedTitle && nestedTitle !== actionTrail.at(-1)
          ? [...actionTrail, nestedTitle]
          : actionTrail,
        entry.depth,
      )
    }
  }
  return { document: source, sections, warnings, truncated }
}
