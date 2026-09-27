import { useCallback, useEffect, useRef, useState } from 'react'
import { queryKey } from './cache'
import { asHostError, type HostError } from './errors'
import { RPCClient, type RPCMethodContract } from './rpc-core'

export * from './rpc-core'

export type QueryState<T> = { status: 'loading' } | { status: 'success'; data: T } | { status: 'error'; error: HostError }

export function useRPCQuery<TRequest, TResponse>(client: RPCClient, contract: RPCMethodContract, request: TRequest): QueryState<TResponse> {
  const [state, setState] = useState<QueryState<TResponse>>({ status: 'loading' })
  const generation = useRef(0)
  const requestKey = queryKey(contract.namespace, contract.method, request)[2]
  useEffect(() => {
    const current = ++generation.current
    const controller = new AbortController()
    setState({ status: 'loading' })
    void client.query<TRequest, TResponse>(contract, request, controller.signal).then(
      (data) => { if (generation.current === current) setState({ status: 'success', data }) },
      (cause) => { if (!controller.signal.aborted && generation.current === current) setState({ status: 'error', error: asHostError(cause) }) },
    )
    return () => controller.abort()
  }, [client, contract, requestKey])
  return state
}

export function useRPCMutation<TRequest, TResponse>(client: RPCClient, contract: RPCMethodContract) {
  const controller = useRef<AbortController>()
  useEffect(() => () => controller.current?.abort(), [])
  return useCallback(async (request: TRequest): Promise<TResponse> => {
    controller.current?.abort()
    controller.current = new AbortController()
    return client.mutate<TRequest, TResponse>(contract, request, controller.current.signal)
  }, [client, contract])
}
