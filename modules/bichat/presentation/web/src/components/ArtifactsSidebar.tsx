import { useEffect, useMemo, useState } from 'react'
import type { Artifact } from '../types/artifacts'
import { ArtifactList, ArtifactPreview } from './artifacts'
import { CaretLeft } from '@phosphor-icons/react'
import { createAppletRPCClient } from '@iota-uz/sdk'
import { useIotaContext } from '../contexts/IotaContext'
import { toRPCErrorDisplay, type RPCErrorDisplay } from '../utils/rpcErrors'

interface ArtifactsSidebarProps {
  sessionId: string
  onArtifactSelect?: (artifact: Artifact) => void
}

export function ArtifactsSidebar({ sessionId, onArtifactSelect }: ArtifactsSidebarProps) {
  const [selectedArtifact, setSelectedArtifact] = useState<Artifact | null>(null)
  const { config } = useIotaContext()
  const rpc = useMemo(
    () => createAppletRPCClient({ endpoint: config.rpcUIEndpoint }),
    [config.rpcUIEndpoint]
  )

  const [fetching, setFetching] = useState(true)
  const [error, setError] = useState<RPCErrorDisplay | null>(null)
  const [artifacts, setArtifacts] = useState<Artifact[]>([])

  useEffect(() => {
    let alive = true
    ;(async () => {
      setFetching(true)
      setError(null)
      try {
        const data = await rpc.call<
          { sessionId: string; limit: number; offset: number },
          { artifacts: Artifact[] }
        >('bichat.session.artifacts', { sessionId, limit: 200, offset: 0 })
        if (alive) setArtifacts(data.artifacts || [])
      } catch (e) {
        if (!alive) return
        setError(toRPCErrorDisplay(e, 'Failed to load artifacts'))
      } finally {
        if (alive) setFetching(false)
      }
    })()
    return () => {
      alive = false
    }
  }, [rpc, sessionId])

  const handleSelect = (artifact: Artifact) => {
    setSelectedArtifact(artifact)
    onArtifactSelect?.(artifact)
  }

  const handleBack = () => {
    setSelectedArtifact(null)
  }

  return (
    <aside className="w-80 border-l border-gray-200 dark:border-gray-700/80 bg-white dark:bg-gray-900 flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700/80">
        {selectedArtifact ? (
          <>
            <button
              onClick={handleBack}
              className="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-200 hover:text-gray-900 dark:hover:text-white transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 rounded-md p-1 -m-1"
            >
              <CaretLeft className="w-4 h-4" weight="bold" />
              Back
            </button>
          </>
        ) : (
          <h2 className="text-sm font-semibold text-gray-900 dark:text-white">
            Artifacts ({artifacts.length})
          </h2>
        )}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4">
        {fetching && (
          <div className="flex items-center justify-center py-8">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 dark:border-gray-100"></div>
          </div>
        )}

        {error && (
          <div
            className={
              error.isPermissionDenied
                ? 'p-4 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg'
                : 'p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg'
            }
          >
            <p
              className={
                error.isPermissionDenied
                  ? 'text-sm text-amber-700 dark:text-amber-300 font-medium'
                  : 'text-sm text-red-700 dark:text-red-300 font-medium'
              }
            >
              {error.title}
            </p>
            <p
              className={
                error.isPermissionDenied
                  ? 'mt-1 text-sm text-amber-600 dark:text-amber-400'
                  : 'mt-1 text-sm text-red-600 dark:text-red-400'
              }
            >
              {error.description}
            </p>
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
