import { createSignal, onCleanup, type Accessor } from 'solid-js'
import { asHostError, type HostError } from './errors'
import { useClientHost } from './solid'
import {
  subscribeManagedStream,
  type ManagedStreamOptions,
  type StreamContract,
  type StreamDiagnostics,
  type StreamSubscription,
} from './streaming'

export type ManagedStreamState =
  | { status: 'connecting' }
  | { status: 'live'; cursor?: string }
  | { status: 'reconnecting'; attempt: number; cursor?: string }
  | { status: 'done'; cursor?: string }
  | { status: 'error'; error: HostError; cursor?: string }

export interface ManagedStreamHandle {
  state: Accessor<ManagedStreamState>
  close(): void
  readonly closed: boolean
}

export function createManagedStream<TEvent>(
  contract: StreamContract<TEvent>,
  onEvent: (event: TEvent) => void,
  options: ManagedStreamOptions<TEvent>,
): ManagedStreamHandle {
  const [state, setState] = createSignal<ManagedStreamState>({ status: 'connecting' })
  let disposed = false
  let cursor: string | undefined
  const diagnostics: StreamDiagnostics = {
    event(event) {
      if (disposed) return
      cursor = event.cursor ?? cursor
      switch (event.state) {
        case 'connect':
          setState({ status: 'connecting' })
          break
        case 'reconnect':
          setState({ status: 'reconnecting', attempt: event.attempt ?? 0, ...(cursor === undefined ? {} : { cursor }) })
          break
        case 'event':
          setState({ status: 'live', ...(cursor === undefined ? {} : { cursor }) })
          break
        case 'terminal':
          setState({ status: 'done', ...(cursor === undefined ? {} : { cursor }) })
          break
        case 'error':
          setState({ status: 'error', error: event.error ?? asHostError(undefined), ...(cursor === undefined ? {} : { cursor }) })
          break
      }
      options.diagnostics?.event?.(event)
    },
  }
  const subscription = subscribeManagedStream(
    contract,
    (event) => {
      if (disposed) return
      onEvent(event)
    },
    { ...options, diagnostics },
  )
  onCleanup(() => {
    disposed = true
    subscription.close()
  })
  return {
    state,
    close: () => {
      disposed = true
      subscription.close()
    },
    get closed() {
      return subscription.closed
    },
  }
}

export interface HostStreamOptions<TEvent> extends Omit<ManagedStreamOptions<TEvent>, 'session' | 'diagnostics'> {
  diagnostics?: StreamDiagnostics
  /** Stream identity forwarded to host telemetry: `iota.stream.<state>`. */
  telemetry?: boolean
}

/**
 * Subscribes through the mounted client route host: host session/CSRF and
 * typed 401 re-auth, host telemetry for lifecycle states and owner-bound
 * cleanup on route unmount.
 */
export function useManagedStream<TEvent>(
  contract: StreamContract<TEvent>,
  onEvent: (event: TEvent) => void,
  options: HostStreamOptions<TEvent>,
): ManagedStreamHandle {
  const host = useClientHost()
  const { diagnostics, telemetry, ...rest } = options
  const forward: StreamDiagnostics = { event: (event) => diagnostics?.event?.(event) }
  if (telemetry ?? true) {
    const upstream = forward.event
    forward.event = (event) => {
      upstream?.(event)
      if (event.state === 'event') return
      host.telemetry.emit(`iota.stream.${event.state}`, {
        stream: contract.id,
        ...(event.cursor === undefined ? {} : { cursor: event.cursor }),
        ...(event.attempt === undefined ? {} : { attempt: event.attempt }),
      })
    }
  }
  return createManagedStream(contract, onEvent, { ...rest, session: host.session, diagnostics: forward })
}
