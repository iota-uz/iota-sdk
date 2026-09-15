export type PortalSurface = 'modal' | 'drawer' | 'toast' | 'command'
export const WIDGET_SLOT_NAMES = ['navigation-leading', 'navigation-trailing', 'header-actions', 'document-status'] as const
export type WidgetSlotName = typeof WIDGET_SLOT_NAMES[number]

const zLayers: Record<PortalSurface, number> = { drawer: 600, modal: 700, command: 800, toast: 900 }

export interface OverlayEntry { id: string; surface: PortalSurface; restore: HTMLElement | null }

const documentLocks = new WeakMap<Document, { count: number; overflow: string }>()
const backgroundLocks = new WeakMap<HTMLElement, { count: number; inert: boolean; ariaHidden: string | null }>()

function acquireDocumentLock(owner: Document): void {
  const state = documentLocks.get(owner)
  if (state) { state.count += 1; return }
  documentLocks.set(owner, { count: 1, overflow: owner.documentElement.style.overflow })
  owner.documentElement.style.overflow = 'hidden'
}

function releaseDocumentLock(owner: Document): void {
  const state = documentLocks.get(owner)
  if (!state) return
  state.count -= 1
  if (state.count > 0) return
  owner.documentElement.style.overflow = state.overflow
  documentLocks.delete(owner)
}

function acquireBackgroundLock(background: HTMLElement): void {
  const state = backgroundLocks.get(background)
  if (state) { state.count += 1; return }
  backgroundLocks.set(background, { count: 1, inert: Boolean(background.inert), ariaHidden: background.getAttribute('aria-hidden') })
  background.inert = true
  background.setAttribute('aria-hidden', 'true')
}

function releaseBackgroundLock(background: HTMLElement): void {
  const state = backgroundLocks.get(background)
  if (!state) return
  state.count -= 1
  if (state.count > 0) return
  background.inert = state.inert
  if (state.ariaHidden === null) background.removeAttribute('aria-hidden')
  else background.setAttribute('aria-hidden', state.ariaHidden)
  backgroundLocks.delete(background)
}

function isBlocking(surface: PortalSurface): boolean {
  return surface === 'modal' || surface === 'drawer' || surface === 'command'
}

export class PortalRegistry {
  private readonly roots = new Map<PortalSurface, HTMLElement>()
  private readonly overlays: OverlayEntry[] = []
  private locked = false

  constructor(private readonly owner: HTMLElement, private readonly background?: HTMLElement) {}

  get ownerDocument(): Document { return this.owner.ownerDocument }

  root(surface: PortalSurface): HTMLElement {
    let root = this.roots.get(surface)
    if (!root) {
      root = this.owner.ownerDocument.createElement('div')
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
      if (index < 0) return
      const wasTopBlocking = isBlocking(entry.surface) && !this.overlays.slice(index + 1).some(({ surface }) => isBlocking(surface))
      const nextBlocking = this.overlays.slice(index + 1).find(({ surface }) => isBlocking(surface))
      this.overlays.splice(index, 1)
      // A nested overlay normally restores into its parent. If that parent is
      // removed first, preserve the parent's still-connected restore target.
      if (nextBlocking && !nextBlocking.restore?.isConnected) nextBlocking.restore = entry.restore
      this.sync()
      if (wasTopBlocking && entry.restore?.isConnected) entry.restore.focus()
    }
  }

  destroy(): void {
    const restore = this.overlays.find(({ surface }) => isBlocking(surface))?.restore
    this.overlays.splice(0)
    this.sync()
    for (const root of this.roots.values()) root.remove()
    this.roots.clear()
    if (restore?.isConnected) restore.focus()
  }

  private sync(): void {
    const blocking = this.overlays.some(({ surface }) => isBlocking(surface))
    if (blocking && !this.locked) {
      this.locked = true
      acquireDocumentLock(this.owner.ownerDocument)
      if (this.background) acquireBackgroundLock(this.background)
    } else if (!blocking && this.locked) {
      this.locked = false
      releaseDocumentLock(this.owner.ownerDocument)
      if (this.background) releaseBackgroundLock(this.background)
    }
  }
}
