// useQuery / useCommand for client pages. They run on the transport the
// runtime host installs for the page: server execution as the session user,
// typed through the contract schemas, structured errors preserved.
import { createComponent, createContext, createEffect, createSignal, onCleanup, useContext, type Accessor, type Component, type JSX } from 'solid-js'
import { createClient, type RemoteError } from '../client.ts'
import type { AtomicCommand, Command, InputOf, Query, ResultOf } from '../contracts.ts'
import type { Infer } from '../schema.ts'
import { bindTransport, type RuntimeTransport, type TransportError, type UnauthenticatedError } from './transport.ts'

/** Everything a hook can fail with. Business errors are RemoteError (kind, code, params, issues). */
export type CallError = RemoteError | TransportError | UnauthenticatedError | Error

const RuntimeTransportContext = createContext<RuntimeTransport>()

export interface RuntimeProviderProps {
  transport: RuntimeTransport
  children: JSX.Element
}

/** Installs the transport for a subtree. The runtime host wraps every page in it. */
export const RuntimeProvider: Component<RuntimeProviderProps> = (props) =>
  createComponent(RuntimeTransportContext.Provider, {
    get value() { return props.transport },
    get children() { return props.children },
  })

function useTransport(): RuntimeTransport {
  const transport = useContext(RuntimeTransportContext)
  if (!transport) throw new Error('useQuery/useCommand must be used inside a page mounted by the iota runtime host')
  return transport
}

export interface QueryResult<T> {
  /** Last successful value of the latest request. */
  readonly data: Accessor<T | undefined>
  readonly error: Accessor<CallError | undefined>
  readonly loading: Accessor<boolean>
  /** Runs the query again with the current input. */
  refetch(): void
}

function isAbort(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}

/**
 * Runs a query when the component mounts and whenever the input changes. A
 * response that arrives after a newer request started, or after the component
 * was disposed, is ignored.
 */
export function useQuery<Q extends Query>(query: Q, input: InputOf<Q> | Accessor<InputOf<Q>>): QueryResult<Infer<Q['output']>> {
  const transport = useTransport()
  const [data, setData] = createSignal<Infer<Q['output']>>()
  const [error, setError] = createSignal<CallError>()
  const [loading, setLoading] = createSignal(true)
  const [tick, setTick] = createSignal(0)
  let generation = 0
  let controller: AbortController | undefined
  let disposed = false

  createEffect(() => {
    tick()
    const value = typeof input === 'function' ? (input as Accessor<InputOf<Q>>)() : input
    const mine = ++generation
    controller?.abort()
    const current = (controller = new AbortController())
    setLoading(true)
    setError(undefined)
    void createClient(bindTransport(transport, current.signal)).query(query, value).then(
      (result) => {
        if (disposed || mine !== generation) return
        setData(() => result)
        setLoading(false)
      },
      (cause: unknown) => {
        if (disposed || mine !== generation || isAbort(cause)) return
        setError(cause instanceof Error ? cause : new Error(String(cause)))
        setLoading(false)
      },
    )
  })
  onCleanup(() => {
    disposed = true
    controller?.abort()
  })
  return { data, error, loading, refetch: () => setTick((n) => n + 1) }
}

export interface CommandResult<C extends Command | AtomicCommand> {
  /** Runs the command once. It rejects with the structured error; it is never retried. */
  run(input: InputOf<C>): Promise<ResultOf<C>>
  /** True while at least one run is in flight. */
  readonly pending: Accessor<boolean>
  /** Error of the latest finished run, cleared when a new run starts. */
  readonly error: Accessor<CallError | undefined>
  reset(): void
}

/**
 * A command on the server. Calling `run` twice runs it twice: deciding when a
 * repeat is safe is the page's business, not the transport's. Results that
 * arrive after the component was disposed still resolve the returned promise
 * (the command ran) but no longer touch state.
 */
export function useCommand<C extends Command | AtomicCommand>(command: C): CommandResult<C> {
  const transport = useTransport()
  const [inFlight, setInFlight] = createSignal(0)
  const [error, setError] = createSignal<CallError>()
  let disposed = false
  let latest = 0
  onCleanup(() => { disposed = true })
  const api = createClient(bindTransport(transport))
  async function run(input: InputOf<C>): Promise<ResultOf<C>> {
    const mine = ++latest
    setError(undefined)
    setInFlight((n) => n + 1)
    try {
      return await api.command(command, input)
    } catch (cause) {
      if (!disposed && mine === latest) setError(cause instanceof Error ? cause : new Error(String(cause)))
      throw cause
    } finally {
      if (!disposed) setInFlight((n) => n - 1)
    }
  }
  return { run, pending: () => inFlight() > 0, error, reset: () => setError(undefined) }
}
