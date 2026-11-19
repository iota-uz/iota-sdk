import { createEffect, createMemo, createSignal, createUniqueId, onCleanup, Show, type JSX } from 'solid-js'
import type { ChartAnchor } from '../charts/adapter'
import type { Encoding, FieldFormat, Frame, Level, Node, NodeKey, Panel } from '../contract'
import { CaretDown, CaretLeft, CaretRight } from '../icons'
import { MarkSelectionContext, PanelChromeContext } from '../panels/context'
import { RegisteredPanel, type PanelRegistry } from '../panels/registry'
import { colorLabels, seriesColorResolver } from '../panels/data'
import { retargetPanel } from '../panels/kinds'
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
import { DrillOverlay, type DrillOverlayContext, type DrillOverlaySave } from './DrillOverlay'
import { useDrillHost, useReturnLink } from './drillHost'
import { investigationSaveability, nodeValue, placementForPanel, positionProvenance, type InvestigationSlice, type InvestigationUnsupportedReason } from './investigation'
import { originLabel, SaveInvestigation, slicePeriodLabel } from './SaveInvestigation'
import { useReportPlacementHost } from '../reports/placementHost'
import { useLocale } from '../runtime'
import { formatFieldValue } from '../runtime/format'
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

export function ExplorePanel(props: ExplorePanelProps): JSX.Element {
  const { document, navigation } = useDashboard()
  const drill = useDrill()
  const drawer = useDrawer()
  const translate = useTranslate()
  const returnLink = useReturnLink()
  const locale = useLocale()
  const drillHost = useDrillHost()
  const placementHost = useReportPlacementHost()
  const frame = usePanelFrame(props.panel.id)
  const panel = props.panel
  const active = createMemo(() => navigation.panelId === panel.id && navigation.path.length > 0)
  const level = createMemo<Level | undefined>(() => (
    active() ? levelForPath(document, navigation.path) : (panel.drillRoot ? document.drill.edges[panel.drillRoot] : undefined)
  ))
  const perspectives = createMemo(() => perspectivesForLevel(document, level()))
  // A level with no frame of its own is a fork: its perspectives own the data.
  // When the producer declares a default, landing on that fork (including via
  // a breadcrumb) should enter the useful view immediately instead of making
  // the user confirm a choice the document has already made.
  const awaitingPerspective = createMemo(() => Boolean(level() && isPerspectiveFork(document, level()!)))
  // The lens selector's set: at a perspective root this expands to the whole
  // branch sibling set (the choice the overlay offered on the parent segment),
  // because the builder records only the level's own perspective on it. The
  // plain `perspectives` list above keeps driving the resting-card behaviors
  // (single-perspective auto-bind, explorability) exactly as before.
  const positionPerspectives = createMemo(() => perspectivesForPosition(document, level()))
  const perspective = createMemo(() => (
    active() ? document.perspectives.find(({ id }) => id === navigation.perspectiveId) : undefined
  ))
  const semantics = () => perspective()?.semantics ?? panel.semantics
  // A level that declares its own visualization wins over the semantics
  // mapping; documents that never set `Level.view` render exactly as before.
  const kind = createMemo(() => exploreViewForLevel(level()) ?? viewForSemantics(semantics(), panel.kind))
  // Focus-canvas chrome is opt-in per document. It changes nothing until the
  // panel is actively exploring: the resting card stays byte-identical.
  const rootLevel = createMemo(() => (panel.drillRoot ? document.drill.edges[panel.drillRoot] : undefined))
  const focusCanvas = createMemo(() => isFocusCanvas(rootLevel(), level()))
  const focusActive = createMemo(() => focusCanvas() && active() && Boolean(level()))
  // Inside a drawer the focus-canvas host is the drawer's whole subject, so it
  // wears the canvas treatment (full width, roomy body) at rest and surfaces
  // its root-level lenses immediately — the board can switch perspective before
  // drilling, instead of a lone card stranded in a corner. On the main
  // dashboard a resting focus host is unchanged (this is gated on the drawer).
  const inDrawer = drawer.depth > 0
  const drawerFocus = createMemo(() => focusCanvas() && inDrawer)
  const rootLens = createMemo(() => drawerFocus() && !focusActive() && positionPerspectives().length > 1)
  const viewPanel = createMemo<Panel>(() => {
    const current = level()
    const encoding = current?.encoding ?? panel.encoding
    const view = retargetPanel(panel, kind())
    // A table level brings its own columns, formats and row action: the host
    // panel is a chart and has none of them to retarget.
    const table = kind() === 'table' ? current?.table : undefined
    return {
      ...view,
      title: current?.label.trim() || panel.title,
      semantics: semantics(),
      encoding,
      // A table level formats its own fields: the host chart's formats belong
      // to the host's columns and would leak onto same-named table fields.
      format: table ? { ...table.format } : formatsForEncoding(panel, encoding),
      ...(table ? { columns: table.columns, actions: table.actions } : {}),
    } as Panel
  })
  // The header figure is the selected mark's value from the host frame, in the
  // host's units: a table level without a format of its own for that field
  // must not print it raw.
  const headerValueFormat = (): FieldFormat | undefined => {
    const field = viewPanel().encoding.value
    if (!field) return undefined
    return viewPanel().format[field] ?? formatsForEncoding(panel, viewPanel().encoding)[field]
  }
  const breadcrumbs = createMemo(() => breadcrumbsForNavigation(document, panel, navigation))
  let focusRef: HTMLDivElement | undefined
  let exploreRef: HTMLButtonElement | undefined
  const instanceId = createUniqueId()
  const viewKey = createMemo(() => (
    `${active() ? navigation.path.join('|') : panel.drillRoot ?? panel.id}:${navigation.perspectiveId ?? ''}`
  ))
  let previousView = viewKey()
  const [overlay, setOverlay] = createSignal<{
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
  const transitionName = `lens-explore-${`${panel.id}-${instanceId}`.replace(/[^a-zA-Z0-9_-]/g, '-')}`
  const transitionStyle = {
    'view-transition-name': transitionName,
    'view-transition-class': 'lens-explore-level-transition',
  } as JSX.CSSProperties
  // The focus chrome and the source disclosure each carry their own
  // view-transition-name so `runViewTransition` morphs only the chart: the
  // chrome stays put (its own group cross-fades content in place) instead of
  // being swept along with the level swap.
  const chromeTransitionStyle = {
    'view-transition-name': `${transitionName}-chrome`,
  } as JSX.CSSProperties
  const sourceTransitionStyle = {
    'view-transition-name': `${transitionName}-source`,
  } as JSX.CSSProperties
  // A focus-canvas host names its whole card, at rest and while exploring, so
  // entering/leaving the canvas FLIP-morphs the card bounds between its
  // authored grid span and the full row instead of popping. The name exists in
  // both states of the transition — that is what makes it a morph rather than
  // an exit+enter — and non-focus hosts carry no name, so nothing about their
  // transitions changes.
  const hostTransitionStyle: JSX.CSSProperties | undefined = focusCanvas() ? {
    'view-transition-name': `${transitionName}-host`,
    'view-transition-class': 'lens-explore-host-transition',
  } : undefined

  createEffect(() => {
    if (!active()) return
    if (awaitingPerspective()) {
      const defaultPerspective = level()?.defaultPerspective
      if (defaultPerspective && perspectives().some(({ id }) => id === defaultPerspective)) {
        runViewTransition(() => drill.switchPerspective(defaultPerspective, { replace: true }))
        return
      }
    }
    if (perspectives().length !== 1 || perspectives()[0]?.id === navigation.perspectiveId) return
    runViewTransition(() => drill.switchPerspective(perspectives()[0]!.id, { replace: true }))
  })

  createEffect(() => {
    const key = viewKey()
    if (previousView === key) return
    previousView = key
    // Entering a level closes whatever opened it and hands focus to the view.
    setOverlay(undefined)
    focusRef?.focus({ preventScroll: true })
  })

  createEffect(() => {
    void viewKey()
    const element = focusRef
    const transitionDocument = globalThis.document as unknown as TransitionDocument
    if (!element || isVisualRegression() || transitionDocument.startViewTransition ||
      globalThis.window?.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
    element.classList.remove('lens-explore-level-enter')
    const animationFrame = globalThis.requestAnimationFrame(() => element.classList.add('lens-explore-level-enter'))
    onCleanup(() => globalThis.cancelAnimationFrame(animationFrame))
  })

  const themeOf = (element: HTMLElement | null) => {
    const root = element?.closest<HTMLElement>('.lens-root')
    return { theme: root?.dataset.theme, dark: root?.classList.contains('dark') ?? false }
  }
  const [overlayTheme, setOverlayTheme] = createSignal<{ theme?: string; dark: boolean }>({ dark: false })

  const leafHrefFor = (node: Node, owner?: Level): string | undefined => {
    const source = owner ?? level()
    if (!node.action || !source) return undefined
    const location = new URL(globalThis.location.href)
    const rows = source.frame ? document.frames[source.frame] : frame.data
    const href = resolveLeafActionURL(node.action, {
      fields: fieldsForNode(source, rows, node, source.encoding ?? panel.encoding),
      variables: variablesFromLocation(location),
      location,
    })
    return node.action.kind === 'navigate_to_leaf' ? returnLink(href) : href
  }

  const withHrefs = (rows: DrillTarget['breakdown'], owner?: Level) => (
    rows.map((row) => ({ ...row, href: leafHrefFor(row.node, owner) }))
  )

  // Resolves the clicked mark's color exactly as ChartLegend does: the row's
  // position and label field through `seriesColorResolver`, positional pins
  // dropped once the panel is at a drill level whose rows are not its own.
  const colorForNode = (node: Node): string | undefined => {
    if (!level() || !frame.data) return undefined
    const row = rowForNode(node, level()!, frame.data, viewPanel().encoding)
    if (!row) return undefined
    const index = frame.data.rows.indexOf(row)
    if (index < 0) return undefined
    const labelField = viewPanel().encoding.label ?? viewPanel().encoding.category
    const labelIndex = labelField ? frame.data.columns.findIndex((column) => column.name === labelField) : -1
    const raw = labelIndex >= 0 ? row[labelIndex] : undefined
    const label = typeof raw === 'string' ? raw : labelForNode(node, level()!, document, frame.data, viewPanel().encoding)
    return seriesColorResolver(document.theme, viewPanel(), {
      positional: !active(), labels: colorLabels(frame.data, viewPanel()),
    })(label, index)
  }

  const drillTo = (...keys: Array<NodeKey>) => {
    setOverlay(undefined)
    runViewTransition(() => {
      // A mark's breakdown lists the children of the level that mark expands
      // to, so landing on one means entering the mark first; the reducer sees
      // each dispatch in order, so the second key resolves against the first.
      for (const key of keys) drill.drillInto(key, panel.id)
    })
  }

  const enterPerspective = (perspectiveId: string, nodeKey?: NodeKey) => {
    setOverlay(undefined)
    runViewTransition(() => {
      drill.switchPerspective(perspectiveId, nodeKey
        ? { enter: nodeKey, panelId: panel.id }
        : undefined)
    })
  }

  const enterFocusNode = (node: Node, targetLevel: Level | undefined): boolean => {
    if (!focusCanvas() || !targetLevel) return false
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
  }

  const openForMark = (key: NodeKey, anchor?: ChartAnchor) => {
    const current = level()
    if (!current) return
    const node = current.children.find((child) => child.key === key || child.key.endsWith(`/${key}`))
    if (!node) return
    const targetLevel = node.target ? document.drill.edges[node.target] : undefined
    // Focus mode: a segment with exactly one continuation drills straight into
    // it — the chrome (breadcrumb, parent context, lens selector) replaces the
    // popover's affordances. The popover keeps its job for leaves (no level to
    // enter) and forks (a real choice between perspectives).
    if (enterFocusNode(node, targetLevel)) return
    rememberOpener()
    const target = drillTargetForNode(document, current, node, frame.data, targetLevel?.frame ? document.frames[targetLevel.frame] : undefined, panel)
    setOverlayTheme(themeOf(focusRef ?? null))
    setOverlay({
      target: withoutShares({ ...target, leafHref: leafHrefFor(node), breakdown: withHrefs(target.breakdown, targetLevel) }),
      // The swatch must match the slice on screen, so it resolves through the
      // same path the plot and legend take — positional pins by row when the
      // panel shows its own frame, by label once it is at a drill level.
      accentColor: colorForNode(node),
      // Without a pointer position (keyboard activation) the popover anchors
      // to the panel itself.
      anchor: anchor ?? anchorFromElement(focusRef ?? null),
      anchorElement: anchor ? undefined : focusRef,
    })
  }

  const openForLevel = () => {
    const current = level()
    if (!current) return
    setOverlayTheme(themeOf(exploreRef ?? null))
    rememberOpener()
    const target = drillTargetForLevel(document, panel, current, frame.data)
    setOverlay({
      target: withoutShares({ ...target, breakdown: withHrefs(target.breakdown, current) }),
      anchor: anchorFromElement(exploreRef ?? null),
      anchorElement: exploreRef,
    })
  }

  // The element that opened the overlay gets focus back when it closes: a
  // keyboard reader who opened a mark's card lands on that mark again, not
  // on the panel header.
  let overlayOpener: HTMLElement | undefined
  const rememberOpener = () => {
    const active = globalThis.document?.activeElement
    overlayOpener = active instanceof HTMLElement && active !== globalThis.document.body ? active : undefined
  }
  const closeOverlay = () => {
    setOverlay(undefined)
    const opener = overlayOpener
    overlayOpener = undefined
    if (opener?.isConnected) opener.focus()
    else (exploreRef ?? focusRef)?.focus()
  }

  // A ratio (declared, or a percent-formatted value) is not additive: its
  // share of a summed total would be a number nobody computed.
  const ratioValues = () => Boolean(panel.ratio) || headerValueFormat()?.kind === 'percent'
  function withoutShares(target: DrillTarget): DrillTarget {
    if (!ratioValues()) return target
    return {
      ...target, share: undefined, total: undefined,
      breakdown: target.breakdown.map((row) => ({ ...row, share: undefined })),
    }
  }

  // Labels of selections on ancestor levels: the producer's label, the
  // owning frame's label column when it is inline, else the raw value.
  const selectionLabel = (node: Node, owner: Level) => {
    if (node.label.trim()) return node.label
    const rows = owner.frame ? document.frames[owner.frame] : undefined
    const encoding = owner.encoding ?? panel.encoding
    const row = rows ? rowForNode(node, owner, rows, encoding) : undefined
    const index = row && encoding.label ? rows!.columns.findIndex(({ name }) => name === encoding.label) : -1
    const label = index >= 0 ? row![index] : undefined
    return typeof label === 'string' && label.trim() ? label : nodeValue(node)
  }
  const dimensionLabel = (dimension: string) => document.contextEditor?.dimensionLabels?.[dimension] || dimension
  const originNames = () => document.contextEditor?.originLabels
  const formatRatioPart = (field: string, value: unknown) => {
    const format = panel.format[field]
    return value === undefined || value === null ? undefined : formatFieldValue(value, format, locale())
  }

  const overlayContext = (target: DrillTarget): DrillOverlayContext => {
    const provenance = positionProvenance(document, panel, active() ? navigation : { path: [] }, selectionLabel)
    const context = provenance.context
    const subtitle = document.header?.subtitle?.trim()
    const period = context
      ? { label: slicePeriodLabel(context.period, translate), origin: originLabel(context.period.origin, translate, originNames()) }
      : subtitle ? { label: subtitle } : undefined
    const row = target.node && level() ? rowForNode(target.node, level()!, frame.data, viewPanel().encoding) : undefined
    const cell = (field: string) => {
      const index = frame.data?.columns.findIndex(({ name }) => name === field) ?? -1
      return row && index >= 0 ? row[index] : undefined
    }
    const ratio = panel.ratio
    return {
      measure: panel.title,
      ...(period ? { period } : {}),
      constraints: provenance.constraints.map((constraint, index) => ({
        key: `${constraint.dimension}:${index}`,
        label: `${dimensionLabel(constraint.dimension)}: ${constraint.labels.join(', ')}`,
        origin: originLabel(constraint.origin, translate, originNames()),
      })),
      ...(ratio ? {
        ratio: {
          numerator: { label: ratio.numerator.label, value: formatRatioPart(ratio.numerator.field, cell(ratio.numerator.field)) },
          denominator: { label: ratio.denominator.label, value: formatRatioPart(ratio.denominator.field, cell(ratio.denominator.field)) },
          ...(ratio.description ? { description: ratio.description } : {}),
        },
      } : {}),
    }
  }

  const unsupportedCopy = (reason: InvestigationUnsupportedReason, detail?: string): string => {
    switch (reason) {
      case 'not-a-report': return translate('explore.save.notReport', 'Only an exploration of a saved report can be saved.')
      case 'source-cannot-save': return translate('explore.save.sourceCannot', 'This report’s data source does not support saving explorations.')
      case 'context-blocked': return translate('explore.save.contextBlocked', 'The current filters conflict or are not supported by this report, so the exploration cannot be saved.')
      case 'level-undeclared': return translate('explore.save.levelUndeclared', 'The level “{level}” cannot be carried into a saved report.', { level: detail ?? '' })
      case 'dimension-unsupported': return translate('explore.save.dimensionUnsupported', 'The report cannot be saved with a filter on “{dimension}”.', { dimension: dimensionLabel(detail ?? '') })
      case 'drill-conflict': return translate('explore.save.drillConflict', 'This selection contradicts a filter already applied to “{dimension}”.', { dimension: dimensionLabel(detail ?? '') })
    }
  }
  const [saving, setSaving] = createSignal<{ slice: InvestigationSlice; opener?: HTMLElement }>()
  // Saving is about the position the reader stands on, so it is offered on
  // the card that describes the current level, not on a single mark.
  const overlaySave = (target: DrillTarget): DrillOverlaySave | undefined => {
    const host = drillHost.investigation
    if (!host || target.node || !active()) return undefined
    const group = placementForPanel(document, panel.id)
    const capabilities = group ? host.capabilities?.(group.id) ?? placementHost?.capabilities?.(group.id) : undefined
    const result = investigationSaveability({ document, panel, navigation, capabilities, labelFor: selectionLabel })
    if (!result.saveable) return { kind: 'unsupported', message: unsupportedCopy(result.reason, result.detail) }
    return {
      kind: 'available',
      onSave: () => {
        const opener = overlayOpener
        overlayOpener = undefined
        setOverlay(undefined)
        setSaving({ slice: result.slice, opener })
      },
    }
  }

  const applyPerspective = (perspectiveId: string, target: DrillTarget) => {
    // A segment's perspectives belong to the level it expands to, so entering
    // the segment first is what makes the perspective addressable. That is
    // one user action, so it leaves one history entry: the perspective is
    // folded into the step that entered the segment, and Back returns to the
    // chart the segment was picked from rather than to the level in between,
    // which is a fork the user never asked to stand on.
    enterPerspective(perspectiveId, target.node?.key)
  }

  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'Escape' || !active() || !drill.canGoBack || overlay()) return
    event.preventDefault()
    runViewTransition(drill.back)
  }

  // A level with no children but several perspectives is still explorable: the
  // perspective choice is the only thing the overlay would show, and hiding
  // the affordance would strand it.
  const explorable = createMemo(() => Boolean(level()?.children.length) || perspectives().length > 1)
  const chrome = {
    get explore() {
      return explorable() ? (
        <button
          aria-haspopup="dialog"
          aria-label={translate('explore.openBreakdown', 'Show breakdown')}
          class="lens-icon-button lens-explore-affordance"
          onClick={openForLevel}
          ref={exploreRef}
          title={translate('explore.openBreakdown', 'Show breakdown')}
          type="button"
        >
          <CaretDown />
        </button>
      ) : undefined
    },
    // The header is the tightest space on the card (a total badge and two icon
    // buttons share it), so it carries only what stays readable: one step back
    // and the level you are on. The full path lives in the overlay the current
    // level opens — chopping every ancestor down to a letter served nobody.
    get trail() {
      return breadcrumbs().length > 1 ? (
        <nav
          aria-label={translate('explore.path', '{name} exploration path', { name: panel.title })}
          class="lens-panel-trail"
        >
          <Show when={drill.canGoBack}>
            <button
              aria-label={translate('explore.back', 'Back')}
              class="lens-icon-button lens-trail-back"
              onClick={() => runViewTransition(drill.back)}
              title={translate('explore.back', 'Back')}
              type="button"
            >
              <CaretLeft />
            </button>
          </Show>
          <button
            aria-current="page"
            aria-haspopup="dialog"
            class="lens-trail-current"
            onClick={openForLevel}
            title={breadcrumbs().map((crumb) => crumb.label).join(' › ')}
            type="button"
          >
            {breadcrumbs().at(-1)?.label}
          </button>
        </nav>
      ) : undefined
    },
  }

  const focusContext = createMemo(() => (
    focusActive() && level()
      ? focusContextForLevel(document, panel, [...navigation.path], level()!)
      : undefined
  ))
  const jumpToCrumb = (pathIndex: number) => {
    runViewTransition(() => drill.jumpTo(pathIndex))
  }
  // Mini-chart colors resolve by label, never positionally: the parent is a
  // drill level, and its slices must keep the colors the plot drew for them.
  const miniColorFor = (label: string, index: number) => (
    seriesColorResolver(document.theme, panel, {
      positional: false, labels: colorLabels(frame.data, panel),
    })(label, index)
  )
  const switchLens = (perspectiveId: string) => {
    runViewTransition(() => drill.switchPerspective(perspectiveId))
  }
  // A root lens is chosen before any drill has entered a level, so the switch
  // carries the panel id: the reducer resolves the position from the host's
  // drill root and enters the chosen perspective in one step.
  const switchRootLens = (perspectiveId: string) => {
    runViewTransition(() => drill.switchPerspective(perspectiveId, { panelId: panel.id }))
  }
  const focusParent = () => focusContext()?.parent

  return (
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- shortcuts are delegated from focusable panel descendants; the article is not another focus stop.
    <article
      aria-label={translate('explore.panel', 'Explore {name}', { name: panel.title })}
      class={`lens-explore${focusActive() ? ' lens-explore-focus' : ''}${drawerFocus() ? ' lens-explore-drawer-focus' : ''}`}
      onKeyDown={onKeyDown}
      style={hostTransitionStyle}
    >
      <Show when={rootLens()}>
        <div class="lens-focus-chrome lens-focus-chrome-root" style={chromeTransitionStyle}>
          <LensSelector
            activeId={navigation.perspectiveId}
            label={translate('focus.viewAs', 'View as')}
            moreLabel={translate('focus.moreViews', 'More')}
            onSelect={switchRootLens}
            perspectives={positionPerspectives()}
          />
        </div>
      </Show>
      <Show when={focusContext()}>
        {(context) => (
          <div class="lens-focus-chrome" style={chromeTransitionStyle}>
            <FocusContextHeader
              breadcrumbs={breadcrumbs()}
              colorFor={miniColorFor}
              // A host-rendered view draws no Lens mini chart of its parent:
              // that would be a lookalike of a view the host owns.
              context={panel.renderer ? { ...context(), parent: undefined } : context()}
              onCrumb={jumpToCrumb}
              onParent={focusParent() ? () => jumpToCrumb(focusParent()!.pathIndex) : undefined}
              periodLabel={document.header?.subtitle?.trim() || undefined}
              valueFormat={headerValueFormat()}
            />
            <Show when={positionPerspectives().length > 1}>
              <LensSelector
                activeId={navigation.perspectiveId}
                label={translate('focus.viewAs', 'View as')}
                moreLabel={translate('focus.moreViews', 'More')}
                onSelect={switchLens}
                perspectives={positionPerspectives()}
              />
            </Show>
          </div>
        )}
      </Show>
      <div
        class="lens-explore-level"
        data-explore-view={kind()}
        ref={focusRef}
        style={transitionStyle}
        tabIndex={-1}
      >
        <PanelChromeContext.Provider value={chrome}>
          <MarkSelectionContext.Provider value={openForMark}>
            <Show
              when={level()}
              fallback={(
                <section aria-label={viewPanel().title} class="lens-panel">
                  <header class="lens-panel-header">
                    {chrome.trail ?? <h3 class="lens-panel-title">{viewPanel().title}</h3>}
                    {chrome.explore}
                  </header>
                  <div class="lens-panel-body">
                    <div class="lens-placeholder-state">
                      {translate('explore.unavailable', 'This exploration level is unavailable.')}
                    </div>
                  </div>
                </section>
              )}
            >
              {(current) => (
                <Show
                  when={!awaitingPerspective()}
                  fallback={(
                    <section aria-label={viewPanel().title} class="lens-panel">
                      <header class="lens-panel-header">
                        {chrome.trail ?? <h3 class="lens-panel-title">{viewPanel().title}</h3>}
                        {chrome.explore}
                      </header>
                      <div class="lens-panel-body">
                        <div class="lens-explore-awaiting">
                          <p class="lens-explore-awaiting-text">
                            {translate('explore.chooseView', 'Choose a view for {name}', { name: viewPanel().title })}
                          </p>
                          <button class="lens-explore-awaiting-action" onClick={openForLevel} type="button">
                            {translate('explore.views', '{n} views', { n: perspectives().length })}
                            <CaretRight />
                          </button>
                        </div>
                      </div>
                    </section>
                  )}
                >
                  {(() => { void current; return <RegisteredPanel panel={viewPanel()} registry={props.registry} /> })()}
                </Show>
              )}
            </Show>
          </MarkSelectionContext.Provider>
        </PanelChromeContext.Provider>
      </div>
      <Show
        keyed
        when={focusActive() && level()?.source
          ? { key: viewKey(), source: level()!.source! }
          : undefined}
      >
        {(entry) => <SourceDataDisclosure source={entry.source} style={sourceTransitionStyle} />}
      </Show>
      <Show when={overlay()}>
        {(value) => (
          <DrillOverlay
            accentColor={value().accentColor}
            anchor={value().anchor}
            context={overlayContext(value().target)}
            save={overlaySave(value().target)}
            anchorElement={value().anchorElement}
            path={breadcrumbs().map((crumb) => ({
              label: crumb.label,
              current: crumb.current,
              onSelect: () => { closeOverlay(); runViewTransition(() => drill.jumpTo(crumb.pathIndex)) },
            }))}
            dark={overlayTheme().dark}
            onClose={closeOverlay}
            onDrillChild={(childKey) => {
              const node = value().target.node
              if (node) {
                drillTo(node.key, childKey)
                return
              }
              const child = level()?.children.find((candidate) => (
                candidate.key === childKey || candidate.key.endsWith(`/${childKey}`)
              ))
              const targetLevel = child?.target ? document.drill.edges[child.target] : undefined
              if (child && enterFocusNode(child, targetLevel)) return
              drillTo(childKey)
            }}
            onPrefetchChild={(childKey) => {
              const node = value().target.node
              return drill.prefetch(node ? [node.key, childKey] : childKey, panel.id)
            }}
            onDrillInto={(target) => {
              if (!target.node) return
              const targetLevel = target.node.target ? document.drill.edges[target.node.target] : undefined
              if (!enterFocusNode(target.node, targetLevel)) drillTo(target.node.key)
            }}
            onPerspective={(perspectiveId) => applyPerspective(perspectiveId, value().target)}
            selectedPerspectiveId={navigation.perspectiveId}
            target={value().target}
            theme={overlayTheme().theme}
            valueFormat={headerValueFormat()}
          />
        )}
      </Show>
      <Show when={saving()}>
        {(entry) => (
          <SaveInvestigation
            dashboardId={document.meta.dashboardId}
            dimensionLabels={document.contextEditor?.dimensionLabels}
            host={drillHost.investigation!}
            onClose={() => setSaving(undefined)}
            originNames={originNames()}
            restoreFocus={entry().opener?.isConnected ? entry().opener : exploreRef ?? undefined}
            slice={entry().slice}
            theme={overlayTheme().theme}
          />
        )}
      </Show>
    </article>
  )
}

function anchorFromElement(element: HTMLElement | null): ChartAnchor {
  const rect = element?.getBoundingClientRect()
  if (!rect) return { x: 0, y: 0 }
  return { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 }
}
