import { WarningCircle, DownloadSimple, Calendar, HardDrive, Tag, File } from '@phosphor-icons/react'
import type { ChartData, ChartSeries, SessionArtifact } from '../types'
import { ChartCard } from './ChartCard'
import { useTranslation } from '../hooks/useTranslation'

interface SessionArtifactPreviewProps {
  artifact: SessionArtifact
}

const SUPPORTED_CHART_TYPES = new Set<ChartData['chartType']>(['line', 'bar', 'area', 'pie', 'donut'])

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

function parseChartDataFromArtifact(artifact: SessionArtifact): ChartData | null {
  const metadata = artifact.metadata
  if (!metadata || !isRecord(metadata)) {
    return null
  }

  const candidate = isRecord(metadata.spec) ? metadata.spec : metadata
  if (!isRecord(candidate)) {
    return null
  }

  const chartTypeRaw = candidate.chartType
  const titleRaw = candidate.title
  const seriesRaw = candidate.series

  if (typeof chartTypeRaw !== 'string' || !SUPPORTED_CHART_TYPES.has(chartTypeRaw as ChartData['chartType'])) {
    return null
  }

  const series = toChartSeries(seriesRaw)
  if (!series) {
    return null
  }

  const labels = Array.isArray(candidate.labels)
    ? candidate.labels.filter((label): label is string => typeof label === 'string')
    : undefined

  const colors = Array.isArray(candidate.colors)
    ? candidate.colors.filter((color): color is string => typeof color === 'string')
    : undefined

  const height = typeof candidate.height === 'number' && Number.isFinite(candidate.height)
    ? candidate.height
    : undefined

  return {
    chartType: chartTypeRaw as ChartData['chartType'],
    title: typeof titleRaw === 'string' && titleRaw.trim() ? titleRaw : artifact.name,
    series,
    labels,
    colors,
    height,
  }
}

function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return '0 B'
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx++
  }

  const precision = idx === 0 ? 0 : value >= 10 ? 1 : 2
  return `${value.toFixed(precision)} ${units[idx]}`
}

function ArtifactMetadata({ artifact }: { artifact: SessionArtifact }) {
  const { t } = useTranslation()

  const items = [
    { icon: Tag, label: t('artifacts.typeLabel'), value: artifact.type },
    artifact.mimeType ? { icon: File, label: t('artifacts.mimeTypeLabel'), value: artifact.mimeType } : null,
    { icon: HardDrive, label: t('artifacts.sizeLabel'), value: formatFileSize(artifact.sizeBytes) },
    {
      icon: Calendar,
      label: t('artifacts.createdLabel'),
      value: new Date(artifact.createdAt).toLocaleString(),
    },
  ].filter(Boolean) as Array<{ icon: typeof Tag; label: string; value: string }>

  return (
    <div className="rounded-xl border border-gray-200/80 bg-gray-50/80 p-3 dark:border-gray-700/60 dark:bg-gray-800/40">
      <div className="grid gap-2.5">
        {items.map((item) => (
          <div key={item.label} className="flex items-center gap-2 text-xs">
            <item.icon className="h-3.5 w-3.5 shrink-0 text-gray-400 dark:text-gray-500" weight="duotone" />
            <span className="shrink-0 font-medium text-gray-500 dark:text-gray-400">{item.label}</span>
            <span className="ml-auto truncate text-gray-700 dark:text-gray-300">{item.value}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

function GenericMetadataBlock({ artifact }: { artifact: SessionArtifact }) {
  const { t } = useTranslation()

  if (!artifact.metadata || Object.keys(artifact.metadata).length === 0) {
    return null
  }

  return (
    <details className="group rounded-xl border border-gray-200/80 bg-white p-3 dark:border-gray-700/60 dark:bg-gray-900">
      <summary className="flex cursor-pointer items-center gap-1.5 text-xs font-medium text-gray-600 transition-colors hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-200">
        <svg
          className="h-3 w-3 shrink-0 transition-transform duration-150 group-open:rotate-90"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
        </svg>
        {t('artifacts.metadata')}
      </summary>
      <pre className="mt-2.5 overflow-auto rounded-lg bg-gray-50 p-2.5 text-[11px] leading-relaxed text-gray-700 dark:bg-gray-800 dark:text-gray-200">
        {JSON.stringify(artifact.metadata, null, 2)}
      </pre>
    </details>
  )
}

function WarningBox({ message }: { message: string }) {
  return (
    <div className="flex items-start gap-2.5 rounded-xl border border-amber-200/80 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800/40 dark:bg-amber-950/20 dark:text-amber-200">
      <WarningCircle className="mt-0.5 h-4 w-4 shrink-0" weight="duotone" />
      <span className="leading-relaxed">{message}</span>
    </div>
  )
}

export function SessionArtifactPreview({ artifact }: SessionArtifactPreviewProps) {
  const { t } = useTranslation()

  if (artifact.type === 'chart') {
    const chartData = parseChartDataFromArtifact(artifact)
    if (chartData) {
      return (
        <div className="space-y-3">
          <ChartCard chartData={chartData} />
          <ArtifactMetadata artifact={artifact} />
        </div>
      )
    }

    return (
      <div className="space-y-3">
        <WarningBox message={t('artifacts.chartUnavailable')} />
        <GenericMetadataBlock artifact={artifact} />
        <ArtifactMetadata artifact={artifact} />
      </div>
    )
  }

  if (
    (artifact.type === 'code_output' || artifact.type === 'attachment') &&
    artifact.mimeType?.startsWith('image/')
  ) {
    if (!artifact.url) {
      return <WarningBox message={t('artifacts.imageUnavailable')} />
    }

    return (
      <div className="space-y-3">
        <div className="overflow-hidden rounded-xl border border-gray-200/80 bg-gray-50/50 dark:border-gray-700/60 dark:bg-gray-800/30">
          <img
            src={artifact.url}
            alt={artifact.name}
            className="h-auto w-full"
            loading="lazy"
          />
        </div>
        {artifact.description && (
          <p className="px-0.5 text-xs leading-relaxed text-gray-500 dark:text-gray-400">{artifact.description}</p>
        )}
        <ArtifactMetadata artifact={artifact} />
      </div>
    )
  }

  if (artifact.url) {
    return (
      <div className="space-y-3">
        <div className="rounded-xl border border-gray-200/80 bg-white p-4 dark:border-gray-700/60 dark:bg-gray-900">
          <h3 className="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">{artifact.name}</h3>
          {artifact.description && (
            <p className="mt-1.5 text-sm leading-relaxed text-gray-500 dark:text-gray-400">{artifact.description}</p>
          )}
          <a
            href={artifact.url}
            target="_blank"
            rel="noreferrer"
            className="mt-4 inline-flex items-center gap-2 rounded-lg bg-primary-600 px-4 py-2 text-xs font-medium text-white shadow-sm transition-all duration-150 hover:bg-primary-700 hover:shadow focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 active:translate-y-px"
          >
            <DownloadSimple className="h-3.5 w-3.5" weight="bold" />
            {t('artifacts.download')}
          </a>
        </div>
        <ArtifactMetadata artifact={artifact} />
        <GenericMetadataBlock artifact={artifact} />
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <div className="rounded-xl border border-gray-200/80 bg-white p-4 dark:border-gray-700/60 dark:bg-gray-900">
        <h3 className="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">{artifact.name}</h3>
        {artifact.description && (
          <p className="mt-1.5 text-sm leading-relaxed text-gray-500 dark:text-gray-400">{artifact.description}</p>
        )}
        <p className="mt-3 text-xs text-gray-400 dark:text-gray-500">{t('artifacts.downloadUnavailable')}</p>
      </div>
      <ArtifactMetadata artifact={artifact} />
      <GenericMetadataBlock artifact={artifact} />
    </div>
  )
}
