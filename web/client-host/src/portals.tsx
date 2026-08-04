import { createContext, type KeyboardEvent, type PropsWithChildren, type ReactNode, useContext, useEffect, useId, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

export type PortalSurface = 'modal' | 'drawer' | 'toast' | 'command'
export type WidgetSlotName = 'navigation-leading' | 'navigation-trailing' | 'header-actions' | 'document-status'

const zLayers: Record<PortalSurface, number> = { drawer: 600, modal: 700, command: 800, toast: 900 }

interface OverlayEntry { id: string; surface: PortalSurface; restore: HTMLElement | null }

class PortalRegistry {
  private readonly roots = new Map<PortalSurface, HTMLElement>()
  private readonly overlays: OverlayEntry[] = []
  constructor(private readonly owner: HTMLElement, private readonly background?: HTMLElement) {}

  root(surface: PortalSurface): HTMLElement {
    let root = this.roots.get(surface)
    if (!root) {
      root = document.createElement('div')
      root.dataset.iotaPortal = surface
      root.style.position = 'relative'
      root.style.zIndex = String(zLayers[surface])
      this.owner.append(root)
      this.roots.set(surface, root)
    }
    return root
  }

  mount(entry: OverlayEntry): () => void {
    this.overlays.push(entry)
    this.sync()
    return () => {
      const index = this.overlays.findIndex((candidate) => candidate.id === entry.id)
      if (index >= 0) this.overlays.splice(index, 1)
      this.sync()
      entry.restore?.focus()
    }
  }

  destroy(): void {
    this.overlays.splice(0)
    this.sync()
    for (const root of this.roots.values()) root.remove()
    this.roots.clear()
  }

  private sync(): void {
    const blocking = this.overlays.some(({ surface }) => surface === 'modal' || surface === 'drawer' || surface === 'command')
    document.documentElement.style.overflow = blocking ? 'hidden' : ''
    if (this.background) {
      this.background.inert = blocking
      if (blocking) this.background.setAttribute('aria-hidden', 'true')
      else this.background.removeAttribute('aria-hidden')
    }
  }
}

const PortalContext = createContext<PortalRegistry | undefined>(undefined)
const WidgetContext = createContext<ReadonlyMap<WidgetSlotName, HTMLElement>>(new Map())

export interface ClientHostProviderProps extends PropsWithChildren {
  portalOwner: HTMLElement
  background?: HTMLElement | undefined
  widgetSlots?: Partial<Record<WidgetSlotName, HTMLElement>> | undefined
  theme?: 'light' | 'dark' | undefined
}

/** Owns document-lifetime portal roots; feature roots must never append to body. */
export function ClientHostProvider({ portalOwner, background, widgetSlots = {}, theme, children }: ClientHostProviderProps) {
  const registry = useMemo(() => new PortalRegistry(portalOwner, background), [portalOwner, background])
  const slots = useMemo(() => new Map(Object.entries(widgetSlots) as [WidgetSlotName, HTMLElement][]), [widgetSlots])
  useEffect(() => {
    if (theme) portalOwner.dataset.theme = theme
    return () => { registry.destroy(); delete portalOwner.dataset.theme }
  }, [portalOwner, registry, theme])
  return <PortalContext.Provider value={registry}><WidgetContext.Provider value={slots}>{children}</WidgetContext.Provider></PortalContext.Provider>
}

export interface PortalProps extends PropsWithChildren { surface: PortalSurface; label: string; className?: string; onEscape?: () => void; theme?: 'light' | 'dark' }

export function Portal({ surface, label, className, onEscape, theme, children }: PortalProps) {
  const registry = useContext(PortalContext)
  if (!registry) throw new Error('Portal must be rendered inside ClientHostProvider')
  const id = useId()
  const content = useRef<HTMLDivElement>(null)
  useEffect(() => registry.mount({ id, surface, restore: document.activeElement as HTMLElement | null }), [id, registry, surface])
  useEffect(() => {
    if (surface === 'toast') return
    const root = content.current
    const first = root?.querySelector<HTMLElement>('button,[href],input,select,textarea,[tabindex]:not([tabindex="-1"])')
    first?.focus()
  }, [surface])
  const trapFocus = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape' && onEscape) { event.preventDefault(); event.stopPropagation(); onEscape(); return }
    if (event.key !== 'Tab' || surface === 'toast') return
    const focusable = Array.from(content.current?.querySelectorAll<HTMLElement>('a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])') ?? [])
    if (focusable.length === 0) { event.preventDefault(); content.current?.focus(); return }
    const first = focusable[0]
    const last = focusable.at(-1)
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus() }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
  }
  return createPortal(
    <div ref={content} role={surface === 'toast' ? 'status' : 'dialog'} aria-label={label} aria-modal={surface === 'toast' ? undefined : true} className={className} data-iota-surface={surface} data-theme={theme} onKeyDown={trapFocus} tabIndex={-1}>{children}</div>,
    registry.root(surface),
  )
}

/** Uses an existing standard host when present, otherwise creates a bounded
 * compatibility host inside the feature root (never a body-level root). */
export function ClientHostBoundary({ children, theme }: PropsWithChildren<{ theme?: 'light' | 'dark' | undefined }>) {
  const existing = useContext(PortalContext)
  if (existing) return children
  return <EmbeddedClientHost theme={theme}>{children}</EmbeddedClientHost>
}

function EmbeddedClientHost({ children, theme }: PropsWithChildren<{ theme?: 'light' | 'dark' | undefined }>) {
  const [background, setBackground] = useState<HTMLDivElement | null>(null)
  const [portals, setPortals] = useState<HTMLDivElement | null>(null)
  return <>
    <div data-client-route-background ref={setBackground}>
      {background && portals ? <ClientHostProvider portalOwner={portals} background={background} theme={theme}>{children}</ClientHostProvider> : null}
    </div>
    <div data-client-route-portals ref={setPortals} />
  </>
}

/** Widget slots are persistent shell surfaces and intentionally separate from overlays. */
export function WidgetSlot({ name, children }: { name: WidgetSlotName; children: ReactNode }) {
  const slot = useContext(WidgetContext).get(name)
  return slot ? createPortal(children, slot) : null
}
