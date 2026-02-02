import { useState } from 'react'
import { useQuery } from 'urql'
import type { Artifact } from '../types/artifacts'
import { ArtifactList, ArtifactPreview } from './artifacts'
import { CaretLeft } from '@phosphor-icons/react'

interface ArtifactsSidebarProps {
  sessionId: string
  onArtifactSelect?: (artifact: Artifact) => void
}

const SessionArtifactsQuery = `
  query SessionArtifacts($id: UUID!) {
    session(id: $id) {
      id
      artifacts {
        id
        sessionID
        messageID
        type
        name
        description
        mimeType
        url
        sizeBytes
        metadata
        createdAt
      }
    }
  }
`

export function ArtifactsSidebar({ sessionId, onArtifactSelect }: ArtifactsSidebarProps) {
  const [selectedArtifact, setSelectedArtifact] = useState<Artifact | null>(null)

  const [result] = useQuery({
    query: SessionArtifactsQuery,
    variables: { id: sessionId },
  })

  const { data, fetching, error } = result

  const handleSelect = (artifact: Artifact) => {
    setSelectedArtifact(artifact)
    onArtifactSelect?.(artifact)
  }

  const handleBack = () => {
    setSelectedArtifact(null)
  }

  const artifacts: Artifact[] = data?.session?.artifacts || []

  return (
    <aside className="w-80 border-l border-gray-200 bg-white flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200">
        {selectedArtifact ? (
          <>
            <button
              onClick={handleBack}
              className="flex items-center gap-2 text-sm font-medium text-gray-700 hover:text-gray-900"
            >
              <CaretLeft className="w-4 h-4" weight="bold" />
              Back
            </button>
          </>
        ) : (
          <h2 className="text-sm font-semibold text-gray-900">
            Artifacts ({artifacts.length})
          </h2>
        )}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4">
        {fetching && (
          <div className="flex items-center justify-center py-8">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900"></div>
          </div>
        )}

        {error && (
          <div className="p-4 bg-red-50 border border-red-200 rounded-lg text-sm text-red-800">
            Failed to load artifacts: {error.message}
          </div>
        )}

        {!fetching && !error && selectedArtifact && (
          <div className="space-y-4">
            <ArtifactPreview artifact={selectedArtifact} />
          </div>
        )}

        {!fetching && !error && !selectedArtifact && (
          <ArtifactList artifacts={artifacts} onSelect={handleSelect} />
        )}
      </div>
    </aside>
  )
}
