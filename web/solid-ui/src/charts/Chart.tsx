import { createEffect, createSignal, onCleanup, onMount, Show, splitProps, type JSX } from 'solid-js'

export type ApexChartOptions = Record<string, any>

export interface ApexChartInstance {
  render(): void | Promise<unknown>
  destroy(): void
  updateOptions?(options: ApexChartOptions, redrawPaths?: boolean, animate?: boolean): void | Promise<unknown>
  hideSeries?(name: string): void
  ctx?: Record<string, any>
  w?: Record<string, any>
}

export interface ApexChartsConstructor {
  new (element: HTMLElement, options: ApexChartOptions): ApexChartInstance
}

export type ApexChartFactory = (element: HTMLElement, options: ApexChartOptions) => ApexChartInstance

export interface ChartProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onError'> {
  options: ApexChartOptions
  factory?: ApexChartFactory
  apexCharts?: ApexChartsConstructor
  label?: string
  retryLabel?: string
  onReady?: (chart: ApexChartInstance) => void
  onError?: (error: unknown) => void
}

declare global {
  interface Window {
    ApexCharts?: ApexChartsConstructor
    initializeChartEvents?: () => void
  }
}

type ChartContainer = HTMLDivElement & {
  __apexChart?: ApexChartInstance
  __apexHiddenSeries?: string[]
  __apexCircularOriginalSeries?: unknown[]
}

type RerenderScope = Element & { __apexSharedHidden?: string[] }

function chartContext(chart: ApexChartInstance): Record<string, any> {
  return chart.ctx ?? chart
}

export function apexHiddenSeries(chart: ApexChartInstance | undefined): string[] {
  if (!chart) return []
  const globals = (chart.ctx?.w ?? chart.w)?.globals
  if (!globals) return []
  const names: string[] = globals.seriesNames?.length ? globals.seriesNames : globals.labels ?? []
  const hidden = new Set<string>()
  for (const index of globals.collapsedSeriesIndices ?? []) if (names[index]) hidden.add(names[index])
  for (const series of globals.collapsedSeries ?? []) {
    if (typeof series === 'string') hidden.add(series)
    else if (series && typeof series.name === 'string') hidden.add(series.name)
  }
  return Array.from(hidden)
}

function sharedScope(container: Element): RerenderScope | undefined {
  return container.closest('[data-lens-rerender-scope]') as RerenderScope | null ?? undefined
}

function readSharedHidden(container: Element): string[] | undefined {
  const value = sharedScope(container)?.__apexSharedHidden
  return Array.isArray(value) ? value : undefined
}

function writeSharedHidden(container: Element, names: readonly string[]) {
  const scope = sharedScope(container)
  if (scope) scope.__apexSharedHidden = Array.from(names)
}

function isCircular(context: Record<string, any>): boolean {
  const type = context.w?.config?.chart?.type
  return type === 'pie' || type === 'donut' || type === 'polarArea'
}

export function setCircularSeriesHidden(chart: ApexChartInstance, seriesIndex: number, shouldHide: boolean, originalSeries: readonly unknown[]): boolean {
  const context = chartContext(chart)
  const globals = context.w?.globals
  const update = context.updateHelpers?._updateSeries
  if (!isCircular(context) || !globals || typeof update !== 'function' || !Array.isArray(originalSeries)) return false
  const collapsed = new Set<number>(globals.collapsedSeriesIndices ?? [])
  if (shouldHide) collapsed.add(seriesIndex)
  else collapsed.delete(seriesIndex)
  const indices = Array.from(collapsed).sort((left, right) => left - right)
  globals.collapsedSeriesIndices = indices
  globals.collapsedSeries = indices.map((index) => ({ index, data: originalSeries[index] }))
  const masked = originalSeries.map((value, index) => collapsed.has(index) ? 0 : value)
  const dynamic = Boolean(context.w?.config?.chart?.animations?.dynamicAnimation?.enabled)
  update.call(context.updateHelpers, masked, dynamic)
  return true
}

function withSharedLegendEvents(options: ApexChartOptions, publish: (chart: ApexChartInstance) => void): ApexChartOptions {
  const chartOptions = { ...(options.chart ?? {}) }
  const events = { ...(chartOptions.events ?? {}) }
  const previous = events.legendClick
  events.legendClick = (chart: ApexChartInstance, seriesIndex: number, eventOptions: unknown) => {
    if (typeof previous === 'function') previous(chart, seriesIndex, eventOptions)
    queueMicrotask(() => publish(chart))
  }
  chartOptions.events = events
  return { ...options, chart: chartOptions }
}

export function Chart(props: ChartProps) {
  let container!: ChartContainer
  let chart: ApexChartInstance | undefined
  let disposed = false
  let mounted = false
  let renderedOptions: ApexChartOptions | undefined
  let renderedFactory: ApexChartFactory | undefined
  let previousFactory: ApexChartFactory | undefined
  let previousConstructor: ApexChartsConstructor | undefined
  let queue = Promise.resolve()
  const [failure, setFailure] = createSignal<unknown>()
  const [local, native] = splitProps(props, ['options', 'factory', 'apexCharts', 'label', 'retryLabel', 'onReady', 'onError', 'class', 'ref'])

  const defaultFactory: ApexChartFactory = (element, options) => {
    const Constructor = local.apexCharts ?? window.ApexCharts
    if (!Constructor) throw new Error('ApexCharts is not available; provide Chart.factory or Chart.apexCharts')
    return new Constructor(element, options)
  }

  const resolveFactory = (): ApexChartFactory => {
    if (local.factory) return local.factory
    return defaultFactory
  }

  const publishHidden = (source: ApexChartInstance) => {
    const names = apexHiddenSeries(source)
    container.__apexHiddenSeries = names
    writeSharedHidden(container, names)
  }

  const destroy = () => {
    if (!chart) return
    chart.destroy()
    chart = undefined
    container.__apexChart = undefined
  }

  const mountChart = async (options: ApexChartOptions, factory: ApexChartFactory) => {
    const shared = readSharedHidden(container)
    const hidden = new Set(shared ?? container.__apexHiddenSeries ?? [])
    if (chart && shared === undefined) for (const name of apexHiddenSeries(chart)) hidden.add(name)
    destroy()
    container.replaceChildren()
    const configured = withSharedLegendEvents(options, publishHidden)
    chart = factory(container, configured)
    container.__apexChart = chart
    container.__apexHiddenSeries = Array.from(hidden)
    container.__apexCircularOriginalSeries = Array.isArray(options.series) ? Array.from(options.series) : []
    const current = chart
    await current.render()
    if (disposed || chart !== current) return
    const globals = (current.w ?? current.ctx?.w)?.globals
    const names: string[] = globals?.seriesNames?.length ? globals.seriesNames : globals?.labels ?? []
    for (const name of hidden) {
      const index = names.indexOf(name)
      if (index < 0) continue
      if (globals?.axisCharts && current.hideSeries) current.hideSeries(name)
      else setCircularSeriesHidden(current, index, true, container.__apexCircularOriginalSeries ?? [])
    }
    container.__apexHiddenSeries = apexHiddenSeries(current)
    renderedOptions = options
    renderedFactory = factory
    setFailure(undefined)
    local.onReady?.(current)
  }

  const schedule = (options: ApexChartOptions, forceRecreate = false) => {
    queue = queue.catch(() => undefined).then(async () => {
      if (disposed || !mounted) return
      try {
        const factory = resolveFactory()
        if (!forceRecreate && chart && renderedFactory === factory && renderedOptions !== options && chart.updateOptions) {
          const configured = withSharedLegendEvents(options, publishHidden)
          await chart.updateOptions(configured, false, true)
          renderedOptions = options
          setFailure(undefined)
          return
        }
        await mountChart(options, factory)
      } catch (error) {
        if (disposed) return
        setFailure(error)
        local.onError?.(error)
      }
    })
  }

  const handleCircularLegend = (event: MouseEvent) => {
    const legend = event.composedPath().find((node) => node instanceof Element && node.classList.contains('apexcharts-legend-series')) as Element | undefined
    if (!legend || !container.contains(legend) || !chart) return
    const index = Number.parseInt(legend.getAttribute('rel') ?? '', 10) - 1
    const context = chartContext(chart)
    if (!Number.isInteger(index) || index < 0 || !isCircular(context)) return
    const collapsed: number[] = context.w?.globals?.collapsedSeriesIndices ?? []
    if (collapsed.length === 0) container.__apexCircularOriginalSeries = Array.from(context.w?.config?.series ?? local.options.series ?? [])
    if (!setCircularSeriesHidden(chart, index, !collapsed.includes(index), container.__apexCircularOriginalSeries ?? [])) return
    event.preventDefault()
    event.stopImmediatePropagation()
    queueMicrotask(() => chart && publishHidden(chart))
  }

  const handleRerender = (event: Event) => {
    const root = (event as CustomEvent<{ root?: Element }>).detail?.root
    if (root && !root.contains(container) && !container.contains(root)) return
    schedule(local.options, true)
  }

  onMount(() => {
    mounted = true
    container.addEventListener('click', handleCircularLegend, true)
    document.addEventListener('sdk:rerenderCharts', handleRerender)
    window.initializeChartEvents?.()
  })

  createEffect(() => {
    const options = local.options
    const factory = local.factory
    const constructor = local.apexCharts
    const forceRecreate = renderedOptions !== undefined && (factory !== previousFactory || constructor !== previousConstructor)
    previousFactory = factory
    previousConstructor = constructor
    if (mounted) schedule(options, forceRecreate)
  })

  onCleanup(() => {
    disposed = true
    container.removeEventListener('click', handleCircularLegend, true)
    document.removeEventListener('sdk:rerenderCharts', handleRerender)
    void queue.finally(destroy)
  })

  return (
    <>
      <div
        {...native}
        ref={(node) => {
          container = node as ChartContainer
          if (typeof local.ref === 'function') local.ref(node)
        }}
        class={local.class}
        role={local.label ? 'img' : undefined}
        aria-label={local.label}
        aria-busy={!failure() && !chart ? 'true' : undefined}
      />
      <Show when={failure()}>
        <div role="alert" class="flex items-center gap-2 text-sm text-red-600">
          <span>{failure() instanceof Error ? (failure() as Error).message : 'Unable to render chart'}</span>
          <button type="button" class="btn btn-secondary btn-sm" onClick={() => {
            setFailure(undefined)
            schedule(local.options, true)
          }}>{local.retryLabel ?? 'Retry'}</button>
        </div>
      </Show>
    </>
  )
}
