// Import compiled Tailwind CSS from dist folder (built by build:css script)
import compiledStyles from '../dist/style.css?raw'

export function injectStyles(shadowRoot: ShadowRoot): void {
  const styleElement = document.createElement('style')
  styleElement.textContent = compiledStyles
  shadowRoot.appendChild(styleElement)
}

export function syncDarkMode(element: HTMLElement): MutationObserver {
  const shadowRoot = element.shadowRoot
  if (!shadowRoot) {
    throw new Error('Shadow root not found')
  }

  const reactRoot = shadowRoot.getElementById('react-root')
  if (!reactRoot) {
    throw new Error('React root not found in shadow DOM')
  }

  const syncDarkModeClass = () => {
    if (document.documentElement.classList.contains('dark')) {
      reactRoot.classList.add('dark')
    } else {
      reactRoot.classList.remove('dark')
    }
  }

  syncDarkModeClass()

  const observer = new MutationObserver(syncDarkModeClass)
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['class'],
  })

  return observer
}
