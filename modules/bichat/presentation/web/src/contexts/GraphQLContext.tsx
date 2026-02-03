import { ReactNode } from 'react'
import { Provider, createClient, fetchExchange } from 'urql'
import { useIotaContext } from './IotaContext'

export function GraphQLProvider({ children }: { children: ReactNode }) {
  const { config } = useIotaContext()

  const client = createClient({
    url: config.graphQLEndpoint,
    exchanges: [fetchExchange],
    fetchOptions: {
      credentials: 'include',
    },
  })

  return <Provider value={client}>{children}</Provider>
}
