import { waitFor } from '@testing-library/react'
import type { EChartsOption } from 'echarts'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ChartInput } from '../adapter'
import { createEChartsAdapter } from './adapter'

class FakeResizeObserver {
  static instances: FakeResizeObserver[] = []
  readonly disconnect = vi.fn()
  readonly observe = vi.fn()
  private readonly callback: ResizeObserverCallback

  constructor(callback: ResizeObserverCallback) {
    this.callback = callback
    FakeResizeObserver.instances.push(this)
  }

  resize() {
    this.callback([], this as unknown as ResizeObserver)
  }
}

class FakeChart {
  readonly handlers = new Map<string, (event: { data?: unknown }) => void>()
  readonly options: EChartsOption[] = []
  readonly resize = vi.fn()
  readonly dispose = vi.fn()

  on(name: string, handler: (event: { data?: unknown }) => void) {
    this.handlers.set(name, handler)
  }

  setOption(option: EChartsOption) {
    this.options.push(option)
  }

  emit(name: string, event: { data?: unknown } = {}) {
    this.handlers.get(name)?.(event)
  }
}

function chartInput(): ChartInput {
  return {
    kind: 'bar',
    frame: {
      columns: [
        { name: 'id', type: 'string' },
        { name: 'label', type: 'string' },
        { name: 'value', type: 'number' },
      ],
      rows: [['stable/key', 'Localized label', 42]],
    },
    encoding: { id: 'id', label: 'label', value: 'value' },
    format: (_field, value) => String(value),
    theme: { palette: {}, series: {} },
  }
}

describe('ECharts adapter', () => {
  beforeEach(() => {
    FakeResizeObserver.instances = []
    vi.stubGlobal('ResizeObserver', FakeResizeObserver)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('emits NodeKeys for selection and hover and clears hover on exit', () => {
    const chart = new FakeChart()
    const onSelect = vi.fn()
    const onHover = vi.fn()
    const element = document.createElement('div')
    document.body.append(element)
    const instance = createEChartsAdapter(() => chart as never).mount(element, chartInput(), { onSelect, onHover })

    chart.emit('click', { data: { nodeKey: 'stable/key' } })
    chart.emit('click', { data: { value: 42 } })
    chart.emit('mouseover', { data: { nodeKey: 'stable/key' } })
    chart.emit('mouseout')

    expect(onSelect).toHaveBeenCalledOnce()
    expect(onSelect).toHaveBeenCalledWith('stable/key')
    expect(onHover).toHaveBeenNthCalledWith(1, 'stable/key')
    expect(onHover).toHaveBeenNthCalledWith(2, null)

    FakeResizeObserver.instances[0]?.resize()
    expect(chart.resize).toHaveBeenCalledOnce()
    instance.dispose()
    expect(chart.dispose).toHaveBeenCalledOnce()
    expect(FakeResizeObserver.instances[0]?.disconnect).toHaveBeenCalledOnce()
  })

  it('rebuilds centralized theme options when CSS variables change', async () => {
    const chart = new FakeChart()
    const root = document.createElement('div')
    root.className = 'lens-root'
    root.dataset.theme = 'light'
    root.style.setProperty('--lens-text', '#111111')
    const element = document.createElement('div')
    root.append(element)
    document.body.append(root)
    const instance = createEChartsAdapter(() => chart as never).mount(element, chartInput(), {
      onSelect: vi.fn(),
      onHover: vi.fn(),
    })

    expect(chart.options[0]?.textStyle).toMatchObject({ color: '#111111' })
    root.dataset.theme = 'dark'
    root.style.setProperty('--lens-text', '#eeeeee')

    await waitFor(() => expect(chart.options.at(-1)?.textStyle).toMatchObject({ color: '#eeeeee' }))
    expect(chart.options.length).toBeGreaterThan(1)
    instance.dispose()
  })

  it('updates data incrementally without disposing the chart', () => {
    const chart = new FakeChart()
    const element = document.createElement('div')
    document.body.append(element)
    const instance = createEChartsAdapter(() => chart as never).mount(element, chartInput(), {
      onSelect: vi.fn(),
      onHover: vi.fn(),
    })
    const next = chartInput()
    next.frame = { ...next.frame, rows: [['next/key', 'Next', 84]] }

    instance.update(next)

    expect(chart.options).toHaveLength(2)
    expect(chart.dispose).not.toHaveBeenCalled()
    instance.dispose()
  })
})
