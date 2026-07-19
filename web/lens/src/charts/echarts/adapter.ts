import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { init, use as registerEChartsModules, type ECharts, type EChartsCoreOption } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import type { ChartAdapter, ChartEvents, ChartInput, ChartInstance } from '../adapter'
import { nodeKeyFromEvent } from './events'
import { buildChartOption } from './options'
import { buildEChartsTheme } from './theme'

registerEChartsModules([BarChart, LineChart, PieChart, GridComponent, TooltipComponent, CanvasRenderer])

type ChartInitializer = (element: HTMLElement) => ECharts

function observeTheme(element: HTMLElement, rebuild: () => void): MutationObserver | undefined {
  if (typeof MutationObserver === 'undefined') return undefined
  const root = element.closest<HTMLElement>('.lens-root') ?? element
  const observer = new MutationObserver(rebuild)
  observer.observe(root, { attributes: true, attributeFilter: ['class', 'data-theme', 'style'] })
  return observer
}

function observeSize(element: HTMLElement, resize: () => void): ResizeObserver | undefined {
  if (typeof ResizeObserver === 'undefined') return undefined
  const observer = new ResizeObserver(resize)
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
      const select = (event: Parameters<typeof nodeKeyFromEvent>[0]) => {
        const key = nodeKeyFromEvent(event)
        if (key !== undefined) events.onSelect(key)
      }
      const hover = (event: Parameters<typeof nodeKeyFromEvent>[0]) => {
        const key = nodeKeyFromEvent(event)
        if (key !== undefined) events.onHover(key)
      }

      chart.on('click', select)
      chart.on('mouseover', hover)
      chart.on('mouseout', () => events.onHover(null))
      chart.on('globalout', () => events.onHover(null))
      const resizeObserver = observeSize(element, () => chart.resize())
      const themeObserver = observeTheme(element, render)
      render()

      return {
        update(nextInput: ChartInput) {
          input = nextInput
          render()
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
