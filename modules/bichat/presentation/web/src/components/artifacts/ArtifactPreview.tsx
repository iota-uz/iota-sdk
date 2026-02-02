import type { Artifact } from '../../types/artifacts'
import { ChartPreview } from './ChartPreview'
import { ImagePreview } from './ImagePreview'
import { FileDownload } from './FileDownload'
import { GenericPreview } from './GenericPreview'

interface ArtifactPreviewProps {
  artifact: Artifact
}

export function ArtifactPreview({ artifact }: ArtifactPreviewProps) {
  // Route based on artifact type
  if (artifact.type === 'chart') {
    return <ChartPreview artifact={artifact} />
  }

  if (artifact.type === 'code_output' && artifact.mimeType?.startsWith('image/')) {
    return <ImagePreview artifact={artifact} />
  }

  if (artifact.url) {
    return <FileDownload artifact={artifact} />
  }

  return <GenericPreview artifact={artifact} />
}
