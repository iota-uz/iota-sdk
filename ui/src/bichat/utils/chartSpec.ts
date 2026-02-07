/**
 * Shared chart spec parsing for draw_chart tool output and artifact metadata.
 * Used by SessionArtifactPreview, HttpDataSource (turn.chartData), and MarkdownRenderer (code blocks).
 */

import type { ChartData, ChartSeries } from '../types'

export const SUPPORTED_CHART_TYPES = new Set<ChartData['chartType']>([
  'line',
  'bar',
  'area',
  'pie',
  'donut',
])

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function toChartSeries(value: unknown): ChartSeries[] | null {
  if (!Array.isArray(value) || value.length === 0) {
    return null
  }

  const result: ChartSeries[] = []
  for (const item of value) {
    if (!isRecord(item) || typeof item.name !== 'string' || !Array.isArray(item.data)) {
      return null
    }

    const data: number[] = []
    for (const point of item.data) {
      if (typeof point !== 'number' || !Number.isFinite(point)) {
        return null
      }
      data.push(point)
    }

    result.push({
      name: item.name,
      data,
    })
  }

  return result.length > 0 ? result : null
}

/**
 * Parses a chart spec object (e.g. from artifact.metadata.spec or draw_chart tool output) into ChartData.
 */
export function parseChartDataFromSpec(
  spec: Record<string, unknown>,
  fallbackTitle = 'Chart'
): ChartData | null {
  const chartTypeRaw = spec.chartType
  const titleRaw = spec.title
  const seriesRaw = spec.series

  if (
    typeof chartTypeRaw !== 'string' ||
    !SUPPORTED_CHART_TYPES.has(chartTypeRaw as ChartData['chartType'])
  ) {
    return null
  }

  const series = toChartSeries(seriesRaw)
  if (!series) {
    return null
  }

  const labels = Array.isArray(spec.labels)
    ? spec.labels.filter((label): label is string => typeof label === 'string')
    : undefined

  const colors = Array.isArray(spec.colors)
    ? spec.colors.filter((color): color is string => typeof color === 'string')
    : undefined

  const height =
    typeof spec.height === 'number' && Number.isFinite(spec.height) ? spec.height : undefined

  return {
    chartType: chartTypeRaw as ChartData['chartType'],
    title: typeof titleRaw === 'string' && titleRaw.trim() ? titleRaw : fallbackTitle,
    series,
    labels,
    colors,
    height,
  }
}

/**
 * Parses a JSON string as a chart spec (e.g. from a ```chart or ```json code block).
 */
export function parseChartDataFromJsonString(
  json: string,
  fallbackTitle = 'Chart'
): ChartData | null {
  const trimmed = json.trim()
  if (!trimmed) return null

  let spec: unknown
  try {
    spec = JSON.parse(trimmed)
  } catch {
    return null
  }

  if (!isRecord(spec)) return null
  return parseChartDataFromSpec(spec, fallbackTitle)
}
