import { createRoot, type Root } from 'react-dom/client'
import { LensDashboard } from './LensDashboard'
import { normalizeLensTheme } from './runtime'

const tagName = 'lens-dashboard'

export class LensDashboardElement extends HTMLElement {
  static readonly observedAttributes = ['src', 'locale', 'theme', 'csrf']
  private root?: Root
  private fallbackHTML?: string

  connectedCallback() {
    // Captured before createRoot, which clears the element's children.
    this.fallbackHTML ??= this.innerHTML.trim() || undefined
    this.root ??= createRoot(this)
    this.renderDashboard()
  }

  disconnectedCallback() {
    this.root?.unmount()
    this.root = undefined
  }

  attributeChangedCallback() {
    if (this.isConnected) {
      this.renderDashboard()
    }
  }

  private renderDashboard() {
    this.root?.render(
      <LensDashboard
        src={this.getAttribute('src') ?? undefined}
        locale={this.getAttribute('locale') ?? undefined}
        theme={normalizeLensTheme(this.getAttribute('theme'))}
        csrf={this.getAttribute('csrf') ?? undefined}
        fallbackHTML={this.fallbackHTML}
      />,
    )
  }
}

export function registerLensDashboardElement() {
  if (!customElements.get(tagName)) {
    customElements.define(tagName, LensDashboardElement)
  }
}
