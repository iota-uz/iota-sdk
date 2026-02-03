import { useMemo } from 'react'
import type { ChatDataSource } from '@iotauz/iota-sdk/bichat'
import { createHttpDataSource } from '@iotauz/iota-sdk/bichat'
import { useIotaContext } from '../contexts/IotaContext'

export function useBiChatDataSource(
  onNavigateToSession?: (sessionId: string) => void
): ChatDataSource {
  const ctx = useIotaContext()

  return useMemo(() => {
    const ds = createHttpDataSource({
      baseUrl: '',
      graphQLEndpoint: ctx.config.graphQLEndpoint,
      streamEndpoint: ctx.config.streamEndpoint,
    })

    if (onNavigateToSession) {
      ds.navigateToSession = onNavigateToSession
    }

    return ds
  }, [ctx.config.graphQLEndpoint, ctx.config.streamEndpoint, onNavigateToSession])
}
