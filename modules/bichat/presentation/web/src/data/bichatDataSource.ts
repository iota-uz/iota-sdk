import { useMemo } from 'react'
import type { ChatDataSource } from '@iota-uz/sdk/bichat'
import { createHttpDataSource } from '@iota-uz/sdk/bichat'
import { useIotaContext } from '../contexts/IotaContext'

export function useBiChatDataSource(
  onNavigateToSession?: (sessionId: string) => void
): ChatDataSource {
  const ctx = useIotaContext()

  return useMemo(() => {
    const ds = createHttpDataSource({
      baseUrl: '',
      rpcEndpoint: ctx.config.rpcUIEndpoint,
      streamEndpoint: ctx.config.streamEndpoint,
    })

    if (onNavigateToSession) {
      ds.navigateToSession = onNavigateToSession
    }

    return ds
  }, [ctx.config.rpcUIEndpoint, ctx.config.streamEndpoint, onNavigateToSession])
}
