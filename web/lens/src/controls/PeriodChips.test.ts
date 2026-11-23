import { afterEach, describe, expect, it } from 'vitest'
import { installKitStyles } from './PeriodChips'

afterEach(() => {
  document.head.innerHTML = ''
})

describe('installKitStyles', () => {
  // The kit sheet goes after the host's, where the runtime installer's
  // constructed sheet lands too: the kit's base utilities are at zero
  // specificity, so host rules win, and its variants keep their weight, so a
  // kit-only `lg:block` still beats a host `.hidden` by order.
  it('appends the kit sheet after the host stylesheets, once', () => {
    const host = document.createElement('link')
    host.rel = 'stylesheet'
    host.href = '/assets/css/main.min.css'
    document.head.append(host)

    installKitStyles('@layer utilities{:where(.hidden){display:none}}')
    installKitStyles('@layer utilities{:where(.hidden){display:none}}')

    expect(document.head.querySelectorAll('style[data-lens-solid-ui]')).toHaveLength(1)
    const kit = document.head.querySelector('style[data-lens-solid-ui]')
    expect(document.head.lastElementChild).toBe(kit)
    expect(host.compareDocumentPosition(kit as Node)).toBe(Node.DOCUMENT_POSITION_FOLLOWING)
  })
})
