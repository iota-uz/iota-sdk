import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { DataZoomComponent, GraphicComponent, GridComponent, TooltipComponent } from 'echarts/components'
import { init, registerMap, use as registerEChartsModules, type ECharts, type EChartsCoreOption } from 'echarts/core'
import { UniversalTransition } from 'echarts/features'
import { CanvasRenderer } from 'echarts/renderers'
import type { ChartAdapter, ChartAnchor, ChartEvents, ChartInput, ChartInstance } from '../adapter'
import { nodeKeyFromEvent } from './events'
import { buildChartOption } from './options'
import { buildEChartsTheme } from './theme'

registerEChartsModules([
  BarChart, LineChart, PieChart,
  DataZoomComponent, GraphicComponent, GridComponent, TooltipComponent,
  CanvasRenderer, UniversalTransition,
])

const optionalKinds = new Set<ChartInput['kind']>(['boxplot', 'heatmap', 'map'])
let optionalModules: Promise<void> | undefined

/** Loads distribution/map renderers only when a document actually uses one. */
function ensureKindModules(kind: ChartInput['kind']): Promise<void> | undefined {
  if (!optionalKinds.has(kind)) return undefined
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
  return optionalModules
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

      const responsiveInput = (): ChartInput => ({ ...input, viewportWidth: element.clientWidth })

      let disposed = false
      let renderGeneration = 0
      const renderReady = () => {
        if (disposed) return
        if (input.kind === 'map' && input.map) registerMap(input.map.name, input.map.geoJSON as unknown as Parameters<typeof registerMap>[1])
        const theme = buildEChartsTheme(element, input.theme)
        const option: EChartsCoreOption = buildChartOption(responsiveInput(), theme)
        chart.setOption(option, { notMerge: false, replaceMerge: ['series', 'xAxis', 'yAxis'] })
      }
      const render = () => {
        const generation = ++renderGeneration
        const pending = ensureKindModules(input.kind)
        if (!pending) {
          renderReady()
          return
        }
        void pending.then(() => {
          if (generation === renderGeneration) renderReady()
        }).catch((error: unknown) => {
          console.error('[lens] optional ECharts modules failed to load', error)
        })
      }
      // Selection restyle: merge the rebuilt option in place with animation
      // forced off, so the outline appears instantly without replacing the
      // series or re-running the entrance transition.
      const restyleSelection = () => {
        const theme = buildEChartsTheme(element, input.theme)
        const option = buildChartOption(responsiveInput(), theme) as EChartsCoreOption & { animation?: boolean }
        option.animation = false
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
        chart.resize({ width, height })
        if (crossedCompactLabelBoundary && (input.kind === 'donut' || input.kind === 'pie')) render()
      }
      const resizeObserver = observeSize(element, resizeChart)
      const themeObserver = observeTheme(element, render)
      render()

      return {
        update(nextInput: ChartInput) {
          const selectionOnly = isSelectionOnlyChange(input, nextInput)
          input = nextInput
          if (selectionOnly) restyleSelection()
          else render()
        },
        resetZoom() {
          chart.dispatchAction({ type: 'dataZoom', start: 0, end: 100 })
        },
        dispose() {
          disposed = true
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
        },
      }
    },
  }
}

export const echartsAdapter = createEChartsAdapter()
