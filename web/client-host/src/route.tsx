import type { ComponentType } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { ClientHostProvider, type WidgetSlotName } from './portals'
import { assertClientBootstrap, type ClientRouteContext } from './bootstrap'

export { ClientHostProtocolError, assertClientHostProtocol, type ClientRouteContext } from './bootstrap'

export interface MountClientRouteOptions<TInitial, TProps extends object> {
  root: HTMLElement
  portals: HTMLElement
  background?: HTMLElement
  widgetSlots?: Partial<Record<WidgetSlotName, HTMLElement>>
  component: ComponentType<TProps & { route: ClientRouteContext<TInitial> }>
  props: TProps
  context: ClientRouteContext<TInitial>
}

export function mountClientRoute<TInitial, TProps extends object>(options: MountClientRouteOptions<TInitial, TProps>): () => void {
  assertClientBootstrap(options.context, false)
  const currentOwner = options.root.dataset.iotaClientOwner
  if (currentOwner) throw new Error(`Client route root is already owned by ${currentOwner}`)
  options.root.dataset.iotaClientOwner = 'react'
  const root: Root = createRoot(options.root)
  const Component = options.component
  root.render(
    <ClientHostProvider portalOwner={options.portals} background={options.background} widgetSlots={options.widgetSlots} theme={options.context.theme}>
      <Component {...options.props} route={options.context} />
    </ClientHostProvider>,
  )
  return () => {
    root.unmount()
    delete options.root.dataset.iotaClientOwner
  }
}
