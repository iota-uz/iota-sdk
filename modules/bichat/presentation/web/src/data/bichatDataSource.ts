import { useMemo } from 'react'
import type { ChatDataSource } from '@iota-uz/sdk/bichat'
import { createHttpDataSource } from '@iota-uz/sdk/bichat'
import { useIotaContext } from '../contexts/IotaContext'
import { useSessionEvents } from '../contexts/SessionEventContext'

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

    if (onNavigateToSession) {
      ds.navigateToSession = (sessionId: string) => {
        sessionEvents.notifySessionCreated(sessionId)
        onNavigateToSession(sessionId)
      }
    }

    return ds
  }, [ctx.config.rpcUIEndpoint, ctx.config.streamEndpoint, onNavigateToSession, sessionEvents])
}
