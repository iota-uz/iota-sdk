export type NavigationGuard = () => boolean

export interface NavigationService {
  guard(check: NavigationGuard): () => void
  canNavigate(): boolean
}

export class BrowserNavigationService implements NavigationService {
  private readonly guards = new Set<NavigationGuard>()
  private readonly beforeUnload = (event: BeforeUnloadEvent) => {
    if (this.canNavigate()) return
    event.preventDefault()
    event.returnValue = ''
  }
  private readonly click = (event: MouseEvent) => {
    const target = event.target instanceof Element ? event.target.closest('a[href]') : null
    if (!target || event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
    const anchor = target as HTMLAnchorElement
    if (anchor.target === '_blank' || anchor.download || anchor.origin !== window.location.origin) return
    if (!this.canNavigate()) event.preventDefault()
  }

  constructor(private readonly owner: Window = window) {
    owner.addEventListener('beforeunload', this.beforeUnload)
    owner.document.addEventListener('click', this.click, true)
  }

  guard(check: NavigationGuard): () => void {
    this.guards.add(check)
    return () => this.guards.delete(check)
  }

  canNavigate(): boolean {
    for (const check of this.guards) if (!check()) return false
    return true
  }

  dispose(): void {
    this.owner.removeEventListener('beforeunload', this.beforeUnload)
    this.owner.document.removeEventListener('click', this.click, true)
    this.guards.clear()
  }
}
