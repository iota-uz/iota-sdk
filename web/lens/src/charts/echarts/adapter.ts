import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { DataZoomComponent, GraphicComponent, GridComponent, TooltipComponent } from 'echarts/components'
import { init, registerMap, use as registerEChartsModules, type ECharts, type EChartsCoreOption } from 'echarts/core'
import { LabelLayout, UniversalTransition } from 'echarts/features'
import { CanvasRenderer } from 'echarts/renderers'
import type { ChartAdapter, ChartAnchor, ChartEvents, ChartInput, ChartInstance } from '../adapter'
import { nodeKeyFromEvent } from './events'
import { buildChartOption } from './options'
import { buildEChartsTheme } from './theme'

registerEChartsModules([
  BarChart, LineChart, PieChart,
  DataZoomComponent, GraphicComponent, GridComponent, TooltipComponent,
  CanvasRenderer, LabelLayout, UniversalTransition,
])

const optionalKinds = new Set<ChartInput['kind']>(['boxplot', 'heatmap', 'map'])
let optionalModules: Promise<void> | undefined

/** Loads distribution/map renderers only when a document actually uses one. */
export async function prepareEChartsKind(kind: ChartInput['kind']): Promise<void> {
  if (!optionalKinds.has(kind)) return
  optionalModules ??= Promise.all([
    import('echarts/lib/chart/boxplot/install.js'),
    import('echarts/lib/chart/heatmap/install.js'),
    import('echarts/lib/chart/map/install.js'),
    import('echarts/lib/component/visualMap/install.js'),
  ]).then(([boxplot, heatmap, map, visualMap]) => {
    registerEChartsModules([
      boxplot.install, heatmap.install, map.install, visualMap.install,
    ])
  })
  await optionalModules
}

type ChartInitializer = (element: HTMLElement) => ECharts

/**
 * True when the only thing that changed between two inputs is the selected
 * mark. Everything else — the frame, the encoding, the theme, the formatters —
 * is referentially stable across a selection click (ChartPanel keeps those the
 * same and only bumps `selectedKey`), so reference equality is exact here.
 *
 * A selection-only change must restyle the chosen mark in place: it must not
 * replace the series, which would tear the marks down and re-run the entrance
 * animation — the visible "reload" a mere outline should never cost.
 */
function isSelectionOnlyChange(previous: ChartInput, next: ChartInput): boolean {
  return previous !== next
    && previous.selectedKey !== next.selectedKey
    && previous.frame === next.frame
    && previous.encoding === next.encoding
    && previous.theme === next.theme
    && previous.kind === next.kind
    && previous.presentation === next.presentation
    && previous.radial === next.radial
    && previous.map === next.map
    && previous.format === next.format
    && previous.formatAxis === next.formatAxis
}

/**
 * The mark lives on a canvas, so the only way to anchor UI to it is the
 * pointer position ECharts forwards from the native event.
 */
function anchorFromEvent(event: unknown): ChartAnchor | undefined {
  const wrapper = (event as { event?: { event?: MouseEvent } } | undefined)?.event?.event
  if (!wrapper || typeof wrapper.clientX !== 'number' || typeof wrapper.clientY !== 'number') return undefined
  return { x: wrapper.clientX, y: wrapper.clientY }
}

function activationFromEvent(event: unknown) {
  const wrapper = (event as { event?: { event?: MouseEvent } } | undefined)?.event?.event
  return { newTab: Boolean(wrapper?.metaKey || wrapper?.ctrlKey) }
}

function observeTheme(element: HTMLElement, rebuild: () => void): MutationObserver | undefined {
  if (typeof MutationObserver === 'undefined') return undefined
  const root = element.closest<HTMLElement>('.lens-root') ?? element
  const observer = new MutationObserver(rebuild)
  observer.observe(root, { attributes: true, attributeFilter: ['class', 'data-theme', 'style'] })
  return observer
}

interface Box {
  width: number
  height: number
}

/**
 * Every scrollable ancestor of the chart, plus the window. The dashboard
 * scrolls an inner container rather than the document — `body` is
 * `overflow: hidden` — so a listener on the window alone never learns that the
 * chart moved out from under the pointer.
 */
function scrollSources(element: HTMLElement): EventTarget[] {
  const sources: EventTarget[] = []
  for (let node = element.parentElement; node; node = node.parentElement) {
    const { overflowY, overflowX } = getComputedStyle(node)
    if (/(auto|scroll|overlay)/.test(`${overflowY} ${overflowX}`)) sources.push(node)
  }
  if (typeof window !== 'undefined') sources.push(window)
  return sources
}

/**
 * The box the chart should occupy, read from the ResizeObserver entry when the
 * browser supplies one (the authoritative content box) and falling back to the
 * element's own client box otherwise.
 */
function readBox(element: HTMLElement, entries: ReadonlyArray<ResizeObserverEntry>): Box {
  const rect = entries[0]?.contentRect
  return rect
    ? { width: rect.width, height: rect.height }
    : { width: element.clientWidth, height: element.clientHeight }
}

function observeSize(
  element: HTMLElement,
  onResize: (entries: ReadonlyArray<ResizeObserverEntry>) => void,
): ResizeObserver | undefined {
  if (typeof ResizeObserver === 'undefined') return undefined
  const observer = new ResizeObserver((entries) => onResize(entries))
  observer.observe(element)
  return observer
}

export function createEChartsAdapter(initialize: ChartInitializer = init): ChartAdapter {
  return {
    mount(element: HTMLElement, initialInput: ChartInput, events: ChartEvents): ChartInstance {
      const chart = initialize(element)
      let input = initialInput
      let hasRenderedOption = false
      let disposed = false
      let renderRevision = 0
      let readinessFrame: number | undefined

      const responsiveInput = (): ChartInput => ({ ...input, viewportWidth: element.clientWidth })

      // `setOption` and `resize` only enqueue work in ECharts. In particular,
      // ResizeObserver can fire after the first option has been accepted and
      // move the final line endpoint by a subpixel on the following frame. A
      // synchronous "ready" marker therefore described submitted work, not a
      // settled canvas. ECharts' `finished` event is its public signal that the
      // current render/animation queue has drained; keep readiness tied to that
      // signal for the initial draw and every subsequent render or resize. Two
      // paint frames then give ResizeObserver its specified post-layout turn;
      // any resize invalidates the revision and starts settlement again.
      const beginRender = () => {
        renderRevision += 1
        if (readinessFrame !== undefined) cancelAnimationFrame(readinessFrame)
        readinessFrame = undefined
        if (hasRenderedOption) delete element.dataset.chartReady
      }
      const finishRender = () => {
        if (disposed || !hasRenderedOption) return
        const revision = renderRevision
        if (readinessFrame !== undefined) cancelAnimationFrame(readinessFrame)
        readinessFrame = requestAnimationFrame(() => {
          readinessFrame = requestAnimationFrame(() => {
            readinessFrame = undefined
            if (!disposed && revision === renderRevision) element.dataset.chartReady = 'true'
          })
        })
      }
      chart.on('finished', finishRender)

      const renderReady = () => {
        if (disposed) return
        if (input.kind === 'map' && input.map) registerMap(input.map.name, input.map.geoJSON as unknown as Parameters<typeof registerMap>[1])
        const theme = buildEChartsTheme(element, input.theme)
        const option: EChartsCoreOption = buildChartOption(responsiveInput(), theme)
        hasRenderedOption = true
        beginRender()
        chart.setOption(option, { notMerge: false, replaceMerge: ['series', 'xAxis', 'yAxis'] })
      }
      const render = () => {
        renderReady()
      }
      // Selection restyle: merge the rebuilt option in place with animation
      // forced off, so the outline appears instantly without replacing the
      // series or re-running the entrance transition.
      const restyleSelection = () => {
        const theme = buildEChartsTheme(element, input.theme)
        const option = buildChartOption(responsiveInput(), theme) as EChartsCoreOption & { animation?: boolean }
        option.animation = false
        beginRender()
        chart.setOption(option, { notMerge: false })
      }
      const select = (event: Parameters<typeof nodeKeyFromEvent>[0]) => {
        const key = nodeKeyFromEvent(event)
        if (key !== undefined) events.onSelect(key, anchorFromEvent(event), activationFromEvent(event))
      }
      const hover = (event: Parameters<typeof nodeKeyFromEvent>[0]) => {
        const key = nodeKeyFromEvent(event)
        if (key !== undefined) events.onHover(key)
      }

      let axisTooltip: HTMLDivElement | undefined
      const hideAxisTooltip = () => {
        axisTooltip?.remove()
        axisTooltip = undefined
      }
      const showAxisTooltip = (event: unknown) => {
        const record = event && typeof event === 'object' ? event as Record<string, unknown> : {}
        if (record.componentType !== 'xAxis' && record.componentType !== 'yAxis') {
          hideAxisTooltip()
          return
        }
        const raw = record.value
        if (raw === null || raw === undefined || !input.encoding.category && !input.encoding.label) return
        const wrapper = (record.event as { event?: MouseEvent } | undefined)?.event
        if (!wrapper) return
        const categoryField = input.encoding.category ?? input.encoding.label ?? ''
        const label = input.format(categoryField, raw)
        hideAxisTooltip()
        axisTooltip = document.createElement('div')
        axisTooltip.className = 'lens-axis-label-tooltip'
        axisTooltip.setAttribute('role', 'tooltip')
        axisTooltip.textContent = label
        axisTooltip.style.left = `${Math.min(wrapper.clientX + 12, window.innerWidth - 280)}px`
        axisTooltip.style.top = `${Math.max(8, wrapper.clientY - 36)}px`
        document.body.append(axisTooltip)
      }

      chart.on('click', select)
      chart.on('mouseover', (event) => {
        showAxisTooltip(event)
        hover(event)
      })
      chart.on('mouseout', () => {
        hideAxisTooltip()
        events.onHover(null)
      })
      chart.on('globalout', () => {
        hideAxisTooltip()
        events.onHover(null)
      })

      // The tooltip lives on `body`, and ECharts only ever hides it from
      // pointer events it receives over the canvas. Scrolling, switching tabs,
      // or leaving the panel without crossing the canvas edge therefore strands
      // a visible tooltip at a position that no longer means anything — and the
      // next hover strands another one beside it. Each of these is a moment the
      // tooltip has stopped describing what is under the pointer, so each one
      // dismisses it.
      const hideTooltip = () => {
        chart.dispatchAction({ type: 'hideTip' })
      }
      const scrollTargets = scrollSources(element)
      const detach: Array<() => void> = []
      const listen = (target: EventTarget, type: string, options?: AddEventListenerOptions) => {
        target.addEventListener(type, hideTooltip, options)
        detach.push(() => target.removeEventListener(type, hideTooltip, options))
      }
      listen(element, 'mouseleave')
      for (const target of scrollTargets) listen(target, 'scroll', { passive: true })
      if (typeof window !== 'undefined') listen(window, 'blur')
      if (typeof document !== 'undefined') listen(document, 'visibilitychange')

      // Resize is driven off an explicit box, not `chart.resize()`'s implicit
      // re-measurement, and guarded against the canvas feeding its own height
      // back into the observed element. The container sits in an auto-sized grid
      // row, so the chart's own rendered height is part of what the ResizeObserver
      // measures: sizing the canvas taller grows the row, which the observer
      // reports as a taller box, which would grow the canvas again — an unbounded
      // loop that shows up as pies inflating while the sidebar animates its width.
      // A height increase with an unchanged width is that loop's signature, so it
      // is ignored; width changes (sidebar toggle, expand) and genuine shrinks are
      // always honored, keeping the chart correctly fitted without runaway growth.
      let appliedBox: Box | undefined
      const resizeChart = (entries: ReadonlyArray<ResizeObserverEntry>) => {
        const { width, height } = readBox(element, entries)
        if (width <= 0 || height <= 0) return
        if (appliedBox && width === appliedBox.width && height >= appliedBox.height) return
        const crossedCompactLabelBoundary = appliedBox
          ? (appliedBox.width < 500) !== (width < 500)
          : false
        appliedBox = { width, height }
        beginRender()
        chart.resize({ width, height })
        if (crossedCompactLabelBoundary && (input.kind === 'donut' || input.kind === 'pie')) render()
      }
      const resizeObserver = observeSize(element, resizeChart)
      const themeObserver = observeTheme(element, render)
      // Canvas text is rasterized at draw time. If the dashboard font is still
      // loading, a first draw captures fallback-font metrics and a later redraw
      // can change both glyphs and axis intervals. Start the first draw only
      // after the requested face settles so there is no intermediate canvas for
      // users or pixel-exact VR to observe.
      const fonts = typeof document === 'undefined' ? undefined : document.fonts
      if (fonts) {
        const fontFamily = buildEChartsTheme(element, input.theme).fontFamily
        // Canvas does not reliably start a web-font request merely by assigning
        // ctx.font. FontFaceSet.load does, and its promise resolves only when a
        // draw can use the final glyph raster instead of the fallback.
        void fonts.load(`16px ${fontFamily}`).then(render).catch((error: unknown) => {
          console.warn('[lens] chart font failed to load; using fallback metrics', error)
          render()
        })
      } else render()

      return {
        update(nextInput: ChartInput) {
          const selectionOnly = isSelectionOnlyChange(input, nextInput)
          input = nextInput
          if (selectionOnly) restyleSelection()
          else render()
        },
        resetZoom() {
          beginRender()
          chart.dispatchAction({ type: 'dataZoom', start: 0, end: 100 })
        },
        dispose() {
          disposed = true
          if (readinessFrame !== undefined) cancelAnimationFrame(readinessFrame)
          for (const remove of detach) remove()
          detach.length = 0
          // Hide before disposing: a tooltip shown at teardown has already been
          // handed to `body`, and hiding it is what returns it to ECharts' own
          // cleanup path.
          hideTooltip()
          hideAxisTooltip()
          resizeObserver?.disconnect()
          themeObserver?.disconnect()
          chart.dispose()
          delete element.dataset.chartReady
        },
      }
    },
  }
}

export const echartsAdapter = createEChartsAdapter()
