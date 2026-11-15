import type { ShellRegistry } from './shell-registry'
import type { ThemeService } from './services'

export const SHELL_STANDARD_ATTRIBUTE = 'data-iota-shell-standard'
export const SHELL_OUTLET_ATTRIBUTE = 'data-iota-shell-outlet'
export const SHELL_STANDARD_QUERY = 'iota-shell'

export interface ShellUser { name: string; email: string }

export interface ShellRendererContext {
  /** Mount point owned by the renderer. */
  root: HTMLElement
  /**
   * The page content element, moved here from the standard shell with its
   * identity intact so HTMX swaps, scripts and modals keep working. The
   * renderer places it inside its layout; it is returned on deactivation.
   */
  outlet: HTMLElement
  navigation: ShellRegistry
  theme: ThemeService
  locale: { language: string }
  user: ShellUser | null
  actions: { logout(): void }
  /** Fires when the server reports changed notifications (the `notificationsChanged` page event). */
  notifications: { subscribe(listener: () => void): () => void }
  /** Reports an asynchronous renderer failure and returns to the standard shell. */
  fail(cause: unknown): void
}

export interface ShellRenderer {
  id: string
  mount(context: ShellRendererContext): void | (() => void)
}

export type ShellHostDiagnosticCode = 'renderer-conflict' | 'renderer-failed' | 'standard-shell-missing' | 'renderer-disabled'

export interface ShellHostDiagnostic { code: ShellHostDiagnosticCode; renderer: string; message: string }

export interface ShellHostOptions {
  registry: ShellRegistry
  theme: ThemeService
  user?: ShellUser | null
  logout?: () => void
}

interface Activation {
  renderer: ShellRenderer
  root: HTMLElement
  outlet: HTMLElement
  parent: Node
  next: Node | null
  standard: HTMLElement
  cleanup: void | (() => void)
}

/**
 * ShellHost owns the single app-wide shell renderer. Activating one hides the
 * standard shell, hands it the existing page outlet and exposes the shared
 * navigation registry; any failure puts everything back.
 */
export class ShellHost {
  private current: Activation | undefined
  private readonly problems: ShellHostDiagnostic[] = []

  constructor(private readonly owner: Document, private readonly options: ShellHostOptions) {}

  activeRenderer(): string | null { return this.current?.renderer.id ?? null }

  diagnostics(): ShellHostDiagnostic[] { return [...this.problems] }

  /** True when an administrator forced the standard shell with `?iota-shell=standard`. */
  standardForced(): boolean {
    return this.owner.defaultView?.location.search.includes(`${SHELL_STANDARD_QUERY}=standard`) ?? false
  }

  activate(renderer: ShellRenderer): boolean {
    if (this.standardForced()) {
      this.report('renderer-disabled', renderer.id, `Renderer ${renderer.id} not activated; the standard shell was requested`)
      return false
    }
    if (this.current) {
      this.report('renderer-conflict', renderer.id, `Renderer ${renderer.id} not activated; ${this.current.renderer.id} is already active`)
      return false
    }
    const standard = this.owner.querySelector<HTMLElement>(`[${SHELL_STANDARD_ATTRIBUTE}]`)
    const outlet = this.owner.querySelector<HTMLElement>(`[${SHELL_OUTLET_ATTRIBUTE}]`)
    if (!standard || !outlet || !outlet.parentNode) {
      this.report('standard-shell-missing', renderer.id, 'The page has no standard shell with a content outlet')
      return false
    }
    const root = this.owner.createElement('div')
    root.dataset.iotaShellRenderer = renderer.id
    const activation: Activation = { renderer, root, outlet, parent: outlet.parentNode, next: outlet.nextSibling, standard, cleanup: undefined }
    this.current = activation
    standard.after(root)
    standard.hidden = true
    standard.inert = true
    root.append(outlet)
    try {
      activation.cleanup = renderer.mount({
        root,
        outlet,
        navigation: this.options.registry,
        theme: this.options.theme,
        locale: { language: this.owner.documentElement.lang || 'en' },
        user: this.options.user ?? null,
        actions: { logout: () => this.options.logout?.() },
        notifications: {
          subscribe: (listener) => {
            this.owner.body.addEventListener('notificationsChanged', listener)
            return () => this.owner.body.removeEventListener('notificationsChanged', listener)
          },
        },
        fail: (cause) => { if (this.current === activation) this.fall(activation, cause) },
      })
    } catch (cause) {
      this.fall(activation, cause)
      return false
    }
    if (this.current !== activation) {
      try { activation.cleanup?.() } catch { /* renderer failed during mount and was already rolled back */ }
      return false
    }
    return true
  }

  deactivate(): void {
    if (this.current) this.restore(this.current)
  }

  private fall(activation: Activation, cause: unknown): void {
    this.report('renderer-failed', activation.renderer.id, `Renderer ${activation.renderer.id} failed: ${cause instanceof Error ? cause.message : String(cause)}`)
    this.restore(activation)
  }

  private restore(activation: Activation): void {
    if (this.current !== activation) return
    this.current = undefined
    try { activation.cleanup?.() } catch (cause) {
      this.report('renderer-failed', activation.renderer.id, `Renderer ${activation.renderer.id} cleanup failed: ${cause instanceof Error ? cause.message : String(cause)}`)
    }
    activation.parent.insertBefore(activation.outlet, activation.next && activation.next.parentNode === activation.parent ? activation.next : null)
    activation.root.remove()
    activation.standard.hidden = false
    activation.standard.inert = false
  }

  private report(code: ShellHostDiagnosticCode, renderer: string, message: string): void {
    this.problems.push({ code, renderer, message })
    this.owner.defaultView?.console.warn(message)
  }
}
