import { createEffect, createSignal, onCleanup, type Accessor } from 'solid-js'
import { queryKey } from './cache'
import { asHostError, type HostError } from './errors'
import { RPCClient, type RPCMethodContract } from './rpc-core'

export type SolidQueryState<T> =
  | { status: 'loading' }
  | { status: 'success'; data: T }
  | { status: 'error'; error: HostError }

export function createRPCQuery<TRequest, TResponse>(client: RPCClient, contract: RPCMethodContract, request: Accessor<TRequest>) {
  const [state, setState] = createSignal<SolidQueryState<TResponse>>({ status: 'loading' })
  let generation = 0
  let controller: AbortController | undefined
  const run = (input: TRequest) => {
    const current = ++generation
    controller?.abort()
    controller = new AbortController()
    setState({ status: 'loading' })
    void client.query<TRequest, TResponse>(contract, input, controller.signal).then(
      (data) => { if (generation === current && !controller?.signal.aborted) setState({ status: 'success', data }) },
      (cause) => { if (generation === current && !controller?.signal.aborted) setState({ status: 'error', error: asHostError(cause) }) },
    )
  }
  createEffect(() => {
    const input = request()
    queryKey(contract.namespace, contract.method, input)
    run(input)
  })
  onCleanup(() => controller?.abort())
  return { state, refetch: () => run(request()), cancel: () => controller?.abort() }
}

export function createRPCAction<TRequest, TResponse>(client: RPCClient, contract: RPCMethodContract) {
  const [pending, setPending] = createSignal(false)
  const [error, setError] = createSignal<HostError>()
  let generation = 0
  let controller: AbortController | undefined
  const execute = async (request: TRequest): Promise<TResponse> => {
    const current = ++generation
    controller?.abort()
    controller = new AbortController()
    setPending(true)
    setError(undefined)
    try {
      return await client.mutate<TRequest, TResponse>(contract, request, controller.signal)
    } catch (cause) {
      const hostError = asHostError(cause)
      if (generation === current && !controller.signal.aborted) setError(hostError)
      throw hostError
    } finally {
      if (generation === current) setPending(false)
    }
  }
  onCleanup(() => controller?.abort())
  return { execute, pending, error, fieldErrors: () => error()?.fieldErrors ?? {}, cancel: () => controller?.abort() }
}
