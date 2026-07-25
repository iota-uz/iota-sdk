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
import { seriesColorResolver } from '../panels/data'
import {
  isPerspectiveFork,
  levelForPath,
  perspectivesForPosition,
  useDashboard,
  useDrawer,
  useDrill,
  usePanelFrame,
  useTranslate,
} from '../runtime'
import { isVisualRegression } from '../visualRegression'
import { recordForRow, resolveLeafActionURL, variablesFromLocation } from './actions'
import { DrillOverlay } from './DrillOverlay'
import { exploreViewForLevel, focusContextForLevel, isFocusCanvas } from './focusModel'
import { FocusContextHeader } from './FocusContextHeader'
import { LensSelector } from './LensSelector'
import {
  breadcrumbsForNavigation,
  drillTargetForLevel,
  drillTargetForNode,
  labelForNode,
  perspectivesForLevel,
  rowForNode,
  viewForSemantics,
  type DrillTarget,
} from './model'
import { SourceDataDisclosure } from './SourceDataDisclosure'

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

function fieldsForNode(
  level: Level, frame: Frame | undefined, node: Node, encoding: Encoding | undefined,
): Record<string, unknown> {
  if (!frame) return {}
  const row = rowForNode(node, level, frame, encoding)
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
  const drawer = useDrawer()
  const translate = useTranslate()
  const frame = usePanelFrame(panel.id)
  const active = navigation.panelId === panel.id && navigation.path.length > 0
  const rootLevel = panel.drillRoot ? document.drill.edges[panel.drillRoot] : undefined
  const level = active ? levelForPath(document, navigation.path) : rootLevel
  const perspectives = useMemo(() => perspectivesForLevel(document, level), [document, level])
  // The lens selector's set: at a perspective root this expands to the whole
  // branch sibling set (the choice the overlay offered on the parent segment),
  // because the builder records only the level's own perspective on it. The
  // plain `perspectives` list above keeps driving the resting-card behaviors
  // (single-perspective auto-bind, explorability) exactly as before.
  const positionPerspectives = useMemo(() => perspectivesForPosition(document, level), [document, level])
  const perspective = active ? document.perspectives.find(({ id }) => id === navigation.perspectiveId) : undefined
  const semantics = perspective?.semantics ?? panel.semantics
  // A level that declares its own visualization wins over the semantics
  // mapping; documents that never set `Level.view` render exactly as before.
  const kind = exploreViewForLevel(level) ?? viewForSemantics(semantics, panel.kind)
  // Focus-canvas chrome is opt-in per document. It changes nothing until the
  // panel is actively exploring: the resting card stays byte-identical.
  const focusCanvas = isFocusCanvas(rootLevel, level)
  const focusActive = focusCanvas && active && Boolean(level)
  // Inside a drawer the focus-canvas host is the drawer's whole subject, so it
  // wears the canvas treatment (full width, roomy body) at rest and surfaces
  // its root-level lenses immediately — the board can switch perspective before
  // drilling, instead of a lone card stranded in a corner. On the main
  // dashboard a resting focus host is unchanged (this is gated on the drawer).
  const inDrawer = drawer.depth > 0
  const drawerFocus = focusCanvas && inDrawer
  const rootLens = drawerFocus && !focusActive && positionPerspectives.length > 1
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
  const [overlay, setOverlay] = useState<{
    target: DrillTarget
    anchor: ChartAnchor
    // Element-anchored overlays keep the element so the popover can re-measure
    // it once the layout settles; pointer-anchored ones carry coordinates that
    // no reflow can invalidate.
    anchorElement?: HTMLElement | null
    // The clicked mark's series color, resolved through the same path the plot
    // and legend use; a level card describes no single mark and carries none.
    accentColor?: string
  }>()
  const transitionName = useMemo(() => {
    const identifier = `${panel.id}-${instanceId}`.replace(/[^a-zA-Z0-9_-]/g, '-')
    return `lens-explore-${identifier}`
  }, [instanceId, panel.id])
  const transitionStyle = useMemo(() => ({
    viewTransitionName: transitionName,
    viewTransitionClass: 'lens-explore-level-transition',
  }) as CSSProperties, [transitionName])
  // The focus chrome and the source disclosure each carry their own
  // view-transition-name so `runViewTransition` morphs only the chart: the
  // chrome stays put (its own group cross-fades content in place) instead of
  // being swept along with the level swap.
  const chromeTransitionStyle = useMemo(() => ({
    viewTransitionName: `${transitionName}-chrome`,
  }) as CSSProperties, [transitionName])
  const sourceTransitionStyle = useMemo(() => ({
    viewTransitionName: `${transitionName}-source`,
  }) as CSSProperties, [transitionName])
  // A focus-canvas host names its whole card, at rest and while exploring, so
  // entering/leaving the canvas FLIP-morphs the card bounds between its
  // authored grid span and the full row instead of popping. The name exists in
  // both states of the transition — that is what makes it a morph rather than
  // an exit+enter — and non-focus hosts carry no name, so nothing about their
  // transitions changes.
  const hostTransitionStyle = useMemo(() => (focusCanvas ? {
    viewTransitionName: `${transitionName}-host`,
    viewTransitionClass: 'lens-explore-host-transition',
  } as CSSProperties : undefined), [focusCanvas, transitionName])

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
      fields: fieldsForNode(source, rows, node, source.encoding ?? panel.encoding),
      variables: variablesFromLocation(location),
      location,
    })
  }, [document.frames, frame.data, level, panel.encoding])

  const withHrefs = useCallback((rows: DrillTarget['breakdown'], owner?: Level) => (
    rows.map((row) => ({ ...row, href: leafHrefFor(row.node, owner) }))
  ), [leafHrefFor])

  // Resolves the clicked mark's color exactly as ChartLegend does: the row's
  // position and label field through `seriesColorResolver`, positional pins
  // dropped once the panel is at a drill level whose rows are not its own.
  const colorForNode = useCallback((node: Node): string | undefined => {
    if (!level || !frame.data) return undefined
    const row = rowForNode(node, level, frame.data, viewPanel.encoding)
    if (!row) return undefined
    const index = frame.data.rows.indexOf(row)
    if (index < 0) return undefined
    const labelField = viewPanel.encoding.label ?? viewPanel.encoding.category
    const labelIndex = labelField ? frame.data.columns.findIndex((column) => column.name === labelField) : -1
    const raw = labelIndex >= 0 ? row[labelIndex] : undefined
    const label = typeof raw === 'string' ? raw : labelForNode(node, level, document, frame.data, viewPanel.encoding)
    return seriesColorResolver(document.theme, viewPanel, { positional: !active })(label, index)
  }, [active, document, frame.data, level, viewPanel])

  const drillTo = useCallback((...keys: Array<NodeKey>) => {
    setOverlay(undefined)
    runViewTransition(() => {
      // A mark's breakdown lists the children of the level that mark expands
      // to, so landing on one means entering the mark first; the reducer sees
      // each dispatch in order, so the second key resolves against the first.
      for (const key of keys) drill.drillInto(key, panel.id)
    })
  }, [drill, panel.id])

  const enterPerspective = useCallback((perspectiveId: string, nodeKey?: NodeKey) => {
    setOverlay(undefined)
    runViewTransition(() => {
      drill.switchPerspective(perspectiveId, nodeKey
        ? { enter: nodeKey, panelId: panel.id }
        : undefined)
    })
  }, [drill, panel.id])

  const enterFocusNode = useCallback((node: Node, targetLevel: Level | undefined): boolean => {
    if (!focusCanvas || !targetLevel) return false
    const fork = isPerspectiveFork(document, targetLevel)
    const available = perspectivesForLevel(document, targetLevel)
    const defaultPerspective = targetLevel.defaultPerspective
    if (fork && defaultPerspective && available.some(({ id }) => id === defaultPerspective)) {
      // Focus canvases are progressive-disclosure surfaces, not setup
      // dialogs. Enter the producer-selected useful view immediately; the
      // focus header keeps every sibling lens one click away.
      enterPerspective(defaultPerspective, node.key)
      return true
    }
    if (!fork || available.length <= 1) {
      drillTo(node.key)
      return true
    }
    return false
  }, [document, drillTo, enterPerspective, focusCanvas])

  const openForMark = useCallback((key: NodeKey, anchor?: ChartAnchor) => {
    if (!level) return
    const node = level.children.find((child) => child.key === key || child.key.endsWith(`/${key}`))
    if (!node) return
    const targetLevel = node.target ? document.drill.edges[node.target] : undefined
    // Focus mode: a segment with exactly one continuation drills straight into
    // it — the chrome (breadcrumb, parent context, lens selector) replaces the
    // popover's affordances. The popover keeps its job for leaves (no level to
    // enter) and forks (a real choice between perspectives).
    if (enterFocusNode(node, targetLevel)) return
    const target = drillTargetForNode(document, level, node, frame.data, targetLevel?.frame ? document.frames[targetLevel.frame] : undefined, panel)
    setOverlayTheme(themeOf(focusRef.current))
    setOverlay({
      target: { ...target, leafHref: leafHrefFor(node), breakdown: withHrefs(target.breakdown, targetLevel) },
      // The swatch must match the slice on screen, so it resolves through the
      // same path the plot and legend take — positional pins by row when the
      // panel shows its own frame, by label once it is at a drill level.
      accentColor: colorForNode(node),
      // Without a pointer position (keyboard activation) the popover anchors
      // to the panel itself.
      anchor: anchor ?? anchorFromElement(focusRef.current),
      anchorElement: anchor ? undefined : focusRef.current,
    })
  }, [colorForNode, document, enterFocusNode, frame.data, leafHrefFor, level, panel, themeOf, withHrefs])

  const openForLevel = useCallback(() => {
    if (!level) return
    setOverlayTheme(themeOf(exploreRef.current))
    const target = drillTargetForLevel(document, panel, level, frame.data)
    setOverlay({
      target: { ...target, breakdown: withHrefs(target.breakdown, level) },
      anchor: anchorFromElement(exploreRef.current),
      anchorElement: exploreRef.current,
    })
  }, [document, frame.data, level, panel, themeOf, withHrefs])

  const closeOverlay = useCallback(() => {
    setOverlay(undefined)
    exploreRef.current?.focus()
  }, [])

  const applyPerspective = useCallback((perspectiveId: string, target: DrillTarget) => {
    // A segment's perspectives belong to the level it expands to, so entering
    // the segment first is what makes the perspective addressable. That is
    // one user action, so it leaves one history entry: the perspective is
    // folded into the step that entered the segment, and Back returns to the
    // chart the segment was picked from rather than to the level in between,
    // which is a fork the user never asked to stand on.
    enterPerspective(perspectiveId, target.node?.key)
  }, [enterPerspective])

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
    // The header is the tightest space on the card (a total badge and two icon
    // buttons share it), so it carries only what stays readable: one step back
    // and the level you are on. The full path lives in the overlay the current
    // level opens — chopping every ancestor down to a letter served nobody.
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
        <button
          aria-current="page"
          aria-haspopup="dialog"
          className="lens-trail-current"
          onClick={openForLevel}
          title={breadcrumbs.map((crumb) => crumb.label).join(' › ')}
          type="button"
        >
          {breadcrumbs.at(-1)?.label}
        </button>
      </nav>
    ) : undefined,
  }), [breadcrumbs, drill, explorable, openForLevel, panel.title, translate])

  // A level with no frame of its own is a fork: its perspectives own the data.
  // Until one is chosen there is nothing truthful to draw, so the panel asks
  // for the choice instead of showing the parent level's numbers.
  const awaitingPerspective = Boolean(level && isPerspectiveFork(document, level))

  // A state that replaces the panel still needs the panel's chrome: without the
  // trail there is no way back out of the level it is reporting on.
  const stateCard = (body: ReactNode) => (
    <section aria-label={viewPanel.title} className="lens-panel">
      <header className="lens-panel-header">
        {chrome.trail ?? <h3 className="lens-panel-title">{viewPanel.title}</h3>}
        {chrome.explore}
      </header>
      <div className="lens-panel-body">{body}</div>
    </section>
  )

  let content: ReactNode
  if (!level) {
    content = stateCard(
      <div className="lens-placeholder-state">
        {translate('explore.unavailable', 'This exploration level is unavailable.')}
      </div>,
    )
  } else if (awaitingPerspective) {
    content = stateCard(
      <div className="lens-explore-awaiting">
        <p className="lens-explore-awaiting-text">
          {translate('explore.chooseView', 'Choose a view for {name}', { name: viewPanel.title })}
        </p>
        <button className="lens-explore-awaiting-action" onClick={openForLevel} type="button">
          {translate('explore.views', '{n} views', { n: perspectives.length })}
          <CaretRight />
        </button>
      </div>,
    )
  } else content = <RegisteredPanel panel={viewPanel} registry={registry} />

  // The focus header is a projection over the same document + path the chart
  // reads; a deep link whose parent frame has not loaded degrades to a
  // name-only header instead of crashing.
  const focusContext = focusActive && level
    ? focusContextForLevel(document, panel, navigation.path, level)
    : undefined
  const jumpToCrumb = useCallback((pathIndex: number) => {
    runViewTransition(() => drill.jumpTo(pathIndex))
  }, [drill])
  // Mini-chart colors resolve by label, never positionally: the parent is a
  // drill level, and its slices must keep the colors the plot drew for them.
  const miniColorFor = useCallback((label: string, index: number) => (
    seriesColorResolver(document.theme, panel, { positional: false })(label, index)
  ), [document.theme, panel])
  const switchLens = useCallback((perspectiveId: string) => {
    runViewTransition(() => drill.switchPerspective(perspectiveId))
  }, [drill])
  // A root lens is chosen before any drill has entered a level, so the switch
  // carries the panel id: the reducer resolves the position from the host's
  // drill root and enters the chosen perspective in one step.
  const switchRootLens = useCallback((perspectiveId: string) => {
    runViewTransition(() => drill.switchPerspective(perspectiveId, { panelId: panel.id }))
  }, [drill, panel.id])
  const focusParent = focusContext?.parent

  return (
    <article
      aria-label={translate('explore.panel', 'Explore {name}', { name: panel.title })}
      className={`lens-explore${focusActive ? ' lens-explore-focus' : ''}${drawerFocus ? ' lens-explore-drawer-focus' : ''}`}
      onKeyDown={onKeyDown}
      style={hostTransitionStyle}
    >
      {rootLens && (
        <div className="lens-focus-chrome lens-focus-chrome-root" style={chromeTransitionStyle}>
          <LensSelector
            activeId={navigation.perspectiveId}
            label={translate('focus.viewAs', 'View as')}
            moreLabel={translate('focus.moreViews', 'More')}
            onSelect={switchRootLens}
            perspectives={positionPerspectives}
          />
        </div>
      )}
      {focusContext && (
        <div className="lens-focus-chrome" style={chromeTransitionStyle}>
          <FocusContextHeader
            breadcrumbs={breadcrumbs}
            colorFor={miniColorFor}
            context={focusContext}
            onCrumb={jumpToCrumb}
            onParent={focusParent ? () => jumpToCrumb(focusParent.pathIndex) : undefined}
            periodLabel={document.header?.subtitle?.trim() || undefined}
            valueFormat={viewPanel.encoding.value ? viewPanel.format[viewPanel.encoding.value] : undefined}
          />
          {positionPerspectives.length > 1 && (
            <LensSelector
              activeId={navigation.perspectiveId}
              label={translate('focus.viewAs', 'View as')}
              moreLabel={translate('focus.moreViews', 'More')}
              onSelect={switchLens}
              perspectives={positionPerspectives}
            />
          )}
        </div>
      )}
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
      {focusActive && level?.source && (
        <SourceDataDisclosure key={viewKey} source={level.source} style={sourceTransitionStyle} />
      )}
      {overlay && (
        <DrillOverlay
          accentColor={overlay.accentColor}
          anchor={overlay.anchor}
          anchorElement={overlay.anchorElement}
          path={breadcrumbs.map((crumb) => ({
            label: crumb.label,
            current: crumb.current,
            onSelect: () => { closeOverlay(); runViewTransition(() => drill.jumpTo(crumb.pathIndex)) },
          }))}

          dark={overlayTheme.dark}
          onClose={closeOverlay}
          onDrillChild={(childKey) => {
            const node = overlay.target.node
            if (node) {
              drillTo(node.key, childKey)
              return
            }
            const child = level?.children.find((candidate) => (
              candidate.key === childKey || candidate.key.endsWith(`/${childKey}`)
            ))
            const targetLevel = child?.target ? document.drill.edges[child.target] : undefined
            if (child && enterFocusNode(child, targetLevel)) return
            drillTo(childKey)
          }}
          onDrillInto={(target) => {
            if (!target.node) return
            const targetLevel = target.node.target ? document.drill.edges[target.node.target] : undefined
            if (!enterFocusNode(target.node, targetLevel)) drillTo(target.node.key)
          }}
          onPerspective={(perspectiveId) => applyPerspective(perspectiveId, overlay.target)}
          selectedPerspectiveId={navigation.perspectiveId}
          target={overlay.target}
          theme={overlayTheme.theme}
          valueFormat={viewPanel.encoding.value ? viewPanel.format[viewPanel.encoding.value] : undefined}
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
