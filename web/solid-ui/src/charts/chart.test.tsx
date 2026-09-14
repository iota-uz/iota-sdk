import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createSignal } from 'solid-js'
import { render } from 'solid-js/web'
import { Chart, type ApexChartInstance, type ApexChartOptions } from './Chart'

let dispose: (() => void) | undefined
const host = document.createElement('div')

beforeEach(() => document.body.append(host))

afterEach(async () => {
  dispose?.()
  dispose = undefined
  host.replaceChildren()
  host.remove()
  delete window.ApexCharts
  await Promise.resolve()
})

function instance(overrides: Partial<ApexChartInstance> = {}): ApexChartInstance {
  return {
    render: vi.fn(async () => undefined),
    destroy: vi.fn(),
    updateOptions: vi.fn(async () => undefined),
    w: { globals: { seriesNames: ['Revenue'], collapsedSeriesIndices: [], collapsedSeries: [], axisCharts: true } },
    ...overrides,
  }
}

describe('Chart', () => {
  it('mounts, updates options, recreates on the SDK event, and destroys on cleanup', async () => {
    const charts: ApexChartInstance[] = []
    const factory = vi.fn((_element: HTMLElement, _options: ApexChartOptions) => {
      const chart = instance()
      charts.push(chart)
      return chart
    })
    let setOptions!: (options: ApexChartOptions) => ApexChartOptions
    dispose = render(() => {
      const [options, update] = createSignal<ApexChartOptions>({ chart: { type: 'bar' }, series: [] })
      setOptions = update
      return <Chart options={options()} factory={factory} label="Revenue" />
    }, host)
    await vi.waitFor(() => expect(factory).toHaveBeenCalledOnce())
    setOptions({ chart: { type: 'line' }, series: [] })
    await vi.waitFor(() => expect(charts[0]!.updateOptions).toHaveBeenCalledOnce())
    document.dispatchEvent(new CustomEvent('sdk:rerenderCharts'))
    await vi.waitFor(() => expect(factory).toHaveBeenCalledTimes(2))
    expect(charts[0]!.destroy).toHaveBeenCalledOnce()
    dispose()
    dispose = undefined
    await vi.waitFor(() => expect(charts[1]!.destroy).toHaveBeenCalledOnce())
  })

  it('shows an accessible error and retries after the constructor becomes available', async () => {
    dispose = render(() => <Chart options={{ series: [] }} />, host)
    await vi.waitFor(() => expect(host.querySelector('[role="alert"]')).not.toBeNull())
    const chart = instance()
    window.ApexCharts = class { constructor() { return chart } } as unknown as typeof window.ApexCharts
    ;(host.querySelector('button') as HTMLButtonElement).click()
    await vi.waitFor(() => expect(chart.render).toHaveBeenCalledOnce())
    expect(host.querySelector('[role="alert"]')).toBeNull()
  })

  it('masks circular series and publishes hidden names to the rerender scope', async () => {
    const updateSeries = vi.fn()
    let element!: HTMLElement
    const chart = instance({
      render: vi.fn(async () => {
        const legend = document.createElement('button')
        legend.className = 'apexcharts-legend-series'
        legend.setAttribute('rel', '2')
        element.append(legend)
      }),
      ctx: {
        w: {
          config: { chart: { type: 'donut', animations: {} }, series: [10, 20] },
          globals: { labels: ['A', 'B'], collapsedSeriesIndices: [], collapsedSeries: [], axisCharts: false },
        },
        updateHelpers: { _updateSeries: updateSeries },
      },
    })
    const scope = document.createElement('div') as HTMLDivElement & { __apexSharedHidden?: string[] }
    scope.dataset.lensRerenderScope = ''
    host.append(scope)
    dispose = render(() => <Chart options={{ chart: { type: 'donut' }, series: [10, 20] }} factory={(node) => { element = node; return chart }} />, scope)
    await vi.waitFor(() => expect(element.querySelector('.apexcharts-legend-series')).not.toBeNull())
    ;(element.querySelector('.apexcharts-legend-series') as HTMLButtonElement).click()
    await vi.waitFor(() => expect(updateSeries).toHaveBeenCalledWith([10, 0], false))
    expect(scope.__apexSharedHidden).toEqual(['B'])
  })
})
