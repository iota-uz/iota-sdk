import { Fragment, useEffect, useId, useMemo, useState } from 'react'
import ApexCharts from 'apexcharts'
import ReactApexChart from 'react-apexcharts'
import { DownloadSimple } from '@phosphor-icons/react'
import { ErrorBoundary, type ChartData } from '@iota-uz/sdk/bichat'
import type { RichChartData } from '../../charts/chartData'

type ScaleMode = 'linear' | 'log'

const DEFAULT_COLORS = ['#2563EB', '#059669', '#EA580C', '#DC2626', '#7C3AED', '#0891B2']

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

function toNumber(value: unknown): number | null {
  return isFiniteNumber(value) ? value : null
}

function isPieLike(type: string): boolean {
  return type === 'pie' || type === 'donut' || type === 'polararea' || type === 'radialbar'
}

function cloneDeep<T>(value: T): T {
  if (Array.isArray(value)) {
    return value.map((item) => cloneDeep(item)) as T
  }
  if (isRecord(value)) {
    const out: Record<string, unknown> = {}
    Object.entries(value).forEach(([k, v]) => {
      out[k] = cloneDeep(v)
    })
    return out as T
  }
  return value
}

function normalizeLegacyToOptions(chartData: ChartData): Record<string, unknown> {
  const raw = chartData as ChartData & {
    series?: unknown
    labels?: string[]
    colors?: string[]
    height?: number
    chartType?: string
    title?: string
  }
  const chartType = typeof raw.chartType === 'string' ? raw.chartType : 'line'
  const title = typeof raw.title === 'string' ? raw.title : 'Chart'
  const options: Record<string, unknown> = {
    chart: {
      type: chartType,
      height: typeof raw.height === 'number' ? raw.height : 350,
      toolbar: { show: false },
      animations: { enabled: false },
      fontFamily: 'inherit',
    },
    title: { text: title },
    colors: Array.isArray(raw.colors) && raw.colors.length > 0 ? raw.colors : DEFAULT_COLORS,
    series: Array.isArray(raw.series) ? raw.series : [],
  }
  if (Array.isArray(raw.labels) && raw.labels.length > 0) {
    if (isPieLike(chartType)) options.labels = raw.labels
    else options.xaxis = { categories: raw.labels }
  }
  return options
}

function deriveChartState(chartData: ChartData | RichChartData): {
  options: Record<string, unknown>
  chartType: string
  title: string
  warnings: string[]
} {
  const rich = chartData as RichChartData
  const options = isRecord(rich.options) ? cloneDeep(rich.options) : normalizeLegacyToOptions(chartData)
  const chart = isRecord(options.chart) ? options.chart : {}
  const chartTypeRaw = typeof chart.type === 'string' ? chart.type.toLowerCase() : undefined
  const chartType = chartTypeRaw || (typeof chartData.chartType === 'string' ? chartData.chartType : 'line')

  const title = (() => {
    if (isRecord(options.title) && typeof options.title.text === 'string' && options.title.text.trim()) {
      return options.title.text.trim()
    }
    if (typeof chartData.title === 'string' && chartData.title.trim()) return chartData.title.trim()
    return 'Chart'
  })()

  const warnings = Array.isArray(rich.warnings)
    ? rich.warnings.filter((warning): warning is string => typeof warning === 'string' && warning.trim().length > 0)
    : []

  chart.type = chartType
  options.chart = chart
  options.title = isRecord(options.title) ? { ...options.title, text: title } : { text: title }
  if (!Array.isArray(options.colors) || options.colors.length === 0) {
    options.colors = DEFAULT_COLORS
  }
  if (!Array.isArray(options.series)) {
    options.series = []
  }
  return { options, chartType, title, warnings }
}

function extractYValues(series: unknown, chartType: string): number[] {
  if (!Array.isArray(series)) return []
  const values: number[] = []
  if (isPieLike(chartType)) {
    series.forEach((point) => {
      if (isFiniteNumber(point)) values.push(point)
    })
    return values
  }
  series.forEach((seriesItem) => {
    if (!isRecord(seriesItem) || !Array.isArray(seriesItem.data)) return
    seriesItem.data.forEach((point) => {
      if (isFiniteNumber(point)) {
        values.push(point)
        return
      }
      if (isRecord(point) && isFiniteNumber(point.y)) {
        values.push(point.y)
      }
    })
  })
  return values
}

function getScaleStorageKey(title: string, chartType: string): string {
  const path = typeof window === 'undefined' ? 'global' : window.location.pathname || 'global'
  return `bichat:chart-scale:${path}:${title}:${chartType}`
}

function readScale(storageKey: string): ScaleMode | null {
  try {
    const value = window.localStorage.getItem(storageKey)
    if (value === 'linear' || value === 'log') return value
    return null
  } catch {
    return null
  }
}

function persistScale(storageKey: string, scale: ScaleMode): void {
  try {
    window.localStorage.setItem(storageKey, scale)
  } catch {
    // no-op when storage is unavailable
  }
}

function detectCurrencyHint(text: string): string | null {
  const codeMatch = text.match(/\b(USD|EUR|GBP|JPY|CHF|AUD|CAD|NZD|CNY|INR|RUB|UZS)\b/i)
  if (codeMatch) return codeMatch[1].toUpperCase()
  if (text.includes('$')) return 'USD'
  if (text.includes('EUR') || text.includes('€')) return 'EUR'
  if (text.includes('GBP') || text.includes('£')) return 'GBP'
  if (text.includes('JPY') || text.includes('¥')) return 'JPY'
  return null
}

function buildMoneyFormatter(chartData: ChartData | RichChartData, yValues: number[]) {
  const rich = chartData as RichChartData
  const textSignals = [
    chartData.title || '',
    typeof rich.meta?.currency === 'string' ? rich.meta.currency : '',
  ].join(' ')
  const monetaryKeywords =
    /\b(revenue|sales|amount|price|cost|profit|income|expense|balance|payment|salary|budget|currency)\b/i.test(
      textSignals
    ) || /[$€£¥]/.test(textSignals)
  if (!monetaryKeywords || yValues.length === 0) return null

  const currency =
    (typeof rich.meta?.currency === 'string' && rich.meta.currency.toUpperCase()) ||
    detectCurrencyHint(textSignals) ||
    'USD'
  const maxFractionDigits = yValues.some((value) => Math.abs(value) < 1) ? 4 : 2
  try {
    const formatter = new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency,
      maximumFractionDigits: maxFractionDigits,
    })
    return (value: number): string => formatter.format(value)
  } catch {
    const fallback = new Intl.NumberFormat(undefined, { maximumFractionDigits: maxFractionDigits })
    return (value: number): string => fallback.format(value)
  }
}

function applyFormatter(options: Record<string, unknown>, formatter: ((value: number) => string) | null): void {
  if (!formatter) return

  if (Array.isArray(options.yaxis)) {
    options.yaxis = options.yaxis.map((axis) => {
      if (!isRecord(axis)) return axis
      const labels = isRecord(axis.labels) ? { ...axis.labels } : {}
      if (typeof labels.formatter !== 'function') {
        labels.formatter = (value: unknown) => {
          const n = typeof value === 'number' ? value : Number(value)
          return Number.isFinite(n) ? formatter(n) : String(value)
        }
      }
      return { ...axis, labels }
    })
  } else {
    const yaxis = isRecord(options.yaxis) ? { ...options.yaxis } : {}
    const labels = isRecord(yaxis.labels) ? { ...yaxis.labels } : {}
    if (typeof labels.formatter !== 'function') {
      labels.formatter = (value: unknown) => {
        const n = typeof value === 'number' ? value : Number(value)
        return Number.isFinite(n) ? formatter(n) : String(value)
      }
    }
    yaxis.labels = labels
    options.yaxis = yaxis
  }

  const tooltip = isRecord(options.tooltip) ? { ...options.tooltip } : {}
  const yTooltip = isRecord(tooltip.y) ? { ...tooltip.y } : {}
  if (typeof yTooltip.formatter !== 'function') {
    yTooltip.formatter = (value: unknown) => {
      const n = typeof value === 'number' ? value : Number(value)
      return Number.isFinite(n) ? formatter(n) : String(value)
    }
  }
  tooltip.y = yTooltip
  options.tooltip = tooltip
}

function applyScale(options: Record<string, unknown>, scaleMode: ScaleMode): void {
  const logarithmic = scaleMode === 'log'
  if (Array.isArray(options.yaxis)) {
    options.yaxis = options.yaxis.map((axis) => (isRecord(axis) ? { ...axis, logarithmic } : axis))
    return
  }
  const yaxis = isRecord(options.yaxis) ? { ...options.yaxis } : {}
  yaxis.logarithmic = logarithmic
  options.yaxis = yaxis
}

function hasRenderableData(series: unknown, chartType: string): boolean {
  if (!Array.isArray(series) || series.length === 0) return false
  if (isPieLike(chartType)) return series.some((item) => isFiniteNumber(item))
  return series.some((item) => isRecord(item) && Array.isArray(item.data) && item.data.length > 0)
}

function buildTableRows(series: unknown, options: Record<string, unknown>, chartType: string): {
  headers: string[]
  rows: Array<Array<string | number | null>>
} | null {
  if (!Array.isArray(series) || series.length === 0) return null
  if (isPieLike(chartType)) {
    const labels = Array.isArray(options.labels) ? options.labels : []
    return {
      headers: ['Label', 'Value'],
      rows: series.map((value, idx) => [typeof labels[idx] === 'string' ? labels[idx] : `Item ${idx + 1}`, toNumber(value)]),
    }
  }

  type SeriesEntry = { name?: unknown; data: unknown[] }
  const normalizedSeries = series.filter(
    (entry): entry is SeriesEntry => isRecord(entry) && Array.isArray(entry.data)
  )
  if (normalizedSeries.length === 0) return null

  const categories: unknown[] =
    isRecord(options.xaxis) && Array.isArray(options.xaxis.categories) ? options.xaxis.categories : []
  const maxRows = normalizedSeries.reduce((max, entry) => Math.max(max, entry.data.length), 0)
  const headers = ['X', ...normalizedSeries.map((entry, idx) => (typeof entry.name === 'string' ? entry.name : `Series ${idx + 1}`))]

  const rows: Array<Array<string | number | null>> = []
  for (let i = 0; i < maxRows; i += 1) {
    const row: Array<string | number | null> = [
      typeof categories[i] === 'string' || typeof categories[i] === 'number' ? (categories[i] as string | number) : i + 1,
    ]
    normalizedSeries.forEach((entry) => {
      const point = entry.data[i]
      if (isFiniteNumber(point)) {
        row.push(point)
      } else if (isRecord(point) && isFiniteNumber(point.y)) {
        row.push(point.y)
      } else {
        row.push(null)
      }
    })
    rows.push(row)
  }
  return { headers, rows }
}

interface EnhancedChartCardProps {
  chartData: ChartData
}

export default function EnhancedChartCard({ chartData }: EnhancedChartCardProps) {
  const chartId = useId().replace(/:/g, '_')
  const [isExporting, setIsExporting] = useState(false)
  const [scaleMode, setScaleMode] = useState<ScaleMode>('linear')
  const [renderNonce, setRenderNonce] = useState(0)

  const { options: baseOptions, chartType, title, warnings } = useMemo(
    () => deriveChartState(chartData as RichChartData),
    [chartData]
  )
  const series = Array.isArray(baseOptions.series) ? baseOptions.series : []
  const yValues = useMemo(() => extractYValues(series, chartType), [series, chartType])
  const hasData = hasRenderableData(series, chartType)
  const canUseLog =
    !isPieLike(chartType) &&
    yValues.length >= 2 &&
    yValues.every((value) => value > 0) &&
    Math.max(...yValues) / Math.min(...yValues) >= 10
  const storageKey = getScaleStorageKey(title, chartType)

  useEffect(() => {
    if (!canUseLog) {
      setScaleMode('linear')
      return
    }
    const saved = readScale(storageKey)
    if (saved) setScaleMode(saved)
  }, [canUseLog, storageKey])

  useEffect(() => {
    if (canUseLog) persistScale(storageKey, scaleMode)
  }, [canUseLog, storageKey, scaleMode])

  const options = useMemo(() => {
    const next = cloneDeep(baseOptions)
    const chart = isRecord(next.chart) ? { ...next.chart } : {}
    chart.id = chartId
    chart.type = chartType
    if (!isRecord(chart.toolbar)) chart.toolbar = { show: false }
    if (!isRecord(chart.animations)) chart.animations = { enabled: false }
    if (typeof chart.fontFamily !== 'string') chart.fontFamily = 'inherit'
    if (!isFiniteNumber(chart.height)) chart.height = 350
    next.chart = chart

    if (canUseLog) applyScale(next, scaleMode)
    applyFormatter(next, buildMoneyFormatter(chartData, yValues))
    return next
  }, [baseOptions, chartId, chartType, canUseLog, scaleMode, chartData, yValues])

  const fallbackTable = useMemo(() => buildTableRows(series, options, chartType), [series, options, chartType])

  const handleExportPNG = async () => {
    setIsExporting(true)
    try {
      const chart = ApexCharts.getChartByID(chartId)
      if (!chart) return
      const result = await chart.dataURI({ scale: 2 })
      if (!('imgURI' in result)) return
      const link = document.createElement('a')
      link.href = result.imgURI
      link.download = `${title.replace(/[^a-z0-9]/gi, '_').toLowerCase()}_chart.png`
      link.click()
    } catch (error) {
      console.error('Failed to export chart:', error)
    } finally {
      setIsExporting(false)
    }
  }

  if (!hasData) {
    return (
      <div className="rounded-xl border border-gray-200/80 bg-white p-4 shadow-sm dark:border-gray-700/60 dark:bg-gray-800">
        {warnings.length > 0 && (
          <div className="mb-3 rounded-lg border border-amber-200/80 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-800/50 dark:bg-amber-950/20 dark:text-amber-100">
            {warnings.map((warning, idx) => (
              <p key={`${warning}-${idx}`} className={idx === 0 ? '' : 'mt-1'}>
                {warning}
              </p>
            ))}
          </div>
        )}
        <p className="text-sm text-gray-500 dark:text-gray-400">
          {title && <span className="font-medium">{title}: </span>}
          No chart data available.
        </p>
      </div>
    )
  }

  return (
    <div className="group/chart rounded-xl border border-gray-200/80 bg-white p-4 shadow-sm transition-shadow duration-200 hover:shadow dark:border-gray-700/60 dark:bg-gray-800">
      {warnings.length > 0 && (
        <div className="mb-3 rounded-lg border border-amber-200/80 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-800/50 dark:bg-amber-950/20 dark:text-amber-100">
          {warnings.map((warning, idx) => (
            <p key={`${warning}-${idx}`} className={idx === 0 ? '' : 'mt-1'}>
              {warning}
            </p>
          ))}
        </div>
      )}

      <ErrorBoundary
        fallback={(error, reset) => (
          <div className="space-y-3 rounded-xl border border-amber-200/80 bg-amber-50/80 p-3 text-sm text-amber-900 dark:border-amber-800/40 dark:bg-amber-950/20 dark:text-amber-100">
            <div className="flex items-start justify-between gap-3">
              <p className="leading-relaxed">
                Chart preview failed{error?.message ? `: ${error.message}` : ''}. Showing table below.
              </p>
              <button
                type="button"
                onClick={() => {
                  setRenderNonce((value) => value + 1)
                  reset?.()
                }}
                className="rounded-lg border border-amber-300/80 px-2 py-1 text-xs font-medium text-amber-900 transition-colors hover:bg-amber-100 dark:border-amber-700 dark:text-amber-100 dark:hover:bg-amber-900/40"
              >
                Retry
              </button>
            </div>
            {fallbackTable && (
              <div className="overflow-auto rounded-lg border border-amber-200/80 bg-white dark:border-amber-800/50 dark:bg-gray-900">
                <table className="min-w-full text-xs">
                  <thead>
                    <tr className="bg-amber-100/60 dark:bg-amber-900/30">
                      {fallbackTable.headers.map((header, idx) => (
                        <th
                          key={`${header}-${idx}`}
                          className="whitespace-nowrap border-b border-amber-200 px-2 py-1 text-left font-semibold dark:border-amber-800/50"
                        >
                          {header}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {fallbackTable.rows.map((row, rowIdx) => (
                      <tr key={rowIdx} className={rowIdx % 2 ? 'bg-white dark:bg-gray-900' : 'bg-gray-50/70 dark:bg-gray-900/70'}>
                        {row.map((cell, cellIdx) => (
                          <td
                            key={`${rowIdx}-${cellIdx}`}
                            className="whitespace-nowrap border-b border-gray-100 px-2 py-1 text-gray-700 dark:border-gray-800 dark:text-gray-200"
                          >
                            {cell ?? '-'}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}
      >
        <div className="w-full min-w-0">
          <ReactApexChart
            key={`${chartId}-${renderNonce}`}
            options={options as never}
            series={series as never}
            type={chartType as never}
            width="100%"
            height={isRecord(options.chart) && options.chart.height ? (options.chart.height as number) : 350}
          />
        </div>
      </ErrorBoundary>

      <div className="flex items-center justify-end gap-2 pt-2">
        {canUseLog && (
          <div className="inline-flex items-center rounded-lg border border-gray-200 p-0.5 text-xs dark:border-gray-700">
            <button
              type="button"
              onClick={() => setScaleMode('linear')}
              className={`rounded-md px-2 py-1 transition-colors ${
                scaleMode === 'linear'
                  ? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900'
                  : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700'
              }`}
            >
              Linear
            </button>
            <button
              type="button"
              onClick={() => setScaleMode('log')}
              className={`rounded-md px-2 py-1 transition-colors ${
                scaleMode === 'log'
                  ? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900'
                  : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700'
              }`}
            >
              Log
            </button>
          </div>
        )}

        <button
          type="button"
          onClick={handleExportPNG}
          disabled={isExporting}
          className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium text-gray-400 opacity-0 transition-all duration-150 hover:bg-gray-100 hover:text-gray-600 focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 group-hover/chart:opacity-100 disabled:opacity-50 dark:text-gray-500 dark:hover:bg-gray-700 dark:hover:text-gray-300"
          title="Download chart"
        >
          {isExporting ? (
            <span className="text-gray-500 dark:text-gray-400">Exporting...</span>
          ) : (
            <Fragment>
              <DownloadSimple className="h-3.5 w-3.5" weight="bold" />
              <span>Download PNG</span>
            </Fragment>
          )}
        </button>
      </div>
    </div>
  )
}
