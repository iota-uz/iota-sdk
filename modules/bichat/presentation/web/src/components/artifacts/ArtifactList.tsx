import type { Artifact } from '../../types/artifacts'
import {
  FileText,
  FileCsv,
  FilePdf,
  Image as ImageIcon,
  ChartBar,
  File,
} from '@phosphor-icons/react'

interface ArtifactListProps {
  artifacts: Artifact[]
  onSelect: (artifact: Artifact) => void
}

function getArtifactIcon(artifact: Artifact): React.ReactNode {
  const iconClass = 'w-5 h-5'

  if (artifact.type === 'chart') {
    return <ChartBar className={iconClass} weight="duotone" />
  }

  if (artifact.type === 'code_output') {
    if (artifact.mimeType?.startsWith('image/')) {
      return <ImageIcon className={iconClass} weight="duotone" />
    }
    return <FileText className={iconClass} weight="duotone" />
  }

  if (artifact.type === 'export') {
    if (artifact.mimeType === 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet') {
      return <FileCsv className={iconClass} weight="duotone" />
    }
    if (artifact.mimeType === 'application/pdf') {
      return <FilePdf className={iconClass} weight="duotone" />
    }
  }

  return <File className={iconClass} weight="duotone" />
}

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
}

function groupArtifactsByType(artifacts: Artifact[]): Record<string, Artifact[]> {
  return artifacts.reduce((acc, artifact) => {
    const type = artifact.type || 'other'
    if (!acc[type]) acc[type] = []
    acc[type].push(artifact)
    return acc
  }, {} as Record<string, Artifact[]>)
}

export function ArtifactList({ artifacts, onSelect }: ArtifactListProps) {
  const grouped = groupArtifactsByType(artifacts)
  const types = Object.keys(grouped).sort()

  if (artifacts.length === 0) {
    return (
      <div className="text-center py-8 text-gray-500 dark:text-gray-400">
        <File className="w-12 h-12 mx-auto mb-2 opacity-50" weight="duotone" />
        <p className="text-sm">No artifacts yet</p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {types.map((type) => (
        <div key={type}>
          <h3 className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase mb-2 px-2">
            {type.replace('_', ' ')}
          </h3>
          <div className="space-y-1">
            {grouped[type].map((artifact) => (
              <button
                key={artifact.id}
                onClick={() => onSelect(artifact)}
                className="w-full flex items-start gap-3 p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800/50 active:bg-gray-200 dark:active:bg-gray-700/50 transition-colors duration-150 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50"
              >
                <div className="flex-shrink-0 mt-0.5 text-gray-600 dark:text-gray-400">
                  {getArtifactIcon(artifact)}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
                    {artifact.name}
                  </div>
                  {artifact.description && (
                    <div className="text-xs text-gray-500 dark:text-gray-400 truncate">
                      {artifact.description}
                    </div>
                  )}
                  <div className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
                    {formatFileSize(artifact.sizeBytes)}
                  </div>
                </div>
              </button>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
