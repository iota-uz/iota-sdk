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
import type { ChartAnchor } from '../charts/adapter'
import type { Encoding, FieldFormat, Frame, Level, Node, NodeKey, Panel } from '../contract'
import { CaretDown, CaretLeft, CaretRight } from '../icons'
import { MarkSelectionContext, PanelChromeContext, RegisteredPanel, type PanelRegistry } from '../panels'
import { levelForPath, useDashboard, useDrill, usePanelFrame, useTranslate } from '../runtime'
import { isVisualRegression } from '../visualRegression'
import { recordForRow, resolveLeafActionURL, variablesFromLocation } from './actions'
import { DrillOverlay } from './DrillOverlay'
import {
  breadcrumbsForNavigation,
  drillTargetForLevel,
  drillTargetForNode,
  perspectivesForLevel,
  rowForNode,
  viewForSemantics,
  type DrillTarget,
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
  if (isVisualRegression() || !transitionDocument.startViewTransition || reduceMotion) {
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

export interface ExplorePanelProps {
  panel: Panel
  registry?: PanelRegistry
}

export function ExplorePanel({ panel, registry }: ExplorePanelProps) {
  const { document, navigation } = useDashboard()
  const drill = useDrill()
  const translate = useTranslate()
  const frame = usePanelFrame(panel.id)
  const active = navigation.panelId === panel.id && navigation.path.length > 0
  const level = active
    ? levelForPath(document, navigation.path)
    : (panel.drillRoot ? document.drill.edges[panel.drillRoot] : undefined)
  const perspectives = useMemo(() => perspectivesForLevel(document, level), [document, level])
  const perspective = active ? document.perspectives.find(({ id }) => id === navigation.perspectiveId) : undefined
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
  const exploreRef = useRef<HTMLButtonElement>(null)
  const instanceId = useId()
  const viewKey = `${active ? navigation.path.join('|') : panel.drillRoot ?? panel.id}:${navigation.perspectiveId ?? ''}`
  const previousView = useRef(viewKey)
  const [overlay, setOverlay] = useState<{ target: DrillTarget; anchor: ChartAnchor }>()
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
    if (previousView.current === viewKey) return
    previousView.current = viewKey
    // Entering a level closes whatever opened it and hands focus to the view.
    setOverlay(undefined)
    focusRef.current?.focus({ preventScroll: true })
  }, [viewKey])

  useEffect(() => {
    const element = focusRef.current
    const transitionDocument = globalThis.document as unknown as TransitionDocument
    if (!element || isVisualRegression() || transitionDocument.startViewTransition ||
      globalThis.window?.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
    element.classList.remove('lens-explore-level-enter')
    const animationFrame = globalThis.requestAnimationFrame(() => element.classList.add('lens-explore-level-enter'))
    return () => globalThis.cancelAnimationFrame(animationFrame)
  }, [viewKey])

  const themeOf = useCallback((element: HTMLElement | null) => {
    const root = element?.closest<HTMLElement>('.lens-root')
    return { theme: root?.dataset.theme, dark: root?.classList.contains('dark') ?? false }
  }, [])
  const [overlayTheme, setOverlayTheme] = useState<{ theme?: string; dark: boolean }>({ dark: false })

  const leafHrefFor = useCallback((node: Node, owner?: Level): string | undefined => {
    const source = owner ?? level
    if (!node.action || !source) return undefined
    const location = new URL(globalThis.location.href)
    const rows = source.frame ? document.frames[source.frame] : frame.data
    return resolveLeafActionURL(node.action, {
      fields: fieldsForNode(source, rows, node), variables: variablesFromLocation(location), location,
    })
  }, [document.frames, frame.data, level])

  const withHrefs = useCallback((rows: DrillTarget['breakdown'], owner?: Level) => (
    rows.map((row) => ({ ...row, href: leafHrefFor(row.node, owner) }))
  ), [leafHrefFor])

  const openForMark = useCallback((key: NodeKey, anchor?: ChartAnchor) => {
    if (!level) return
    const node = level.children.find((child) => child.key === key || child.key.endsWith(`/${key}`))
    if (!node) return
    const targetLevel = node.target ? document.drill.edges[node.target] : undefined
    const target = drillTargetForNode(document, level, node, frame.data, targetLevel?.frame ? document.frames[targetLevel.frame] : undefined)
    setOverlayTheme(themeOf(focusRef.current))
    setOverlay({
      target: { ...target, leafHref: leafHrefFor(node), breakdown: withHrefs(target.breakdown, targetLevel) },
      // Without a pointer position (keyboard activation) the popover anchors
      // to the panel itself.
      anchor: anchor ?? anchorFromElement(focusRef.current),
    })
  }, [document, frame.data, leafHrefFor, level, themeOf, withHrefs])

  const openForLevel = useCallback(() => {
    if (!level) return
    setOverlayTheme(themeOf(exploreRef.current))
    const target = drillTargetForLevel(document, panel, level, frame.data)
    setOverlay({
      target: { ...target, breakdown: withHrefs(target.breakdown, level) },
      anchor: anchorFromElement(exploreRef.current),
    })
  }, [document, frame.data, level, panel, themeOf, withHrefs])

  const closeOverlay = useCallback(() => {
    setOverlay(undefined)
    exploreRef.current?.focus()
  }, [])

  const drillTo = useCallback((...keys: Array<NodeKey>) => {
    setOverlay(undefined)
    runViewTransition(() => {
      // A mark's breakdown lists the children of the level that mark expands
      // to, so landing on one means entering the mark first; the reducer sees
      // each dispatch in order, so the second key resolves against the first.
      for (const key of keys) drill.drillInto(key, panel.id)
    })
  }, [drill, panel.id])

  const applyPerspective = useCallback((perspectiveId: string, target: DrillTarget) => {
    setOverlay(undefined)
    runViewTransition(() => {
      // A segment's perspectives belong to the level it expands to, so entering
      // the segment first is what makes the perspective addressable.
      if (target.node) drill.drillInto(target.node.key, panel.id)
      drill.switchPerspective(perspectiveId)
    })
  }, [drill, panel.id])

  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key !== 'Escape' || !active || !drill.canGoBack || overlay) return
    event.preventDefault()
    runViewTransition(drill.back)
  }

  // A level with no children but several perspectives is still explorable: the
  // perspective choice is the only thing the overlay would show, and hiding
  // the affordance would strand it.
  const explorable = Boolean(level?.children.length) || perspectives.length > 1
  const chrome = useMemo(() => ({
    explore: explorable ? (
      <button
        aria-haspopup="dialog"
        aria-label={translate('explore.openBreakdown', 'Show breakdown')}
        className="lens-icon-button lens-explore-affordance"
        onClick={openForLevel}
        ref={exploreRef}
        title={translate('explore.openBreakdown', 'Show breakdown')}
        type="button"
      >
        <CaretDown />
      </button>
    ) : undefined,
    trail: breadcrumbs.length > 1 ? (
      <nav
        aria-label={translate('explore.path', '{name} exploration path', { name: panel.title })}
        className="lens-panel-trail"
      >
        {drill.canGoBack && (
          <button
            aria-label={translate('explore.back', 'Back')}
            className="lens-icon-button lens-trail-back"
            onClick={() => runViewTransition(drill.back)}
            title={translate('explore.back', 'Back')}
            type="button"
          >
            <CaretLeft />
          </button>
        )}
        <ol>
          {breadcrumbs.map((crumb, index) => (
            <li key={crumb.pathIndex}>
              {index > 0 && <CaretRight />}
              <button
                aria-current={crumb.current ? 'page' : undefined}
                className="lens-trail-crumb"
                onClick={() => { if (!crumb.current) runViewTransition(() => drill.jumpTo(crumb.pathIndex)) }}
                type="button"
              >
                {crumb.label}
                {crumb.current && crumb.perspective && (
                  <span className="lens-trail-perspective">· {crumb.perspective.label}</span>
                )}
              </button>
            </li>
          ))}
        </ol>
      </nav>
    ) : undefined,
  }), [breadcrumbs, drill, explorable, openForLevel, panel.title, translate])

  let content: ReactNode
  if (!level) content = <div className="lens-placeholder-state">{translate('explore.unavailable', 'This exploration level is unavailable.')}</div>
  else content = <RegisteredPanel panel={viewPanel} registry={registry} />

  return (
    <article
      aria-label={translate('explore.panel', 'Explore {name}', { name: panel.title })}
      className="lens-explore"
      onKeyDown={onKeyDown}
    >
      <div
        className="lens-explore-level"
        data-explore-view={kind}
        ref={focusRef}
        style={transitionStyle}
        tabIndex={-1}
      >
        <PanelChromeContext.Provider value={chrome}>
          <MarkSelectionContext.Provider value={openForMark}>
            {content}
          </MarkSelectionContext.Provider>
        </PanelChromeContext.Provider>
      </div>
      {overlay && (
        <DrillOverlay
          anchor={overlay.anchor}
          dark={overlayTheme.dark}
          onClose={closeOverlay}
          onDrillChild={(childKey) => {
            const node = overlay.target.node
            if (node) drillTo(node.key, childKey)
            else drillTo(childKey)
          }}
          onDrillInto={(target) => { if (target.node) drillTo(target.node.key) }}
          onPerspective={(perspectiveId) => applyPerspective(perspectiveId, overlay.target)}
          selectedPerspectiveId={navigation.perspectiveId}
          target={overlay.target}
          theme={overlayTheme.theme}
          valueFormat={level?.encoding?.value ? viewPanel.format[level.encoding.value] : undefined}
        />
      )}
    </article>
  )
}

function anchorFromElement(element: HTMLElement | null): ChartAnchor {
  const rect = element?.getBoundingClientRect()
  if (!rect) return { x: 0, y: 0 }
  return { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 }
}
