import type { Artifact } from '../../types/artifacts'
import { WarningCircle } from '@phosphor-icons/react'

interface ImagePreviewProps {
  artifact: Artifact
}

export function ImagePreview({ artifact }: ImagePreviewProps) {
  if (!artifact.url) {
    return (
      <div className="flex items-center gap-2 p-4 bg-yellow-50 border border-yellow-200 rounded-lg">
        <WarningCircle className="w-5 h-5 text-yellow-600" weight="duotone" />
        <span className="text-sm text-yellow-800">Image URL not available</span>
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <div className="bg-gray-50 rounded-lg p-4 border border-gray-200">
        <img
          src={artifact.url}
          alt={artifact.name}
          className="max-w-full h-auto rounded"
          loading="lazy"
        />
      </div>
      {artifact.description && (
        <p className="text-sm text-gray-600">{artifact.description}</p>
      )}
    </div>
  )
}
