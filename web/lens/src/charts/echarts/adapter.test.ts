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

  resize(box: { width: number; height: number } = { width: 640, height: 320 }) {
    const entry = { contentRect: { width: box.width, height: box.height } } as ResizeObserverEntry
    this.callback([entry], this as unknown as ResizeObserver)
  }
}

class FakeChart {
  readonly handlers = new Map<string, (event: Record<string, unknown>) => void>()
  readonly options: EChartsOption[] = []
  readonly mergeOptions: Array<{ notMerge?: boolean; replaceMerge?: string[] }> = []
  readonly resize = vi.fn()
  readonly dispose = vi.fn()
  readonly dispatchAction = vi.fn()

  on(name: string, handler: (event: Record<string, unknown>) => void) {
    this.handlers.set(name, handler)
  }

  setOption(option: EChartsOption, mergeOptions: { notMerge?: boolean; replaceMerge?: string[] }) {
    this.options.push(option)
    this.mergeOptions.push(mergeOptions)
  }

  emit(name: string, event: Record<string, unknown> = {}) {
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

function multiSeriesInput(seriesNames: string[]): ChartInput {
  return {
    ...chartInput(),
    frame: {
      columns: [
        { name: 'id', type: 'string' },
        { name: 'label', type: 'string' },
        { name: 'series', type: 'string' },
        { name: 'value', type: 'number' },
      ],
      rows: seriesNames.map((name, index) => [`${name}/key`, 'Localized label', name, index + 1]),
    },
    encoding: { id: 'id', label: 'label', series: 'series', value: 'value' },
  }
}

describe('ECharts adapter', () => {
  let originalFonts: PropertyDescriptor | undefined

  beforeEach(() => {
    originalFonts = Object.getOwnPropertyDescriptor(document, 'fonts')
    FakeResizeObserver.instances = []
    vi.stubGlobal('ResizeObserver', FakeResizeObserver)
  })

  afterEach(() => {
    if (originalFonts) Object.defineProperty(document, 'fonts', originalFonts)
    else Reflect.deleteProperty(document, 'fonts')
    vi.unstubAllGlobals()
  })

  it('waits for the requested font before the first canvas draw', async () => {
    let finishLoading: ((faces: FontFace[]) => void) | undefined
    const load = vi.fn(() => new Promise<FontFace[]>((resolve) => { finishLoading = resolve }))
    Object.defineProperty(document, 'fonts', { configurable: true, value: { load } })
    const chart = new FakeChart()
    const element = document.createElement('div')
    document.body.append(element)

    const instance = createEChartsAdapter(() => chart as never).mount(element, chartInput(), {
      onSelect: vi.fn(), onHover: vi.fn(),
    })

    expect(load).toHaveBeenCalledOnce()
    expect(chart.options).toHaveLength(0)
    finishLoading?.([])
    await waitFor(() => expect(chart.options).toHaveLength(1))
    instance.dispose()
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
    expect(onSelect).toHaveBeenCalledWith('stable/key', undefined, { newTab: false })
    expect(onHover).toHaveBeenNthCalledWith(1, 'stable/key')
    expect(onHover).toHaveBeenNthCalledWith(2, null)

    FakeResizeObserver.instances[0]?.resize()
    expect(chart.resize).toHaveBeenCalledOnce()
    instance.dispose()
    expect(chart.dispose).toHaveBeenCalledOnce()
    expect(FakeResizeObserver.instances[0]?.disconnect).toHaveBeenCalledOnce()
  })

  it('carries Ctrl/Cmd activation and resets the time zoom window', () => {
    const chart = new FakeChart()
    const onSelect = vi.fn()
    const element = document.createElement('div')
    document.body.append(element)
    const instance = createEChartsAdapter(() => chart as never).mount(element, chartInput(), { onSelect, onHover: vi.fn() })

    chart.emit('click', { data: { nodeKey: 'stable/key' }, event: { event: { ctrlKey: true, metaKey: false } } })
    expect(onSelect).toHaveBeenCalledWith('stable/key', undefined, { newTab: true })
    instance.resetZoom?.()
    expect(chart.dispatchAction).toHaveBeenCalledWith({ type: 'dataZoom', start: 0, end: 100 })
    instance.dispose()
  })

  it('shows the full axis label on hover and removes it on exit', () => {
    const chart = new FakeChart()
    const element = document.createElement('div')
    document.body.append(element)
    const instance = createEChartsAdapter(() => chart as never).mount(element, chartInput(), {
      onSelect: vi.fn(), onHover: vi.fn(),
    })

    chart.emit('mouseover', {
      componentType: 'xAxis',
      value: 'A very long insurance product name',
      event: { event: new MouseEvent('mousemove', { clientX: 120, clientY: 80 }) },
    })
    expect(document.querySelector('.lens-axis-label-tooltip')).toHaveTextContent('A very long insurance product name')

    chart.emit('mouseout')
    expect(document.querySelector('.lens-axis-label-tooltip')).toBeNull()
    instance.dispose()
  })

  it('ignores self-fed height growth so the chart converges instead of inflating', () => {
    const chart = new FakeChart()
    const element = document.createElement('div')
    document.body.append(element)
    const instance = createEChartsAdapter(() => chart as never).mount(element, chartInput(), {
      onSelect: vi.fn(),
      onHover: vi.fn(),
    })
    const observer = FakeResizeObserver.instances[0]!

    // Width is settled; the canvas then reports an ever-growing height — the
    // ECharts resize-feedback loop the sidebar animation triggers.
    observer.resize({ width: 640, height: 300 })
    observer.resize({ width: 640, height: 320 })
    observer.resize({ width: 640, height: 340 })
    observer.resize({ width: 640, height: 360 })

    // Only the first, non-inflated height is applied; the growth is suppressed,
    // so the chart converges rather than climbing without bound.
    expect(chart.resize.mock.calls.map((call) => call[0] as { width: number; height: number })).toEqual([{ width: 640, height: 300 }])

    // A width change (sidebar toggling the layout) is always honored so the
    // chart still fits its new column.
    observer.resize({ width: 520, height: 300 })
    expect(chart.resize).toHaveBeenLastCalledWith({ width: 520, height: 300 })

    // A genuine shrink is honored too — only self-inflating growth is ignored.
    observer.resize({ width: 520, height: 260 })
    expect(chart.resize).toHaveBeenLastCalledWith({ width: 520, height: 260 })

    instance.dispose()
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

  it('rebuilds dark theme fallbacks when the root dark class changes', async () => {
    const chart = new FakeChart()
    const root = document.createElement('div')
    root.className = 'lens-root'
    const element = document.createElement('div')
    root.append(element)
    document.body.append(root)
    const instance = createEChartsAdapter(() => chart as never).mount(element, chartInput(), {
      onSelect: vi.fn(),
      onHover: vi.fn(),
    })

    expect(chart.options[0]?.textStyle).toMatchObject({ color: '#334155' })
    root.classList.add('dark')

    await waitFor(() => expect(chart.options.at(-1)?.textStyle).toMatchObject({ color: '#e2e8f0' }))
    instance.dispose()
  })

  it('restyles a selection in place without replacing the series or re-running entrance animation', () => {
    const chart = new FakeChart()
    const element = document.createElement('div')
    document.body.append(element)
    const base = chartInput()
    const instance = createEChartsAdapter(() => chart as never).mount(element, base, {
      onSelect: vi.fn(),
      onHover: vi.fn(),
    })

    // Only the selected mark changes; every other input reference is identical.
    instance.update({ ...base, selectedKey: 'stable/key' })

    expect(chart.options).toHaveLength(2)
    // A selection-only change merges in place — no series-replacing merge.
    expect(chart.mergeOptions[1]).toEqual({ notMerge: false })
    expect(chart.mergeOptions[1]?.replaceMerge).toBeUndefined()
    // The restyle suppresses animation, so the outline appears without the
    // series tearing down and re-entering.
    expect((chart.options[1] as { animation?: boolean }).animation).toBe(false)
    // The first paint still animated normally.
    expect((chart.options[0] as { animation?: boolean }).animation).toBe(true)
    expect(chart.dispose).not.toHaveBeenCalled()
    instance.dispose()
  })

  it('replaces shrinking series and axes without disposing the chart', () => {
    const chart = new FakeChart()
    const element = document.createElement('div')
    document.body.append(element)
    const instance = createEChartsAdapter(() => chart as never).mount(element, multiSeriesInput(['Revenue', 'Cost', 'Profit']), {
      onSelect: vi.fn(),
      onHover: vi.fn(),
    })

    instance.update(multiSeriesInput(['Revenue']))

    expect(chart.options).toHaveLength(2)
    expect(chart.mergeOptions).toHaveLength(2)
    expect(chart.mergeOptions[1]).toEqual({
      notMerge: false,
      replaceMerge: ['series', 'xAxis', 'yAxis'],
    })
    expect(chart.options[0]?.series).toHaveLength(3)
    expect(chart.options[1]?.series).toHaveLength(1)
    expect(chart.options[1]?.series).toMatchObject([{ name: 'Revenue' }])
    expect(chart.dispose).not.toHaveBeenCalled()
    instance.dispose()
  })

  // A tooltip that outlives the pointer is worse than no tooltip: it keeps
  // asserting a number about whatever it happens to be covering. ECharts only
  // hides it from events over the canvas, so every way of leaving the chart
  // without crossing that canvas has to be closed here.
  describe('tooltip dismissal', () => {
    function mountInScroller() {
      const chart = new FakeChart()
      const scroller = document.createElement('div')
      scroller.style.overflowY = 'auto'
      const element = document.createElement('div')
      scroller.append(element)
      document.body.append(scroller)
      const instance = createEChartsAdapter(() => chart as never).mount(element, chartInput(), {
        onSelect: vi.fn(),
        onHover: vi.fn(),
      })
      const hideCalls = () => chart.dispatchAction.mock.calls
        .filter((call) => (call[0] as { type?: string } | undefined)?.type === 'hideTip').length
      return { chart, scroller, element, instance, hideCalls }
    }

    it('hides the tooltip when the pointer leaves, the page scrolls, or the tab goes away', () => {
      const { scroller, element, instance, hideCalls } = mountInScroller()

      element.dispatchEvent(new MouseEvent('mouseleave'))
      expect(hideCalls()).toBe(1)

      // The dashboard scrolls an inner container, not the document — this is
      // the case that stranded tooltips mid-page.
      scroller.dispatchEvent(new Event('scroll'))
      expect(hideCalls()).toBe(2)

      window.dispatchEvent(new Event('scroll'))
      expect(hideCalls()).toBe(3)

      document.dispatchEvent(new Event('visibilitychange'))
      expect(hideCalls()).toBe(4)

      window.dispatchEvent(new Event('blur'))
      expect(hideCalls()).toBe(5)

      instance.dispose()
    })

    it('stops listening once disposed so a torn-down chart cannot be poked', () => {
      const { chart, scroller, element, instance, hideCalls } = mountInScroller()

      instance.dispose()
      const afterDispose = hideCalls()
      expect(chart.dispose).toHaveBeenCalledOnce()

      element.dispatchEvent(new MouseEvent('mouseleave'))
      scroller.dispatchEvent(new Event('scroll'))
      window.dispatchEvent(new Event('blur'))
      expect(hideCalls()).toBe(afterDispose)
    })
  })
})
