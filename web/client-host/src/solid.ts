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
  const services = options.services ?? createClientHostServices(options.context)
  const Component = options.component
  const dispose = render(
    () => createComponent(ClientHostContext.Provider, {
      value: services,
      get children() { return createComponent(Component, { ...options.props, route: options.context }) },
    }),
    options.root,
  )
  let mounted = true
  return () => {
    if (!mounted) return
    mounted = false
    dispose()
    services.dispose()
    delete options.root.dataset.iotaClientOwner
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
