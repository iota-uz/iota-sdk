import React from 'react'
import { createRoot, Root } from 'react-dom/client'
import App from './App'
import { injectStyles, syncDarkMode } from './styles-injector'

export class ChatRootElement extends HTMLElement {
  private reactRoot: Root | null = null
  private containerElement: HTMLDivElement | null = null
  private darkModeObserver: MutationObserver | null = null

  connectedCallback(): void {
    const shadowRoot = this.attachShadow({ mode: 'open' })

    if (!shadowRoot) {
      console.error('[BiChat] Failed to create shadow root')
      return
    }

    injectStyles(shadowRoot)

    this.containerElement = document.createElement('div')
    this.containerElement.id = 'react-root'
    this.containerElement.style.display = 'flex'
    this.containerElement.style.flexDirection = 'column'
    this.containerElement.style.flex = '1'
    this.containerElement.style.minHeight = '0'
    this.containerElement.style.height = '100%'
    this.containerElement.style.width = '100%'

    shadowRoot.appendChild(this.containerElement)

    this.darkModeObserver = syncDarkMode(this)
    this.mountReactApp()
  }

  disconnectedCallback(): void {
    if (this.reactRoot) {
      this.reactRoot.unmount()
      this.reactRoot = null
    }
    if (this.darkModeObserver) {
      this.darkModeObserver.disconnect()
      this.darkModeObserver = null
    }
  }

  static get observedAttributes(): string[] {
    return ['session-id', 'base-path']
  }

  attributeChangedCallback(_name: string, oldValue: string | null, newValue: string | null): void {
    if (oldValue === newValue) return
    if (this.containerElement && this.reactRoot) {
      this.mountReactApp()
    }
  }

  private mountReactApp(): void {
    if (!this.containerElement) return

    if (this.reactRoot) {
      this.reactRoot.unmount()
    }

    this.reactRoot = createRoot(this.containerElement)

    try {
      this.reactRoot.render(React.createElement(App))
    } catch (error) {
      console.error('[BiChat] Failed to mount React app:', error)
    }
  }

  getSessionId(): string | null {
    return this.getAttribute('session-id')
  }

  getBasePath(): string {
    return this.getAttribute('base-path') || '/bi-chat'
  }
}

export function registerChatRootElement(): void {
  if (!customElements.get('bi-chat-root')) {
    customElements.define('bi-chat-root', ChatRootElement)
  }
}
