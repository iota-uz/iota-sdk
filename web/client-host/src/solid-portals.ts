import { createComponent, createContext, createEffect, createUniqueId, onCleanup, onMount, useContext, type JSX } from 'solid-js'
import { insert } from 'solid-js/web'
import { PortalRegistry, type PortalSurface, type WidgetSlotName } from './portal-host'

export type { PortalSurface, WidgetSlotName } from './portal-host'

export interface SolidPortalHost {
  registry: PortalRegistry
  slots: ReadonlyMap<WidgetSlotName, HTMLElement>
  registerCleanup(cleanup: () => void): () => void
}

const PortalHostContext = createContext<SolidPortalHost>()

export function SolidPortalHostProvider(props: { host: SolidPortalHost; children: JSX.Element }): JSX.Element {
  return createComponent(PortalHostContext.Provider, {
    value: props.host,
    get children() { return props.children },
  })
}

export interface PortalProps {
  surface: PortalSurface
  label: string
  class?: string
  className?: string
  onEscape?: () => void
  theme?: 'light' | 'dark'
  children?: JSX.Element
}

const focusableSelector = 'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])'

export function Portal(props: PortalProps): JSX.Element {
  const host = useContext(PortalHostContext)
  if (!host) throw new Error('Portal must be rendered inside a mounted Solid client route with a portal owner')

  const id = createUniqueId()
  const document = host.registry.ownerDocument
  const content = document.createElement('div')
  const marker = document.createTextNode('')
  const activeElement = document.activeElement
  const restore = activeElement && 'focus' in activeElement ? activeElement as HTMLElement : null
  let unmountOverlay: () => void = () => undefined
  let unregisterHostCleanup: () => void = () => undefined
  let cleaned = false

  insert(content, () => props.children)
  createEffect(() => {
    content.className = props.class ?? props.className ?? ''
    content.dataset.iotaSurface = props.surface
    if (props.theme) content.dataset.theme = props.theme
    else delete content.dataset.theme
  })
  content.setAttribute('role', props.surface === 'toast' ? 'status' : 'dialog')
  content.setAttribute('aria-label', props.label)
  content.tabIndex = -1
  if (props.surface !== 'toast') content.setAttribute('aria-modal', 'true')

  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Escape' && props.onEscape) {
      event.preventDefault()
      event.stopPropagation()
      props.onEscape()
      return
    }
    if (event.key !== 'Tab' || props.surface === 'toast') return
    const focusable = Array.from(content.querySelectorAll<HTMLElement>(focusableSelector))
    if (focusable.length === 0) {
      event.preventDefault()
      content.focus()
      return
    }
    const first = focusable[0]
    const last = focusable.at(-1)
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last?.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first?.focus()
    }
  }
  content.addEventListener('keydown', onKeyDown)

  const cleanup = () => {
    if (cleaned) return
    cleaned = true
    content.removeEventListener('keydown', onKeyDown)
    content.remove()
    unmountOverlay()
    unregisterHostCleanup()
  }
  onMount(() => {
    host.registry.root(props.surface).append(content)
    unmountOverlay = host.registry.mount({ id, surface: props.surface, restore })
    unregisterHostCleanup = host.registerCleanup(cleanup)
    if (props.surface !== 'toast') (content.querySelector<HTMLElement>(focusableSelector) ?? content).focus()
  })
  onCleanup(cleanup)

  return marker
}

export function WidgetSlot(props: { name: WidgetSlotName; children?: JSX.Element }): JSX.Element {
  const host = useContext(PortalHostContext)
  if (!host) throw new Error('WidgetSlot must be rendered inside a mounted Solid client route with a portal owner')
  const slot = host.slots.get(props.name)
  if (!slot) {
    host.registry.ownerDocument.defaultView?.console.warn(`Solid widget slot ${props.name} is unavailable`)
    return undefined
  }
  const container = slot.ownerDocument.createElement('div')
  const marker = slot.ownerDocument.createTextNode('')
  let unregisterHostCleanup: () => void = () => undefined
  let cleaned = false
  container.style.display = 'contents'
  insert(container, () => props.children)
  const cleanup = () => {
    if (cleaned) return
    cleaned = true
    container.remove()
    unregisterHostCleanup()
  }
  onMount(() => {
    slot.append(container)
    unregisterHostCleanup = host.registerCleanup(cleanup)
  })
  onCleanup(cleanup)
  return marker
}
