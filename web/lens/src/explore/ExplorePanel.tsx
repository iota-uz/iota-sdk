import {
  type CSSProperties,
  type KeyboardEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from 'react'
import type { Encoding, FieldFormat, Frame, Level, Node, Panel } from '../contract'
import { RegisteredPanel, type PanelRegistry } from '../panels'
import { levelForPath, useDashboard, useDrill, usePanelFrame } from '../runtime'
import { recordForRow, resolveLeafActionURL, variablesFromLocation } from './actions'
import {
  breadcrumbsForNavigation,
  labelForNode,
  perspectivesForLevel,
  rowForNode,
  viewForSemantics,
} from './model'

interface ViewTransition {
  ready?: Promise<unknown>
  finished?: Promise<unknown>
}

interface TransitionDocument {
  startViewTransition?: (update: () => void) => ViewTransition
}

let activeLensTransitions = 0

function runViewTransition(update: () => void): void {
  const transitionDocument = globalThis.document as unknown as TransitionDocument
  const reduceMotion = globalThis.window?.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  if (!transitionDocument.startViewTransition || reduceMotion) {
    update()
    return
  }

  activeLensTransitions += 1
  globalThis.document.documentElement.classList.add('lens-explore-transition-active')
  const transition = transitionDocument.startViewTransition(update)
  void transition.ready?.catch(() => undefined)
  void transition.finished?.catch(() => undefined).finally(() => {
    activeLensTransitions -= 1
    if (activeLensTransitions === 0) {
      globalThis.document.documentElement.classList.remove('lens-explore-transition-active')
    }
  })
}

function fieldsForNode(level: Level, frame: Frame | undefined, node: Node): Record<string, unknown> {
  if (!frame) return {}
  const row = rowForNode(node, level, frame)
  return row ? recordForRow(frame, row) : {}
}

function formatsForEncoding(panel: Panel, encoding: Encoding): Record<string, FieldFormat> {
  const formats = { ...panel.format }
  for (const [role, targetField] of Object.entries(encoding) as Array<[keyof Encoding, string | undefined]>) {
    const sourceField = panel.encoding[role]
    if (targetField && sourceField && panel.format[sourceField] && !formats[targetField]) {
      formats[targetField] = panel.format[sourceField]
    }
  }
  return formats
}

interface SegmentTreeProps {
  document: ReturnType<typeof useDashboard>['document']
  level: Level
  frame?: Frame
  onDrill: (node: Node) => void
}

function SegmentTree({ document, level, frame, onDrill }: SegmentTreeProps) {
  const items = useRef<Array<HTMLElement | null>>([])
  const [rovingKey, setRovingKey] = useState(level.children[0]?.key)
  if (!level.children.length) return null

  const rovingIndex = Math.max(0, level.children.findIndex(({ key }) => key === rovingKey))

  const moveFocus = (event: KeyboardEvent<HTMLElement>, index: number) => {
    const keys = ['ArrowRight', 'ArrowDown', 'ArrowLeft', 'ArrowUp', 'Home', 'End']
    if (!keys.includes(event.key)) return
    event.preventDefault()
    let target = index
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown') target = (index + 1) % level.children.length
    if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') target = (index - 1 + level.children.length) % level.children.length
    if (event.key === 'Home') target = 0
    if (event.key === 'End') target = level.children.length - 1
    setRovingKey(level.children[target]?.key)
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
                onFocus={() => setRovingKey(node.key)}
                onKeyDown={(event) => {
                  if (event.key === ' ') {
                    event.preventDefault()
                    event.currentTarget.click()
                    return
                  }
                  moveFocus(event, index)
                }}
                ref={(element) => { items.current[index] = element }}
                role="treeitem"
                tabIndex={index === rovingIndex ? 0 : -1}
              >
                <span>{label}</span><span aria-hidden="true">↗</span>
              </a>
            )
          }
        }
        return (
          <button
            className="lens-segment"
            key={node.key}
            onClick={() => {
              setRovingKey(node.key)
              onDrill(node)
            }}
            onFocus={() => setRovingKey(node.key)}
            onKeyDown={(event) => moveFocus(event, index)}
            ref={(element) => { items.current[index] = element }}
            role="treeitem"
            tabIndex={index === rovingIndex ? 0 : -1}
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
  const perspectives = useMemo(() => perspectivesForLevel(document, level), [document, level])
  const perspective = active ? document.perspectives.find(({ id }) => id === navigation.perspectiveId) : undefined
  const hasPerspectiveChoice = perspectives.length > 1
  const semantics = perspective?.semantics ?? panel.semantics
  const kind = viewForSemantics(semantics, panel.kind)
  const viewPanel = useMemo<Panel>(() => {
    const encoding = level?.encoding ?? panel.encoding
    return {
      ...panel,
      kind,
      title: level?.label.trim() || panel.title,
      semantics,
      encoding,
      format: formatsForEncoding(panel, encoding),
    }
  }, [kind, level?.encoding, level?.label, panel, semantics])
  const breadcrumbs = breadcrumbsForNavigation(document, panel, navigation)
  const focusRef = useRef<HTMLDivElement>(null)
  const perspectiveItems = useRef<Array<HTMLButtonElement | null>>([])
  const [rovingPerspectiveId, setRovingPerspectiveId] = useState<string>()
  const instanceId = useId()
  const viewKey = `${active ? navigation.path.join('|') : panel.drillRoot ?? panel.id}:${navigation.perspectiveId ?? ''}`
  const previousView = useRef(viewKey)
  const selectedPerspectiveIndex = Math.max(0, perspectives.findIndex(({ id }) => id === navigation.perspectiveId))
  const storedPerspectiveIndex = perspectives.findIndex(({ id }) => id === rovingPerspectiveId)
  const rovingPerspectiveIndex = storedPerspectiveIndex >= 0 ? storedPerspectiveIndex : selectedPerspectiveIndex
  const transitionName = useMemo(() => {
    const identifier = `${panel.id}-${instanceId}`.replace(/[^a-zA-Z0-9_-]/g, '-')
    return `lens-explore-${identifier}`
  }, [instanceId, panel.id])
  const transitionStyle = useMemo(() => ({
    viewTransitionName: transitionName,
    viewTransitionClass: 'lens-explore-level-transition',
  }) as CSSProperties, [transitionName])

  useEffect(() => {
    if (!active || perspectives.length !== 1 || perspectives[0]?.id === navigation.perspectiveId) return
    runViewTransition(() => drill.switchPerspective(perspectives[0]!.id, { replace: true }))
  }, [active, drill, navigation.perspectiveId, perspectives])

  useEffect(() => {
    if (previousView.current !== viewKey) {
      previousView.current = viewKey
      const target = hasPerspectiveChoice ? perspectiveItems.current[rovingPerspectiveIndex] : focusRef.current
      target?.focus({ preventScroll: true })
    }
  }, [hasPerspectiveChoice, rovingPerspectiveIndex, viewKey])

  useEffect(() => {
    const element = focusRef.current
    const transitionDocument = globalThis.document as unknown as TransitionDocument
    if (!element || transitionDocument.startViewTransition ||
      globalThis.window?.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
    element.classList.remove('lens-explore-level-enter')
    const animationFrame = globalThis.requestAnimationFrame(() => element.classList.add('lens-explore-level-enter'))
    return () => globalThis.cancelAnimationFrame(animationFrame)
  }, [viewKey])

  const selectNode = useCallback((node: Node) => {
    runViewTransition(() => drill.drillInto(node.key, panel.id))
  }, [drill, panel.id])

  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key !== 'Escape' || !active || !drill.canGoBack) return
    event.preventDefault()
    runViewTransition(drill.back)
  }

  const movePerspectiveFocus = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    if (!['ArrowRight', 'ArrowDown', 'ArrowLeft', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    let target = index
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown') target = (index + 1) % perspectives.length
    if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') target = (index - 1 + perspectives.length) % perspectives.length
    if (event.key === 'Home') target = 0
    if (event.key === 'End') target = perspectives.length - 1
    setRovingPerspectiveId(perspectives[target]?.id)
    perspectiveItems.current[target]?.focus()
  }

  let content: ReactNode
  if (!level) content = <div className="lens-placeholder-state">This exploration level is unavailable.</div>
  else content = <RegisteredPanel panel={viewPanel} registry={registry} />

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
              onClick={() => {
                setRovingPerspectiveId(item.id)
                runViewTransition(() => drill.switchPerspective(item.id))
              }}
              onFocus={() => setRovingPerspectiveId(item.id)}
              onKeyDown={(event) => movePerspectiveFocus(event, index)}
              ref={(element) => { perspectiveItems.current[index] = element }}
              role="option"
              tabIndex={index === rovingPerspectiveIndex ? 0 : -1}
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
        ref={focusRef}
        style={transitionStyle}
        tabIndex={-1}
      >
        {content}
      </div>
      {level && <SegmentTree document={document} level={level} frame={frame.data} onDrill={selectNode} />}
    </article>
  )
}
