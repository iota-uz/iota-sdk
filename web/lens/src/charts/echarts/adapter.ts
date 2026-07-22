import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { init, use as registerEChartsModules, type ECharts, type EChartsCoreOption } from 'echarts/core'
import { UniversalTransition } from 'echarts/features'
import { CanvasRenderer } from 'echarts/renderers'
import type { ChartAdapter, ChartAnchor, ChartEvents, ChartInput, ChartInstance } from '../adapter'
import { nodeKeyFromEvent } from './events'
import { buildChartOption } from './options'
import { buildEChartsTheme } from './theme'

registerEChartsModules([BarChart, LineChart, PieChart, GridComponent, TooltipComponent, CanvasRenderer, UniversalTransition])

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

      const render = () => {
        const theme = buildEChartsTheme(element, input.theme)
        const option: EChartsCoreOption = buildChartOption(input, theme)
        chart.setOption(option, { notMerge: false, replaceMerge: ['series', 'xAxis', 'yAxis'] })
      }
      // Selection restyle: merge the rebuilt option in place with animation
      // forced off, so the outline appears instantly without replacing the
      // series or re-running the entrance transition.
      const restyleSelection = () => {
        const theme = buildEChartsTheme(element, input.theme)
        const option = buildChartOption(input, theme) as EChartsCoreOption & { animation?: boolean }
        option.animation = false
        chart.setOption(option, { notMerge: false })
      }
      const select = (event: Parameters<typeof nodeKeyFromEvent>[0]) => {
        const key = nodeKeyFromEvent(event)
        if (key !== undefined) events.onSelect(key, anchorFromEvent(event))
      }
      const hover = (event: Parameters<typeof nodeKeyFromEvent>[0]) => {
        const key = nodeKeyFromEvent(event)
        if (key !== undefined) events.onHover(key)
      }

      chart.on('click', select)
      chart.on('mouseover', hover)
      chart.on('mouseout', () => events.onHover(null))
      chart.on('globalout', () => events.onHover(null))

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
        appliedBox = { width, height }
        chart.resize({ width, height })
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
        dispose() {
          resizeObserver?.disconnect()
          themeObserver?.disconnect()
          chart.dispose()
        },
      }
    },
  }
}

export const echartsAdapter = createEChartsAdapter()
