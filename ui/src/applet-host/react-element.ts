import type React from 'react'
import { createRoot, type Root } from 'react-dom/client'

export type RouterMode = 'url' | 'memory'
export type ShellMode = 'embedded' | 'standalone'

export interface AppletHostConfig {
  basePath: string
  shellMode?: ShellMode
  routerMode: RouterMode
  attrs: Record<string, string>
}

export interface DefineReactAppletElementOptions {
  tagName: string
  styles?: string | (() => string)
  render: (host: AppletHostConfig) => React.ReactElement
  observedAttributes?: string[]
  observeDarkMode?: boolean
}

export function defineReactAppletElement(options: DefineReactAppletElementOptions): void {
  if (customElements.get(options.tagName)) return

  const observed = new Set<string>(['base-path', 'shell-mode', 'router-mode'])
  for (const a of options.observedAttributes ?? []) observed.add(a)

  class ReactAppletElement extends HTMLElement {
    private reactRoot: Root | null = null
    private container: HTMLDivElement | null = null
    private darkModeObserver: MutationObserver | null = null

    static get observedAttributes(): string[] {
      return Array.from(observed)
    }

    connectedCallback(): void {
      const shadowRoot = this.shadowRoot ?? this.attachShadow({ mode: 'open' })

      if (!this.container) {
        const styles = typeof options.styles === 'function' ? options.styles() : options.styles
        if (styles) {
          const styleEl = document.createElement('style')
          styleEl.textContent = styles
          shadowRoot.appendChild(styleEl)
        }

        this.container = document.createElement('div')
        this.container.id = 'react-root'
        this.container.style.display = 'flex'
        this.container.style.flexDirection = 'column'
        this.container.style.flex = '1'
        this.container.style.minHeight = '0'
        this.container.style.height = '100%'
        this.container.style.width = '100%'
        shadowRoot.appendChild(this.container)
      } else if (!shadowRoot.querySelector('#react-root')) {
        shadowRoot.appendChild(this.container)
      }

      if (options.observeDarkMode !== false) {
        this.darkModeObserver ??= this.syncDarkMode()
      }

      this.renderReact()
    }

    disconnectedCallback(): void {
      this.darkModeObserver?.disconnect()
      this.darkModeObserver = null

      this.reactRoot?.unmount()
      this.reactRoot = null
    }

    attributeChangedCallback(_name: string, oldValue: string | null, newValue: string | null): void {
      if (oldValue === newValue) return
      if (this.container) this.renderReact()
    }

    private getHostConfig(): AppletHostConfig {
      const attrs: Record<string, string> = {}
      for (const { name, value } of Array.from(this.attributes)) {
        attrs[name] = value
      }

      const basePath = this.getAttribute('base-path') ?? ''
      const shellMode = (this.getAttribute('shell-mode') as ShellMode | null) ?? undefined
      const routerMode = (this.getAttribute('router-mode') as RouterMode | null) ?? 'url'

      return { basePath, shellMode, routerMode, attrs }
    }

    private renderReact(): void {
      if (!this.container) return

      if (!this.reactRoot) {
        this.reactRoot = createRoot(this.container)
      }

      try {
        this.reactRoot.render(options.render(this.getHostConfig()))
      } catch (err) {
        console.error(`[${options.tagName}] failed to mount React app:`, err)
      }
    }

    private syncDarkMode(): MutationObserver {
      const root = this.container
      if (!root) throw new Error('react root container not found')

      const apply = () => {
        if (document.documentElement.classList.contains('dark')) root.classList.add('dark')
        else root.classList.remove('dark')
      }

      apply()

      const observer = new MutationObserver(apply)
      observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
      return observer
    }
  }

  customElements.define(options.tagName, ReactAppletElement)
}
