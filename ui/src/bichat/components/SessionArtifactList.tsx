import type { ReactNode } from 'react'
import {
  ChartBar,
  Code,
  File,
  FileCsv,
  FilePdf,
  FileText,
  Image as ImageIcon,
  Package,
} from '@phosphor-icons/react'
import type { SessionArtifact } from '../types'
import { useTranslation } from '../hooks/useTranslation'

interface SessionArtifactListProps {
  artifacts: SessionArtifact[]
  selectedArtifactId?: string
  onSelect: (artifact: SessionArtifact) => void
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

const TYPE_LABEL_KEYS: Record<string, string> = {
  chart: 'artifacts.groupCharts',
  code_output: 'artifacts.groupCodeOutputs',
  export: 'artifacts.groupExports',
  attachment: 'artifacts.groupAttachments',
  other: 'artifacts.groupOther',
}

function getGroupIcon(type: string): ReactNode {
  const cls = 'h-3.5 w-3.5'
  switch (type) {
    case 'chart':
      return <ChartBar className={cls} weight="bold" />
    case 'code_output':
      return <Code className={cls} weight="bold" />
    case 'export':
      return <FileCsv className={cls} weight="bold" />
    case 'attachment':
      return <ImageIcon className={cls} weight="bold" />
    default:
      return <Package className={cls} weight="bold" />
  }
}

function getArtifactIcon(artifact: SessionArtifact): ReactNode {
  const iconClass = 'h-4 w-4'
  const mime = artifact.mimeType?.toLowerCase() || ''
  const name = artifact.name.toLowerCase()

  if (artifact.type === 'chart') {
    return <ChartBar className={iconClass} weight="duotone" />
  }

  if (artifact.type === 'code_output') {
    if (mime.startsWith('image/')) {
      return <ImageIcon className={iconClass} weight="duotone" />
    }
    if (mime.includes('json')) {
      return <Code className={iconClass} weight="duotone" />
    }
    return <FileText className={iconClass} weight="duotone" />
  }

  if (artifact.type === 'export') {
    if (
      mime === 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' ||
      name.endsWith('.xlsx') ||
      name.endsWith('.xls')
    ) {
      return <FileCsv className={iconClass} weight="duotone" />
    }

    if (mime === 'application/pdf' || name.endsWith('.pdf')) {
      return <FilePdf className={iconClass} weight="duotone" />
    }
  }

  return <File className={iconClass} weight="duotone" />
}

function getArtifactAccent(artifact: SessionArtifact): string {
  const mime = artifact.mimeType?.toLowerCase() || ''
  const name = artifact.name.toLowerCase()

  if (artifact.type === 'chart') return 'text-indigo-500 dark:text-indigo-400'
  if (artifact.type === 'code_output') {
    if (mime.startsWith('image/')) return 'text-violet-500 dark:text-violet-400'
    return 'text-sky-500 dark:text-sky-400'
  }
  if (artifact.type === 'export') {
    if (
      mime === 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' ||
      name.endsWith('.xlsx') ||
      name.endsWith('.xls')
    ) {
      return 'text-emerald-500 dark:text-emerald-400'
    }
    if (mime === 'application/pdf' || name.endsWith('.pdf')) {
      return 'text-rose-500 dark:text-rose-400'
    }
  }
  return 'text-gray-400 dark:text-gray-500'
}

function groupArtifactsByType(artifacts: SessionArtifact[]): Array<{ type: string; items: SessionArtifact[] }> {
  const grouped = new Map<string, SessionArtifact[]>()

  for (const artifact of artifacts) {
    const type = artifact.type || 'other'
    const existing = grouped.get(type)
    if (existing) {
      existing.push(artifact)
      continue
    }
    grouped.set(type, [artifact])
  }

  return Array.from(grouped.entries())
    .map(([type, items]) => ({
      type,
      items: items.sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt)),
    }))
    .sort((a, b) => a.type.localeCompare(b.type))
}

export function SessionArtifactList({
  artifacts,
  selectedArtifactId,
  onSelect,
}: SessionArtifactListProps) {
  const { t } = useTranslation()
  const grouped = groupArtifactsByType(artifacts)

  if (artifacts.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 px-4 py-12 text-center">
        <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-gray-100 dark:bg-gray-800">
          <Package className="h-6 w-6 text-gray-400 dark:text-gray-500" weight="duotone" />
        </div>
        <div>
          <p className="text-sm font-medium text-gray-500 dark:text-gray-400">
            {t('artifacts.empty')}
          </p>
          <p className="mt-0.5 text-xs text-gray-400 dark:text-gray-500">
            {t('artifacts.emptySubtitle')}
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-5">
      {grouped.map((group) => (
        <section key={group.type}>
          <div className="mb-2 flex items-center gap-1.5 px-0.5">
            <span className="text-gray-400 dark:text-gray-500">{getGroupIcon(group.type)}</span>
            <h3 className="text-[11px] font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
              {TYPE_LABEL_KEYS[group.type] ? t(TYPE_LABEL_KEYS[group.type]) : group.type.replace(/_/g, ' ')}
            </h3>
            <span className="ml-auto text-[10px] tabular-nums text-gray-400 dark:text-gray-500">
              {group.items.length}
            </span>
          </div>
          <div className="space-y-1">
            {group.items.map((artifact) => {
              const isSelected = artifact.id === selectedArtifactId
              return (
                <button
                  key={artifact.id}
                  type="button"
                  onClick={() => onSelect(artifact)}
                  className={`cursor-pointer group/item w-full rounded-lg border px-3 py-2.5 text-left transition-all duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 ${
                    isSelected
                      ? 'border-primary-200 bg-primary-50/80 shadow-sm dark:border-primary-800/60 dark:bg-primary-950/40'
                      : 'border-transparent bg-white hover:border-gray-200 hover:bg-gray-50 hover:shadow-sm dark:bg-gray-900 dark:hover:border-gray-700/80 dark:hover:bg-gray-800/60'
                  }`}
                >
                  <div className="flex items-start gap-2.5">
                    <span className={`mt-0.5 transition-colors duration-150 ${getArtifactAccent(artifact)}`}>
                      {getArtifactIcon(artifact)}
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-[13px] font-medium text-gray-900 dark:text-gray-100">
                        {artifact.name}
                      </span>
                      {artifact.description && (
                        <span className="mt-0.5 block truncate text-xs text-gray-500 dark:text-gray-400">
                          {artifact.description}
                        </span>
                      )}
                      <span className="mt-1 flex items-center gap-1.5 text-[11px] text-gray-400 dark:text-gray-500">
                        <span>{formatFileSize(artifact.sizeBytes)}</span>
                      </span>
                    </span>
                  </div>
                </button>
              )
            })}
          </div>
        </section>
      ))}
    </div>
  )
}
