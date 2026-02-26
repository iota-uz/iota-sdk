import { useMemo } from 'react'
import type { ChatDataSource, SessionArtifact } from '@iota-uz/sdk/bichat'
import { createHttpDataSource } from '@iota-uz/sdk/bichat'
import { useIotaContext } from '../contexts/IotaContext'
import { attachRichChartDataToTurns, normalizeChartArtifactsForSdk } from '../charts/chartData'

export function useBiChatDataSource(): ChatDataSource {
  const ctx = useIotaContext()

  return useMemo(() => {
    const ds = createHttpDataSource({
      baseUrl: '',
      rpcEndpoint: ctx.config.rpcUIEndpoint,
      streamEndpoint: ctx.config.streamEndpoint,
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

    return ds
  }, [ctx.config.rpcUIEndpoint, ctx.config.streamEndpoint])
}
