import { createComponent, createContext, useContext, type Component } from 'solid-js'
import { render } from 'solid-js/web'
import { assertClientBootstrap, readClientBootstrap, type ClientRouteContext } from './bootstrap'
import { createClientHostServices, type ClientHostServices } from './services'

const ClientHostContext = createContext<ClientHostServices>()

export function useClientHost(): ClientHostServices {
  const services = useContext(ClientHostContext)
  if (!services) throw new Error('Solid client route is not mounted by the iota client host')
  return services
}

export interface SolidRouteProps<TInitial = unknown> {
  route: ClientRouteContext<TInitial>
}

export interface MountSolidClientRouteOptions<TInitial, TProps extends object> {
  root: HTMLElement
  component: Component<TProps & SolidRouteProps<TInitial>>
  props: TProps
  context: ClientRouteContext<TInitial>
  services?: ClientHostServices
}

export function mountSolidClientRoute<TInitial, TProps extends object>(options: MountSolidClientRouteOptions<TInitial, TProps>): () => void {
  assertClientBootstrap(options.context)
  const currentOwner = options.root.dataset.iotaClientOwner
  if (currentOwner) throw new Error(`Client route root is already owned by ${currentOwner}`)
  options.root.dataset.iotaClientOwner = 'solid'
  let services: ClientHostServices | undefined
  try {
    const owner = options.root.ownerDocument.defaultView
    if (!options.services && !owner) throw new Error('Client route root is not attached to a browser realm')
    services = options.services ?? createClientHostServices(options.context, owner!)
    const mountedServices = services
    const Component = options.component
    const dispose = render(
      () => createComponent(ClientHostContext.Provider, {
        value: mountedServices,
        get children() { return createComponent(Component, { ...options.props, route: options.context }) },
      }),
      options.root,
    )
    let mounted = true
    return () => {
      if (!mounted) return
      mounted = false
      dispose()
      mountedServices.dispose()
      delete options.root.dataset.iotaClientOwner
    }
  } catch (cause) {
    services?.dispose()
    delete options.root.dataset.iotaClientOwner
    throw cause
  }
}

export function mountSolidClientRouteFromDocument<TInitial, TProps extends object>(
  component: Component<TProps & SolidRouteProps<TInitial>>,
  props: TProps,
  owner: Document = document,
): () => void {
  const root = owner.getElementById('iota-client-route-mount')
  if (!root) throw new Error('Client route mount element is missing')
  return mountSolidClientRoute({ root, component, props, context: readClientBootstrap<TInitial>(owner) })
}
