import { afterEach, describe, expect, it } from 'vitest'
import { installKitStyles } from './PeriodChips'

afterEach(() => {
  document.head.innerHTML = ''
})

describe('installKitStyles', () => {
  // Both sheets are Tailwind builds whose utilities share one cascade layer,
  // where the later rule wins: the kit must sit ahead of the host's sheet so
  // its `.hidden` cannot override the host's `lg:block` (the host sidebar
  // vanished on desktop when the kit was appended).
  it('puts the kit sheet ahead of the host stylesheets, once', () => {
    const host = document.createElement('link')
    host.rel = 'stylesheet'
    host.href = '/assets/css/main.min.css'
    document.head.append(host)

    installKitStyles('@layer utilities{.hidden{display:none}}')
    installKitStyles('@layer utilities{.hidden{display:none}}')

    expect(document.head.querySelectorAll('style[data-lens-solid-ui]')).toHaveLength(1)
    const kit = document.head.querySelector('style[data-lens-solid-ui]')
    expect(document.head.firstElementChild).toBe(kit)
    expect(kit?.compareDocumentPosition(host) ?? 0).toBe(Node.DOCUMENT_POSITION_FOLLOWING)
  })
})
