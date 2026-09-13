// @vitest-environment jsdom
import { describe, expect, it } from 'vitest'
import { BrowserNavigationService } from './navigation'

describe('browser navigation guard', () => {
  it('blocks same-origin links while dirty and cleans up on dispose', () => {
    const service = new BrowserNavigationService(window)
    const release = service.guard(() => false)
    const link = document.createElement('a')
    link.href = '/next'
    document.body.append(link)
    const blocked = new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 })
    link.dispatchEvent(blocked)
    expect(blocked.defaultPrevented).toBe(true)
    release()
    expect(service.canNavigate()).toBe(true)
    service.dispose()
  })

  it('uses the owning window for links from another browser realm', () => {
    const owner = {
      document,
      location: { origin: 'https://owner.example' },
      addEventListener: window.addEventListener.bind(window),
      removeEventListener: window.removeEventListener.bind(window),
    } as unknown as Window
    const service = new BrowserNavigationService(owner)
    service.guard(() => false)
    const link = document.createElement('a')
    link.href = 'https://owner.example/next'
    document.body.append(link)
    const blocked = new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 })
    link.dispatchEvent(blocked)
    expect(blocked.defaultPrevented).toBe(true)
    service.dispose()
    link.remove()
  })
})
