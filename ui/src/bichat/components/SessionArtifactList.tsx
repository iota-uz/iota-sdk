import { type ReactNode, useState } from 'react'
import {
  ChartBar,
  Code,
  FileCsv,
  Image as ImageIcon,
  Package,
} from '@phosphor-icons/react'
import type { SessionArtifact } from '../types'
import { useTranslation } from '../hooks/useTranslation'
import { formatFileSize, getFileVisual, CHART_VISUAL, type FileVisual } from '../utils/fileUtils'

interface SessionArtifactListProps {
  artifacts: SessionArtifact[]
  selectedArtifactId?: string
  onSelect: (artifact: SessionArtifact) => void
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

function isImageArtifact(artifact: SessionArtifact): boolean {
  const mime = artifact.mimeType?.toLowerCase() || ''
  const name = artifact.name.toLowerCase()
  return mime.startsWith('image/') || /\.(png|jpe?g|gif|webp|svg|bmp)$/.test(name)
}

function ImageThumbnail({ src, alt }: { src: string; alt: string }) {
  const [failed, setFailed] = useState(false)
  if (failed) {
    return (
      <div className="w-full aspect-video rounded-lg bg-violet-50 dark:bg-violet-900/30 flex items-center justify-center">
        <ImageIcon className="h-6 w-6 text-violet-400 dark:text-violet-500" weight="duotone" />
      </div>
    )
  }
  return (
    <img
      src={src}
      alt={alt}
      onError={() => setFailed(true)}
      className="w-full rounded-lg object-cover max-h-32 bg-gray-100 dark:bg-gray-800"
    />
  )
}

function getArtifactFileVisual(artifact: SessionArtifact): FileVisual {
  if (artifact.type === 'chart') return CHART_VISUAL
  if (artifact.type === 'code_output') {
    const v = getFileVisual(artifact.mimeType, artifact.name)
    // Code outputs get a sky accent unless they resolve to something specific (image, etc.)
    if (v.label === 'TEXT' || v.label === 'FILE') {
      return { ...v, iconColor: 'text-sky-600 dark:text-sky-400', bgColor: 'bg-sky-100 dark:bg-sky-900/40' }
    }
    return v
  }
  return getFileVisual(artifact.mimeType, artifact.name)
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
              const visual = getArtifactFileVisual(artifact)
              const Icon = visual.icon
              return (
                <button
                  key={artifact.id}
                  type="button"
                  onClick={() => onSelect(artifact)}
                  className={`cursor-pointer group/item w-full rounded-lg border px-3 py-2 text-left transition-all duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 ${
                    isSelected
                      ? 'border-primary-200 bg-primary-50/80 shadow-sm dark:border-primary-800/60 dark:bg-primary-950/40'
                      : 'border-transparent bg-white hover:border-gray-200 hover:bg-gray-50 hover:shadow-sm dark:bg-gray-900 dark:hover:border-gray-700/80 dark:hover:bg-gray-800/60'
                  }`}
                >
                  {isImageArtifact(artifact) && artifact.url ? (
                    <div>
                      <ImageThumbnail src={artifact.url} alt={artifact.name} />
                      <div className="mt-2">
                        <span className="block truncate text-[13px] font-medium text-gray-900 dark:text-gray-100">
                          {artifact.name}
                        </span>
                        <span className="flex items-center gap-1.5 text-[11px] text-gray-400 dark:text-gray-500">
                          <span>{formatFileSize(artifact.sizeBytes)}</span>
                          {artifact.description && (
                            <>
                              <span className="w-0.5 h-0.5 rounded-full bg-gray-300 dark:bg-gray-600" />
                              <span className="truncate">{artifact.description}</span>
                            </>
                          )}
                        </span>
                      </div>
                    </div>
                  ) : (
                    <div className="flex items-center gap-2.5">
                      <span className={`flex-shrink-0 flex items-center justify-center w-10 h-10 rounded-lg ${visual.bgColor} ${visual.iconColor}`}>
                        <Icon size={20} weight="duotone" />
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-[13px] font-medium text-gray-900 dark:text-gray-100">
                          {artifact.name}
                        </span>
                        <span className="flex items-center gap-1.5 text-[11px] text-gray-400 dark:text-gray-500">
                          <span>{formatFileSize(artifact.sizeBytes)}</span>
                          {artifact.description && (
                            <>
                              <span className="w-0.5 h-0.5 rounded-full bg-gray-300 dark:bg-gray-600" />
                              <span className="truncate">{artifact.description}</span>
                            </>
                          )}
                        </span>
                      </span>
                    </div>
                  )}
                </button>
              )
            })}
          </div>
        </section>
      ))}
    </div>
  )
}
