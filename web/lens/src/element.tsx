import { render } from 'solid-js/web'
import type { JSX } from 'solid-js'
import { LensDashboard } from './LensDashboard'
import { parseDocument, type DashboardDocument } from './contract'
import { normalizeLensTheme } from './runtime'

const tagName = 'lens-dashboard'

export class LensDashboardElement extends HTMLElement {
  static readonly observedAttributes = ['src', 'locale', 'theme', 'csrf', 'initial-document']
  private dispose?: () => void
  private fallbackHTML?: string

  private initialDocument(): DashboardDocument | undefined {
    const encoded = this.getAttribute('initial-document')
    if (!encoded) return undefined
    try {
      const bytes = Uint8Array.from(globalThis.atob(encoded), (character) => character.charCodeAt(0))
      return parseDocument(JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes)))
    } catch (cause) {
      console.error('[lens] embedded initial document is invalid', cause)
      return undefined
    }
  }

  connectedCallback() {
    // Captured before render, which clears the element's children.
    this.fallbackHTML ??= this.innerHTML.trim() || undefined
    if (this.dispose) return
    this.renderDashboard()
  }

  disconnectedCallback() {
    this.dispose?.()
    this.dispose = undefined
  }

  attributeChangedCallback() {
    if (this.isConnected) {
      this.renderDashboard()
    }
  }

  private renderDashboard() {
    this.dispose?.()
    const component = () => (
      <LensDashboard
        src={this.getAttribute('src') ?? undefined}
        locale={this.getAttribute('locale') ?? undefined}
        theme={normalizeLensTheme(this.getAttribute('theme'))}
        csrf={this.getAttribute('csrf') ?? undefined}
        fallbackHTML={this.fallbackHTML}
        initialDocument={this.initialDocument()}
      />
    ) as JSX.Element
    this.dispose = render(component, this)
  }
}

export function registerLensDashboardElement() {
  if (!customElements.get(tagName)) {
    customElements.define(tagName, LensDashboardElement)
  }
}
