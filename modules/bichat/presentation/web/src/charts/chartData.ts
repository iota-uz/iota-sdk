import type { ChartData, ConversationTurn, SessionArtifact } from '@iota-uz/sdk/bichat'

export type RichChartData = ChartData & {
  options?: Record<string, unknown>
  warnings?: string[]
  meta?: Record<string, unknown>
}

type LegacyChartType = ChartData['chartType']

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function toNumber(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  return null
}

function toStringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) return undefined
  const out = value.filter((item): item is string => typeof item === 'string')
  return out.length > 0 ? out : undefined
}

// Custom clone for JSON-derived chart data; structuredClone() not used because
// the ES2020/browser target may lack structuredClone support.
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

function normalizeWarnings(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) return undefined
  const warnings = value.filter((v): v is string => typeof v === 'string' && v.trim().length > 0)
  return warnings.length > 0 ? warnings : undefined
}

function toLegacyChartType(chartType: string): LegacyChartType {
  switch (chartType) {
    case 'line':
    case 'area':
    case 'bar':
    case 'pie':
    case 'donut':
      return chartType
    default:
      return 'line'
  }
}

function inferTitleFromOptions(options: Record<string, unknown>, fallback: string): string {
  const titleRaw = options.title
  if (!isRecord(titleRaw)) return fallback
  const text = titleRaw.text
  if (typeof text !== 'string') return fallback
  const trimmed = text.trim()
  return trimmed || fallback
}

function inferChartType(options: Record<string, unknown>): string | null {
  const chartRaw = options.chart
  if (!isRecord(chartRaw)) return null
  const rawType = chartRaw.type
  if (typeof rawType !== 'string') return null
  const trimmed = rawType.trim().toLowerCase()
  return trimmed || null
}

function ensureApexChartData(
  optionsInput: Record<string, unknown>,
  fallbackTitle: string,
  warnings?: string[],
  meta?: Record<string, unknown>
): RichChartData | null {
  const options = cloneDeep(optionsInput)
  const chartType = inferChartType(options)
  if (!chartType) return null

  const optionsSeries = options.series
  if (!Array.isArray(optionsSeries) || optionsSeries.length === 0) return null

  const title = inferTitleFromOptions(options, fallbackTitle)
  const legacySpec = optionsToLegacySpec(options, chartType, title)
  const series = Array.isArray(legacySpec?.series)
    ? (legacySpec.series as ChartData['series'])
    : ([] as ChartData['series'])
  const chartData: RichChartData = {
    chartType: toLegacyChartType(chartType),
    title,
    series,
    options,
    warnings,
    meta,
  }
  return chartData
}

function seriesToLegacy(
  seriesRaw: unknown
): Array<{ name: string; data: number[] }> | null {
  if (!Array.isArray(seriesRaw) || seriesRaw.length === 0) return null
  const out: Array<{ name: string; data: number[] }> = []
  seriesRaw.forEach((item, idx) => {
    if (!isRecord(item)) return
    const nameRaw = item.name
    const dataRaw = item.data
    if (!Array.isArray(dataRaw)) return
    const data: number[] = []
    dataRaw.forEach((point) => {
      const num = toNumber(point)
      if (num !== null) {
        data.push(num)
        return
      }
      if (isRecord(point)) {
        const y = toNumber(point.y)
        if (y !== null) data.push(y)
      }
    })
    if (data.length === 0) return
    out.push({
      name: typeof nameRaw === 'string' && nameRaw.trim() ? nameRaw : `Series ${idx + 1}`,
      data,
    })
  })
  return out.length > 0 ? out : null
}

function optionsToLegacySpec(
  options: Record<string, unknown>,
  chartType: string,
  title: string
): Record<string, unknown> | null {
  const isPieLike =
    chartType === 'pie' ||
    chartType === 'donut' ||
    chartType === 'polararea' ||
    chartType === 'radialbar'

  let legacySeries: Array<{ name: string; data: number[] }> | null = null
  if (isPieLike) {
    if (Array.isArray(options.series)) {
      const numeric = options.series
        .map((v) => toNumber(v))
        .filter((v): v is number => v !== null)
      if (numeric.length > 0) {
        legacySeries = [{ name: title || 'Series 1', data: numeric }]
      } else {
        legacySeries = seriesToLegacy(options.series)
      }
    }
  } else {
    legacySeries = seriesToLegacy(options.series)
  }

  if (!legacySeries || legacySeries.length === 0) return null

  const legacy: Record<string, unknown> = {
    chartType,
    title,
    series: legacySeries,
  }

  const labelsFromOptions = toStringArray(options.labels)
  if (labelsFromOptions && labelsFromOptions.length > 0) {
    legacy.labels = labelsFromOptions
  } else if (isRecord(options.xaxis)) {
    const categories = toStringArray(options.xaxis.categories)
    if (categories && categories.length > 0) legacy.labels = categories
  }

  const colors = toStringArray(options.colors)
  if (colors && colors.length > 0) legacy.colors = colors

  if (isRecord(options.chart)) {
    const heightRaw = options.chart.height
    if (typeof heightRaw === 'number' && Number.isFinite(heightRaw)) {
      legacy.height = heightRaw
    }
  }

  return legacy
}

function parseFromLegacySpec(
  spec: Record<string, unknown>,
  fallbackTitle: string
): RichChartData | null {
  const chartTypeRaw = spec.chartType
  if (typeof chartTypeRaw !== 'string' || !chartTypeRaw.trim()) return null
  const chartType = chartTypeRaw.trim().toLowerCase()

  const series = seriesToLegacy(spec.series)
  if (!series) return null

  const titleRaw = spec.title
  const title = typeof titleRaw === 'string' && titleRaw.trim() ? titleRaw.trim() : fallbackTitle
  const labels = toStringArray(spec.labels)
  const colors = toStringArray(spec.colors)
  const height = typeof spec.height === 'number' && Number.isFinite(spec.height) ? spec.height : undefined

  const options: Record<string, unknown> = {
    chart: {
      type: chartType,
      ...(height ? { height } : {}),
    },
    title: { text: title },
    series,
  }
  if (labels && labels.length > 0) {
    if (chartType === 'pie' || chartType === 'donut') {
      options.labels = labels
      options.series = series[0]?.data ?? []
    } else {
      options.xaxis = { categories: labels }
    }
  }
  if (colors && colors.length > 0) options.colors = colors

  return {
    chartType: toLegacyChartType(chartType),
    title,
    series: series as ChartData['series'],
    options,
    warnings: normalizeWarnings(spec.warnings),
    meta: isRecord(spec.meta) ? spec.meta : undefined,
    height,
  }
}

function parseChartDataFromMetadata(
  metadata: Record<string, unknown>,
  fallbackTitle: string
): RichChartData | null {
  const warnings = normalizeWarnings(metadata.warnings)
  const meta = isRecord(metadata.meta) ? metadata.meta : undefined

  const richSpec = isRecord(metadata.richSpec) ? metadata.richSpec : undefined
  if (richSpec) {
    const richParsed = ensureApexChartData(richSpec, fallbackTitle, warnings, meta)
    if (richParsed) return richParsed
  }

  const specCandidate = isRecord(metadata.spec) ? metadata.spec : metadata
  const optionsCandidate = isRecord(specCandidate.options) ? specCandidate.options : specCandidate

  const apexParsed = ensureApexChartData(optionsCandidate, fallbackTitle, warnings, meta)
  if (apexParsed) return apexParsed

  return parseFromLegacySpec(specCandidate, fallbackTitle)
}

export function parseChartDataFromArtifact(artifact: SessionArtifact): RichChartData | null {
  if (!artifact.metadata || !isRecord(artifact.metadata)) return null
  return parseChartDataFromMetadata(artifact.metadata, artifact.name || 'Chart')
}

export function normalizeChartArtifactsForSdk(artifacts: SessionArtifact[]): SessionArtifact[] {
  return artifacts.map((artifact) => {
    if (artifact.type !== 'chart' || !artifact.metadata || !isRecord(artifact.metadata)) {
      return artifact
    }
    const chartData = parseChartDataFromArtifact(artifact)
    if (!chartData?.options) {
      return artifact
    }
    const legacySpec = optionsToLegacySpec(chartData.options, chartData.chartType, chartData.title)
    if (!legacySpec) {
      return artifact
    }
    return {
      ...artifact,
      metadata: {
        ...artifact.metadata,
        // Keep rich source for enhanced renderer while preserving SDK legacy parse paths.
        richSpec: chartData.options,
        warnings: chartData.warnings,
        meta: chartData.meta,
        spec: legacySpec,
      },
    }
  })
}

function toMillis(value: string): number {
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) ? parsed : Number.NaN
}

// Artifacts are emitted during tool execution before the assistant turn finalizes,
// so we match the first assistant turn whose createdAt is >= artifactCreatedAt ("at or after").
function findAssistantIndexByFallback(turns: ConversationTurn[], artifactCreatedAt: string): number | null {
  const assistantPositions = turns
    .map((turn, index) => ({
      index,
      createdAtMs: toMillis(turn.assistantTurn?.createdAt || turn.createdAt),
      hasAssistant: Boolean(turn.assistantTurn),
    }))
    .filter((entry) => entry.hasAssistant)

  if (assistantPositions.length === 0) return null
  const artifactMs = toMillis(artifactCreatedAt)
  if (!Number.isFinite(artifactMs)) return assistantPositions[assistantPositions.length - 1].index

  const found = assistantPositions.find(
    (entry) => Number.isFinite(entry.createdAtMs) && entry.createdAtMs >= artifactMs
  )
  return found ? found.index : assistantPositions[assistantPositions.length - 1].index
}

export function attachRichChartDataToTurns(
  turns: ConversationTurn[],
  artifacts: SessionArtifact[]
): ConversationTurn[] {
  if (turns.length === 0 || artifacts.length === 0) return turns

  const nextTurns = turns.map((turn) => ({
    ...turn,
    assistantTurn: turn.assistantTurn
      ? {
          ...turn.assistantTurn,
        }
      : undefined,
  }))

  const turnIndexByMessageId = new Map<string, number>()
  nextTurns.forEach((turn, idx) => {
    turnIndexByMessageId.set(turn.userTurn.id, idx)
    if (turn.assistantTurn?.id) {
      turnIndexByMessageId.set(turn.assistantTurn.id, idx)
    }
  })

  const chartArtifacts = artifacts
    .filter((artifact) => artifact.type === 'chart')
    .sort((a, b) => toMillis(a.createdAt) - toMillis(b.createdAt))

  chartArtifacts.forEach((artifact) => {
    const chartData = parseChartDataFromArtifact(artifact)
    if (!chartData) return

    const directIndex = artifact.messageId ? turnIndexByMessageId.get(artifact.messageId) : undefined
    const targetIndex =
      directIndex !== undefined ? directIndex : findAssistantIndexByFallback(nextTurns, artifact.createdAt)
    if (targetIndex === null) return

    const assistantTurn = nextTurns[targetIndex]?.assistantTurn
    if (!assistantTurn) return

    if (!assistantTurn.charts) assistantTurn.charts = []
    assistantTurn.charts.push(chartData)
  })

  return nextTurns
}
