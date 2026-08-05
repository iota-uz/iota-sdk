import type { ComponentType } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { ClientHostProvider, type WidgetSlotName } from './portals'
import { CLIENT_HOST_PROTOCOL_VERSION } from './protocol'

export class ClientHostProtocolError extends Error {
  constructor(readonly expected: string, readonly actual: string) {
    super(`Client host protocol ${actual} is incompatible with ${expected}`)
    this.name = 'ClientHostProtocolError'
  }
}

export interface ClientRouteContext<TInitial = unknown> {
  protocolVersion: string
  initial: TInitial
  theme: 'light' | 'dark'
  csrf?: string
}

export interface MountClientRouteOptions<TInitial, TProps extends object> {
  root: HTMLElement
  portals: HTMLElement
  background?: HTMLElement
  widgetSlots?: Partial<Record<WidgetSlotName, HTMLElement>>
  component: ComponentType<TProps & { route: ClientRouteContext<TInitial> }>
  props: TProps
  context: ClientRouteContext<TInitial>
}

export function assertClientHostProtocol(actual: string): void {
  if (actual.split('.', 1)[0] !== CLIENT_HOST_PROTOCOL_VERSION.split('.', 1)[0]) {
    throw new ClientHostProtocolError(CLIENT_HOST_PROTOCOL_VERSION, actual)
  }
}

export function mountClientRoute<TInitial, TProps extends object>(options: MountClientRouteOptions<TInitial, TProps>): () => void {
  assertClientHostProtocol(options.context.protocolVersion)
  const root: Root = createRoot(options.root)
  const Component = options.component
  root.render(
    <ClientHostProvider portalOwner={options.portals} background={options.background} widgetSlots={options.widgetSlots} theme={options.context.theme}>
      <Component {...options.props} route={options.context} />
    </ClientHostProvider>,
  )
  return () => root.unmount()
}
