import { render } from 'solid-js/web'
import type { Component, JSX } from 'solid-js'
import appletStyles from 'virtual:applet-styles'

export interface SolidAppletContext {
  config: {
    basePath: string
    rpcUIEndpoint: string
    streamEndpoint?: string
    uploadEndpoint?: string
    assetsBasePath?: string
    shellMode?: string
  }
  session?: { csrfToken?: string }
  user?: unknown
  tenant?: unknown
  locale?: { language?: string; translations?: Record<string, string> }
  extensions?: unknown
}

export interface SolidAppletHost {
  basePath: string
  routerMode: 'url' | 'memory'
}

let stylesInjected = false

function injectStylesOnce(): void {
  if (stylesInjected) return
  stylesInjected = true
  const style = document.createElement('style')
  style.dataset.bichatAppletStyles = ''
  style.textContent = appletStyles
  document.head.appendChild(style)
}

/**
 * Mounts a Solid app into an applet custom element, mirroring the React
 * applet bridge contract: `base-path` / `router-mode` attributes in, Solid
 * render + dispose lifecycle out. Rendering stays in the light DOM so the
 * embedded Go shell and global styles keep applying.
 */
export function defineSolidAppletElement(
  tagName: string,
  component: Component<{ host: SolidAppletHost }>,
): void {
  if (customElements.get(tagName)) return

  class SolidAppletRoot extends HTMLElement {
    static get observedAttributes(): string[] {
      return ['base-path', 'router-mode']
    }

    private __dispose?: () => void

    connectedCallback(): void {
      if (this.__dispose) return
      injectStylesOnce()
      const host: SolidAppletHost = {
        basePath: this.getAttribute('base-path') ?? '',
        routerMode: this.getAttribute('router-mode') === 'memory' ? 'memory' : 'url',
      }
      const dispose = render(() => component({ host }) as JSX.Element, this)
      this.__dispose = () => {
        dispose()
        this.__dispose = undefined
      }
    }

    disconnectedCallback(): void {
      this.__dispose?.()
    }
  }

  customElements.define(tagName, SolidAppletRoot)
}

export function readAppletContext(): SolidAppletContext | undefined {
  return (window as { __APPLET_CONTEXT__?: SolidAppletContext }).__APPLET_CONTEXT__
}
