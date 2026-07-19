import { type KeyboardEvent, type ReactNode, useCallback, useEffect, useMemo, useRef } from 'react'
import type { Action, Frame, Level, Node, Panel } from '../contract'
import { PanelFrame, RegisteredPanel, type PanelRegistry } from '../panels'
import { levelForPath, useDashboard, useDrill, usePanelFrame } from '../runtime'
import { resolveLeafActionURL } from './actions'
import {
  breadcrumbsForNavigation,
  labelForNode,
  perspectivesForLevel,
  viewForSemantics,
} from './model'

interface TransitionDocument {
  startViewTransition?: (update: () => void) => unknown
}

function runViewTransition(update: () => void): void {
  const transitionDocument = globalThis.document as unknown as TransitionDocument
  if (transitionDocument.startViewTransition) transitionDocument.startViewTransition(update)
  else update()
}

function recordForRow(frame: Frame, row: Array<unknown>): Record<string, unknown> {
  return Object.fromEntries(frame.columns.map((column, index) => [column.name, row[index]]))
}

function variablesFromLocation(location: URL): Record<string, unknown> {
  const variables: Record<string, unknown> = {}
  for (const name of new Set(location.searchParams.keys())) {
    const values = location.searchParams.getAll(name)
    variables[name] = values.length > 1 ? values : values[0]
  }
  return variables
}

function matchingNode(level: Level, panel: Panel, frame: Frame, row: Array<unknown>, rowIndex: number): Node | undefined {
  const idField = level.encoding?.id ?? panel.encoding.id
  const idIndex = idField ? frame.columns.findIndex(({ name }) => name === idField) : -1
  const id = idIndex >= 0 ? String(row[idIndex]) : undefined
  return level.children.find(({ key }) => key === id || Boolean(id && key.endsWith(`/${id}`))) ?? level.children[rowIndex]
}

function fieldsForNode(level: Level, frame: Frame | undefined, node: Node): Record<string, unknown> {
  if (!frame) return {}
  const idField = level.encoding?.id
  const idIndex = idField ? frame.columns.findIndex(({ name }) => name === idField) : -1
  const id = node.key.split('/').at(-1)
  const row = idIndex >= 0 ? frame.rows.find((candidate) => String(candidate[idIndex]) === id) : undefined
  return row ? recordForRow(frame, row) : {}
}

function leafAction(node: Node | undefined, panel: Panel): Action | undefined {
  return node?.action ?? panel.actions.find(({ kind }) => kind === 'navigate_to_leaf')
}

function displayCell(value: unknown): string {
  if (value === null || value === undefined || value === '') return '—'
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'bigint' || typeof value === 'boolean') {
    return String(value)
  }
  return JSON.stringify(value) ?? '—'
}

function InterimRows({ panel, level, frame, kind }: { panel: Panel; level: Level; frame?: Frame; kind: 'cascade' | 'table' }) {
  const location = new URL(globalThis.location.href)
  const variables = variablesFromLocation(location)
  const rows = frame?.rows ?? []

  return (
    <section className="lens-interim-view" aria-label={`${panel.title} interim ${kind} view`}>
      <header className="lens-interim-header">
        <span className="lens-interim-kicker">Interim {kind} view</span>
        <span>Structured {kind} rendering arrives in A9.</span>
      </header>
      <ul className="lens-interim-rows">
        {rows.map((row, rowIndex) => {
          const fields = frame ? recordForRow(frame, row) : {}
          const node = frame ? matchingNode(level, panel, frame, row, rowIndex) : undefined
          const action = leafAction(node, panel)
          const href = action ? resolveLeafActionURL(action, { fields, variables, location }) : undefined
          return (
            <li className="lens-interim-row" key={node?.key ?? rowIndex}>
              <dl>
                {frame?.columns.map((column, columnIndex) => (
                  <div key={column.name}>
                    <dt>{column.name}</dt>
                    <dd>{displayCell(row[columnIndex])}</dd>
                  </div>
                ))}
              </dl>
              {href && <a className="lens-leaf-action" href={href}>Open record</a>}
            </li>
          )
        })}
      </ul>
    </section>
  )
}

interface SegmentTreeProps {
  document: ReturnType<typeof useDashboard>['document']
  level: Level
  frame?: Frame
  onDrill: (node: Node) => void
}

function SegmentTree({ document, level, frame, onDrill }: SegmentTreeProps) {
  const items = useRef<Array<HTMLElement | null>>([])
  if (!level.children.length) return null

  const moveFocus = (event: KeyboardEvent<HTMLElement>, index: number) => {
    const keys = ['ArrowRight', 'ArrowDown', 'ArrowLeft', 'ArrowUp', 'Home', 'End']
    if (!keys.includes(event.key)) return
    event.preventDefault()
    let target = index
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown') target = (index + 1) % level.children.length
    if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') target = (index - 1 + level.children.length) % level.children.length
    if (event.key === 'Home') target = 0
    if (event.key === 'End') target = level.children.length - 1
    items.current[target]?.focus()
  }

  return (
    <div className="lens-segment-tree" role="tree" aria-label={`Segments below ${level.label || 'current view'}`} aria-orientation="horizontal">
      {level.children.map((node, index) => {
        const label = labelForNode(node, level, document, frame)
        const target = node.target ? document.drill.edges[node.target] : undefined
        const perspectiveCount = target?.perspectives.length ?? 0
        if (node.action) {
          const location = new URL(globalThis.location.href)
          const action = resolveLeafActionURL(node.action, {
            fields: fieldsForNode(level, frame, node), variables: variablesFromLocation(location), location,
          })
          if (action) {
            return (
              <a
                className="lens-segment"
                href={action}
                key={node.key}
                onKeyDown={(event) => moveFocus(event, index)}
                ref={(element) => { items.current[index] = element }}
                role="treeitem"
              >
                <span>{label}</span><span aria-hidden="true">↗</span>
              </a>
            )
          }
        }
        return (
          <button
            aria-haspopup={perspectiveCount > 1 ? 'listbox' : undefined}
            className="lens-segment"
            key={node.key}
            onClick={() => onDrill(node)}
            onKeyDown={(event) => moveFocus(event, index)}
            ref={(element) => { items.current[index] = element }}
            role="treeitem"
            type="button"
          >
            <span>{label}</span>
            {perspectiveCount > 1 && (
              <span className="lens-perspective-affordance" aria-label={`${label} has ${perspectiveCount} perspectives`}>
                {perspectiveCount} views
              </span>
            )}
            <span className="lens-segment-arrow" aria-hidden="true">→</span>
          </button>
        )
      })}
    </div>
  )
}

export interface ExplorePanelProps {
  panel: Panel
  registry?: PanelRegistry
}

export function ExplorePanel({ panel, registry }: ExplorePanelProps) {
  const { document, navigation } = useDashboard()
  const drill = useDrill()
  const frame = usePanelFrame(panel.id)
  const active = navigation.panelId === panel.id && navigation.path.length > 0
  const level = active
    ? levelForPath(document, navigation.path)
    : (panel.drillRoot ? document.drill.edges[panel.drillRoot] : undefined)
  const perspectives = perspectivesForLevel(document, level)
  const perspective = active ? document.perspectives.find(({ id }) => id === navigation.perspectiveId) : undefined
  const hasPerspectiveChoice = perspectives.length > 1
  const semantics = perspective?.semantics ?? panel.semantics
  const kind = viewForSemantics(semantics, panel.kind)
  const viewPanel = useMemo<Panel>(() => ({
    ...panel,
    kind,
    title: level?.label.trim() || panel.title,
    semantics,
    encoding: level?.encoding ?? panel.encoding,
  }), [kind, level?.encoding, level?.label, panel, semantics])
  const breadcrumbs = breadcrumbsForNavigation(document, panel, navigation)
  const focusRef = useRef<HTMLDivElement>(null)
  const perspectiveFocusRef = useRef<HTMLButtonElement | null>(null)
  const perspectiveItems = useRef<Array<HTMLButtonElement | null>>([])
  const viewKey = `${active ? navigation.path.join('|') : panel.drillRoot ?? panel.id}:${navigation.perspectiveId ?? ''}`
  const previousView = useRef(viewKey)

  useEffect(() => {
    if (!active || perspectives.length !== 1 || perspectives[0]?.id === navigation.perspectiveId) return
    runViewTransition(() => drill.switchPerspective(perspectives[0]!.id))
  }, [active, drill, navigation.perspectiveId, perspectives])

  useEffect(() => {
    if (previousView.current !== viewKey) {
      previousView.current = viewKey
      const target = hasPerspectiveChoice ? perspectiveFocusRef.current : focusRef.current
      target?.focus({ preventScroll: true })
    }
  }, [hasPerspectiveChoice, viewKey])

  const selectNode = useCallback((node: Node) => {
    runViewTransition(() => drill.drillInto(node.key, panel.id))
  }, [drill, panel.id])

  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key !== 'Escape' || !active || !drill.canGoBack) return
    event.preventDefault()
    runViewTransition(drill.back)
  }

  const movePerspectiveFocus = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    if (!['ArrowRight', 'ArrowDown', 'ArrowLeft', 'ArrowUp'].includes(event.key)) return
    event.preventDefault()
    const direction = event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1 : -1
    const target = (index + direction + perspectives.length) % perspectives.length
    perspectiveItems.current[target]?.focus()
  }

  let content: ReactNode
  if (!level) content = <div className="lens-placeholder-state">This exploration level is unavailable.</div>
  else if (kind === 'cascade' || kind === 'table') {
    content = (
      <PanelFrame panel={viewPanel} frame={frame}>
        <InterimRows panel={viewPanel} level={level} frame={frame.data} kind={kind} />
      </PanelFrame>
    )
  } else {
    content = <RegisteredPanel panel={viewPanel} registry={registry} />
  }

  return (
    <article className="lens-explore" onKeyDown={onKeyDown} aria-label={`Explore ${panel.title}`}>
      <nav className="lens-explore-path" aria-label={`${panel.title} exploration path`}>
        <ol>
          {breadcrumbs.map((crumb) => (
            <li key={crumb.pathIndex}>
              <button
                aria-current={crumb.current ? 'page' : undefined}
                className="lens-crumb"
                onClick={() => {
                  if (!crumb.current) runViewTransition(() => drill.jumpTo(crumb.pathIndex))
                }}
                type="button"
              >
                <span>{crumb.label}</span>
                {crumb.perspectiveCount > 1 && (
                  <span className="lens-crumb-perspective">
                    {crumb.perspective?.label ?? `${crumb.perspectiveCount} views`}
                  </span>
                )}
              </button>
            </li>
          ))}
        </ol>
        {active && drill.canGoBack && (
          <button className="lens-explore-back" type="button" onClick={() => runViewTransition(drill.back)}>
            <span aria-hidden="true">←</span> Back
          </button>
        )}
      </nav>

      {hasPerspectiveChoice && (
        <div className="lens-perspective-set" role="listbox" aria-label={`Perspectives for ${level?.label || panel.title}`}>
          <span className="lens-perspective-label">View this segment as</span>
          {perspectives.map((item, index) => (
            <button
              aria-selected={item.id === navigation.perspectiveId}
              key={item.id}
              onClick={() => runViewTransition(() => drill.switchPerspective(item.id))}
              onKeyDown={(event) => movePerspectiveFocus(event, index)}
              ref={(element) => {
                perspectiveItems.current[index] = element
                if (item.id === navigation.perspectiveId || (!navigation.perspectiveId && index === 0)) {
                  perspectiveFocusRef.current = element
                }
              }}
              role="option"
              type="button"
            >
              <span>{item.label}</span>
              <small>{item.semantics}</small>
            </button>
          ))}
        </div>
      )}

      <div
        className="lens-explore-level"
        data-explore-view={kind}
        key={viewKey}
        ref={focusRef}
        tabIndex={-1}
      >
        {content}
      </div>
      {level && <SegmentTree document={document} level={level} frame={frame.data} onDrill={selectNode} />}
    </article>
  )
}
