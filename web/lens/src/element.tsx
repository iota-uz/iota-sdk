import { createRoot, type Root } from 'react-dom/client'
import { LensDashboard } from './LensDashboard'

const tagName = 'lens-dashboard'

export class LensDashboardElement extends HTMLElement {
  static readonly observedAttributes = ['src', 'locale', 'theme', 'csrf']

  private root?: Root

  connectedCallback() {
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
        theme={this.getAttribute('theme') === 'dark' ? 'dark' : 'light'}
        csrf={this.getAttribute('csrf') ?? undefined}
      />,
    )
  }
}

export function registerLensDashboardElement() {
  if (!customElements.get(tagName)) {
    customElements.define(tagName, LensDashboardElement)
  }
}
