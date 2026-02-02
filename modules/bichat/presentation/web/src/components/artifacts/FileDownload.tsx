import type { Artifact } from '../../types/artifacts'
import { DownloadSimple, WarningCircle } from '@phosphor-icons/react'

interface FileDownloadProps {
  artifact: Artifact
}

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
}

export function FileDownload({ artifact }: FileDownloadProps) {
  if (!artifact.url) {
    return (
      <div className="flex items-center gap-2 p-4 bg-yellow-50 border border-yellow-200 rounded-lg">
        <WarningCircle className="w-5 h-5 text-yellow-600" weight="duotone" />
        <span className="text-sm text-yellow-800">Download URL not available</span>
      </div>
    )
  }

  const handleDownload = () => {
    window.open(artifact.url, '_blank')
  }

  return (
    <div className="space-y-4">
      <div className="border border-gray-200 rounded-lg p-4">
        <div className="flex items-start justify-between gap-4">
          <div className="flex-1 min-w-0">
            <h3 className="text-sm font-medium text-gray-900 mb-1">
              {artifact.name}
            </h3>
            {artifact.description && (
              <p className="text-sm text-gray-600 mb-2">{artifact.description}</p>
            )}
            <div className="flex items-center gap-4 text-xs text-gray-500">
              <span>Size: {formatFileSize(artifact.sizeBytes)}</span>
              {artifact.mimeType && <span>Type: {artifact.mimeType}</span>}
            </div>
            {artifact.metadata?.rowCount && (
              <div className="text-xs text-gray-500 mt-1">
                Rows: {artifact.metadata.rowCount.toLocaleString()}
              </div>
            )}
          </div>
          <button
            onClick={handleDownload}
            className="flex-shrink-0 flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-sm font-medium"
          >
            <DownloadSimple className="w-4 h-4" weight="bold" />
            Download
          </button>
        </div>
      </div>
    </div>
  )
}
