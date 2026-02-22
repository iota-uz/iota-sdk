import { useMemo } from 'react'
import type { ChatDataSource, SessionArtifact } from '@iota-uz/sdk/bichat'
import { createHttpDataSource } from '@iota-uz/sdk/bichat'
import { useIotaContext } from '../contexts/IotaContext'
import { useSessionEvents } from '../contexts/SessionEventContext'
import { attachRichChartDataToTurns, normalizeChartArtifactsForSdk } from '../charts/chartData'

export function useBiChatDataSource(
  onNavigateToSession?: (sessionId: string) => void
): ChatDataSource {
  const ctx = useIotaContext()
  const sessionEvents = useSessionEvents()

  return useMemo(() => {
    const ds = createHttpDataSource({
      baseUrl: '',
      rpcEndpoint: ctx.config.rpcUIEndpoint,
      streamEndpoint: ctx.config.streamEndpoint,
      timeout: 120000,
    })
    const artifactCache = new Map<string, SessionArtifact[]>()

    const originalFetchSessionArtifacts = ds.fetchSessionArtifacts?.bind(ds)
    if (originalFetchSessionArtifacts) {
      ds.fetchSessionArtifacts = async (sessionId, options) => {
        const result = await originalFetchSessionArtifacts(sessionId, options)
        const normalized = normalizeChartArtifactsForSdk(result.artifacts || [])
        artifactCache.set(sessionId, normalized)
        return {
          ...result,
          artifacts: normalized,
        }
      }
    }

    const originalFetchSession = ds.fetchSession.bind(ds)
    ds.fetchSession = async (id) => {
      const result = await originalFetchSession(id)
      if (!result) return result

      let artifacts = artifactCache.get(id)
      if (!artifacts && ds.fetchSessionArtifacts) {
        const collected: SessionArtifact[] = []
        let offset = 0
        const pageSize = 200
        for (;;) {
          const artifactsResult = await ds.fetchSessionArtifacts(id, { limit: pageSize, offset })
          collected.push(...(artifactsResult.artifacts || []))
          if (!artifactsResult.hasMore || (artifactsResult.artifacts?.length ?? 0) === 0) break
          offset = artifactsResult.nextOffset ?? offset + (artifactsResult.artifacts?.length ?? 0)
        }
        artifacts = collected
      }
      if (!artifacts || artifacts.length === 0) return result

      return {
        ...result,
        turns: attachRichChartDataToTurns(result.turns, artifacts),
      }
    }

    if (onNavigateToSession) {
      ds.navigateToSession = (sessionId: string) => {
        sessionEvents.notifySessionCreated(sessionId)
        onNavigateToSession(sessionId)
      }
    }

    return ds
  }, [ctx.config.rpcUIEndpoint, ctx.config.streamEndpoint, onNavigateToSession, sessionEvents])
}
