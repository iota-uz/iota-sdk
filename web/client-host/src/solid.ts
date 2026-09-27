import { createComponent, createContext, useContext, type Component } from 'solid-js'
import { render } from 'solid-js/web'
import { assertClientBootstrap, readClientBootstrap, type ClientRouteContext } from './bootstrap'
import { PortalRegistry, WIDGET_SLOT_NAMES, type WidgetSlotName } from './portal-host'
import { createClientHostServices, type ClientHostServices } from './services'
import { SolidPortalHostProvider, type SolidPortalHost } from './solid-portals'

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
  portals?: HTMLElement
  background?: HTMLElement
  widgetSlots?: Partial<Record<WidgetSlotName, HTMLElement>>
}

function routeElementPrefix(root: HTMLElement): string | undefined {
  return root.id.endsWith('-mount') ? root.id.slice(0, -'-mount'.length) : undefined
}

function discoverPortalOwner(root: HTMLElement): HTMLElement | undefined {
  const prefix = routeElementPrefix(root)
  return prefix ? root.ownerDocument.getElementById(`${prefix}-portals`) ?? undefined : undefined
}

function discoverWidgetSlots(owner: Document): Partial<Record<WidgetSlotName, HTMLElement>> {
  const slots: Partial<Record<WidgetSlotName, HTMLElement>> = {}
  for (const name of WIDGET_SLOT_NAMES) {
    const slot = owner.querySelector<HTMLElement>(`[data-iota-widget-slot="${name}"]`)
    if (slot) slots[name] = slot
  }
  return slots
}

interface PortalThemeLease { token: symbol; theme: 'light' | 'dark' }
interface PortalThemeState { base: string | null; leases: PortalThemeLease[] }
const portalThemeStates = new WeakMap<HTMLElement, PortalThemeState>()

function setPortalTheme(owner: HTMLElement, theme: 'light' | 'dark'): () => void {
  const state = portalThemeStates.get(owner) ?? { base: owner.getAttribute('data-theme'), leases: [] }
  const token = Symbol('portal-theme')
  state.leases.push({ token, theme })
  portalThemeStates.set(owner, state)
  owner.dataset.theme = theme
  return () => {
    const index = state.leases.findIndex((lease) => lease.token === token)
    if (index < 0) return
    state.leases.splice(index, 1)
    const active = state.leases.at(-1)
    if (active) owner.dataset.theme = active.theme
    else {
      if (state.base === null) delete owner.dataset.theme
      else owner.setAttribute('data-theme', state.base)
      portalThemeStates.delete(owner)
    }
  }
}

function cleanupFailure(failures: unknown[], message: string): unknown | undefined {
  if (failures.length === 1) return failures[0]
  if (failures.length > 1) return new AggregateError(failures, message, { cause: failures[0] })
  return undefined
}

function cleanupPortalHost(cleanups: Set<() => void> | undefined, registry: PortalRegistry | undefined): unknown[] {
  const failures: unknown[] = []
  for (const cleanup of cleanups ?? []) {
    try {
      cleanup()
    } catch (cause) {
      failures.push(cause)
    }
  }
  cleanups?.clear()
  try {
    registry?.destroy()
  } catch (cause) {
    failures.push(cause)
  }
  return failures
}

export function mountSolidClientRoute<TInitial, TProps extends object>(options: MountSolidClientRouteOptions<TInitial, TProps>): () => void {
  assertClientBootstrap(options.context)
  const currentOwner = options.root.dataset.iotaClientOwner
  if (currentOwner) throw new Error(`Client route root is already owned by ${currentOwner}`)
  options.root.dataset.iotaClientOwner = 'solid'
  let services: ClientHostServices | undefined
  let portalRegistry: PortalRegistry | undefined
  let portalHostCleanups: Set<() => void> | undefined
  let restorePortalTheme: (() => void) | undefined
  try {
    const owner = options.root.ownerDocument.defaultView
    if (!options.services && !owner) throw new Error('Client route root is not attached to a browser realm')
    services = options.services ?? createClientHostServices(options.context, owner!)
    const mountedServices = services
    const Component = options.component
    const portalOwner = options.portals ?? discoverPortalOwner(options.root)
    portalRegistry = portalOwner
      ? new PortalRegistry(portalOwner, options.background ?? options.root.closest<HTMLElement>('[data-client-route-background]') ?? undefined)
      : undefined
    portalHostCleanups = portalRegistry ? new Set() : undefined
    const registeredCleanups = portalHostCleanups
    const portalHost: SolidPortalHost | undefined = portalRegistry ? {
      registry: portalRegistry,
      slots: new Map(Object.entries(options.widgetSlots ?? discoverWidgetSlots(options.root.ownerDocument)) as [WidgetSlotName, HTMLElement][]),
      registerCleanup(cleanup) {
        registeredCleanups!.add(cleanup)
        return () => registeredCleanups!.delete(cleanup)
      },
    } : undefined
    restorePortalTheme = portalOwner ? setPortalTheme(portalOwner, options.context.theme) : undefined
    const dispose = render(
      () => {
        return portalHost
          ? createComponent(SolidPortalHostProvider, {
              host: portalHost,
              get children() {
                return createComponent(ClientHostContext.Provider, {
                  value: mountedServices,
                  get children() { return createComponent(Component, { ...options.props, route: options.context }) },
                })
              },
            })
          : createComponent(ClientHostContext.Provider, {
              value: mountedServices,
              get children() { return createComponent(Component, { ...options.props, route: options.context }) },
            })
      },
      options.root,
    )
    let mounted = true
    return () => {
      if (!mounted) return
      mounted = false
      const failures: unknown[] = []
      try {
        dispose()
      } catch (cause) {
        failures.push(cause)
      }
      failures.push(...cleanupPortalHost(portalHostCleanups, portalRegistry))
      try {
        restorePortalTheme?.()
      } catch (cause) {
        failures.push(cause)
      }
      try {
        mountedServices.dispose()
      } catch (cause) {
        failures.push(cause)
      } finally {
        delete options.root.dataset.iotaClientOwner
      }
      const failure = cleanupFailure(failures, 'Client route cleanup failed')
      if (failure !== undefined) throw failure
    }
  } catch (cause) {
    const failures = [cause]
    failures.push(...cleanupPortalHost(portalHostCleanups, portalRegistry))
    try {
      restorePortalTheme?.()
    } catch (cleanupCause) {
      failures.push(cleanupCause)
    }
    try {
      services?.dispose()
    } catch (cleanupCause) {
      failures.push(cleanupCause)
    } finally {
      delete options.root.dataset.iotaClientOwner
    }
    if (failures.length === 1) throw cause
    throw new AggregateError(failures, 'Client route mount and cleanup failed', { cause })
  }
}

export function mountSolidClientRouteFromDocument<TInitial, TProps extends object>(
  component: Component<TProps & SolidRouteProps<TInitial>>,
  props: TProps,
  owner: Document = document,
): () => void {
  const root = owner.getElementById('iota-client-route-mount')
  if (!root) throw new Error('Client route mount element is missing')
  const portals = owner.getElementById('iota-client-route-portals') ?? undefined
  const background = root.closest<HTMLElement>('[data-client-route-background]') ?? undefined
  return mountSolidClientRoute({
    root,
    component,
    props,
    context: readClientBootstrap<TInitial>(owner),
    ...(portals ? { portals } : {}),
    ...(background ? { background } : {}),
    widgetSlots: discoverWidgetSlots(owner),
  })
}

export interface SolidFeatureModule {
  // The generated catalog erases each feature's concrete props only at this
  // dispatch boundary; every module is checked against its generated Props.
  default: Component<SolidRouteProps<never>>
}

export type SolidFeatureCatalog = Record<string, () => Promise<SolidFeatureModule>>

export interface MountSolidFeatureCatalogOptions {
  catalog: SolidFeatureCatalog
  owner?: Document
  /** Must navigate or dispose the current mount so Retry does not retain ownership. */
  reload?: () => void
}

function documentTheme(owner: Document, fallback: 'light' | 'dark'): 'light' | 'dark' {
  const root = owner.documentElement
  if (root.dataset.theme === 'dark') return 'dark'
  if (root.dataset.theme === 'light') return 'light'
  if (root.classList.contains('dark')) return 'dark'
  if (root.classList.contains('light')) return 'light'
  if (root.classList.contains('system') && owner.defaultView?.matchMedia?.('(prefers-color-scheme: dark)').matches) return 'dark'
  return fallback
}

// mountSolidFeatureCatalogFromDocument owns route-level loading, bundle
// failure/retry, mount and disposal for statically generated feature catalogs.
// Feature code owns only states inside the loaded screen.
export function mountSolidFeatureCatalogFromDocument(
  options: MountSolidFeatureCatalogOptions,
): () => void {
  const owner = options.owner ?? document
  const root = owner.getElementById('iota-client-route-mount')
  if (!root) throw new Error('Client route mount element is missing')
  const context = readClientBootstrap(owner)
  context.theme = documentTheme(owner, context.theme)
  const featureID = context.route?.featureId
  if (!featureID) throw new Error('Client route feature identity is missing')
  const load = options.catalog[featureID]
  if (!load) throw new Error(`Solid feature ${featureID} is absent from the generated catalog`)
  if (root.dataset.iotaClientOwner) {
    throw new Error(`Client route root is already owned by ${root.dataset.iotaClientOwner}`)
  }

  let active = true
  let generation = 0
  let disposeMounted: (() => void) | undefined
  root.dataset.iotaClientOwner = 'solid-loading'

  const showLoading = () => {
    root.setAttribute('aria-busy', 'true')
    const status = owner.createElement('div')
    status.dataset.solidRouteLoading = 'true'
    status.setAttribute('role', 'status')
    status.textContent = 'Loading…'
    root.replaceChildren(status)
  }

  const start = () => {
    if (!active) return
    const current = ++generation
    showLoading()
    void load().then(
      (module) => {
        if (!active || current !== generation) return
        if (!module || typeof module.default !== 'function') {
          throw new Error(`Solid feature ${featureID} has no default component export`)
        }
        root.removeAttribute('aria-busy')
        root.replaceChildren()
        delete root.dataset.iotaClientOwner
        disposeMounted = mountSolidClientRoute({
          root,
          component: module.default as Component<SolidRouteProps<unknown>>,
          props: {},
          context,
        })
      },
      (cause: unknown) => {
        if (!active || current !== generation) return
        root.removeAttribute('aria-busy')
        const alert = owner.createElement('div')
        alert.dataset.solidRouteError = 'true'
        alert.setAttribute('role', 'alert')
        const message = owner.createElement('p')
        message.textContent = 'The screen could not be loaded.'
        const retry = owner.createElement('button')
        retry.type = 'button'
        retry.dataset.solidRouteRetry = 'true'
        retry.textContent = 'Retry'
        retry.addEventListener('click', () => {
          // Browsers cache a rejected dynamic import for the lifetime of the
          // document. Reloading creates a fresh module map, so Retry can
          // actually request a chunk that became available after the failure.
          const reload = options.reload ?? (() => owner.defaultView?.location.reload())
          reload()
        }, { once: true })
        alert.append(message, retry)
        root.replaceChildren(alert)
        owner.defaultView?.console.error(`Failed to load Solid feature ${featureID}`, cause)
      },
    ).catch((cause: unknown) => {
      if (!active || current !== generation) return
      root.removeAttribute('aria-busy')
      const alert = owner.createElement('div')
      alert.dataset.solidRouteError = 'true'
      alert.setAttribute('role', 'alert')
      alert.textContent = 'The screen module is invalid.'
      root.replaceChildren(alert)
      owner.defaultView?.console.error(`Invalid Solid feature ${featureID}`, cause)
    })
  }

  const dispose = () => {
    if (!active) return
    active = false
    generation++
    owner.defaultView?.removeEventListener('pagehide', dispose)
    owner.removeEventListener('iota:client-route-unmount', dispose)
    try {
      disposeMounted?.()
    } finally {
      root.removeAttribute('aria-busy')
      root.replaceChildren()
      delete root.dataset.iotaClientOwner
    }
  }

  owner.defaultView?.addEventListener('pagehide', dispose, { once: true })
  owner.addEventListener('iota:client-route-unmount', dispose, { once: true })
  start()
  return dispose
}
