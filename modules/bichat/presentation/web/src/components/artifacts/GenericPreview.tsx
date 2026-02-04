import type { Artifact } from '../../types/artifacts'
import { Info } from '@phosphor-icons/react'

interface GenericPreviewProps {
  artifact: Artifact
}

export function GenericPreview({ artifact }: GenericPreviewProps) {
  return (
    <div className="space-y-4">
      <div className="flex items-start gap-3 p-4 bg-blue-50 border border-blue-200 rounded-lg">
        <Info className="w-5 h-5 text-blue-600 flex-shrink-0 mt-0.5" weight="duotone" />
        <div className="flex-1">
          <p className="text-sm font-medium text-blue-900 mb-1">
            {artifact.name}
          </p>
          {artifact.description && (
            <p className="text-sm text-blue-700">{artifact.description}</p>
          )}
        </div>
      </div>

      {artifact.metadata && Object.keys(artifact.metadata).length > 0 && (
        <details className="text-sm">
          <summary className="cursor-pointer text-gray-700 font-medium mb-2">
            Metadata
          </summary>
          <pre className="bg-gray-50 p-3 rounded border border-gray-200 overflow-auto text-xs">
            {JSON.stringify(artifact.metadata, null, 2)}
          </pre>
        </details>
      )}

      <div className="text-xs text-gray-500 space-y-1">
        <div>Type: {artifact.type}</div>
        {artifact.mimeType && <div>MIME Type: {artifact.mimeType}</div>}
        <div>Created: {new Date(artifact.createdAt).toLocaleString()}</div>
      </div>
    </div>
  )
}
