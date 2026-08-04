// @vitest-environment jsdom
import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ClientHostProvider, Portal, WidgetSlot } from './portals'

describe('ClientHostProvider', () => {
  it('keeps widgets separate and owns overlay lifecycle, inert and scroll lock', () => {
    const portalOwner = document.createElement('div')
    const background = document.createElement('main')
    const widget = document.createElement('div')
    document.body.append(background, portalOwner, widget)
    const view = render(
      <ClientHostProvider portalOwner={portalOwner} background={background} widgetSlots={{ 'header-actions': widget }} theme="dark">
        <WidgetSlot name="header-actions"><button>Widget</button></WidgetSlot>
        <Portal surface="drawer" label="Details"><button>Close</button></Portal>
      </ClientHostProvider>,
    )
    expect(widget.textContent).toBe('Widget')
    expect(portalOwner.querySelector('[data-iota-portal="drawer"]')).not.toBeNull()
    expect(portalOwner.dataset.theme).toBe('dark')
    expect(background.inert).toBe(true)
    expect(document.documentElement.style.overflow).toBe('hidden')
    view.unmount()
    expect(background.inert).toBe(false)
    expect(document.documentElement.style.overflow).toBe('')
    expect(portalOwner.childElementCount).toBe(0)
  })

  it('preserves shell state and restores focus only when the top overlay closes', () => {
    const portalOwner = document.createElement('div')
    const background = document.createElement('main')
    const firstTrigger = document.createElement('button')
    const secondTrigger = document.createElement('button')
    background.inert = true
    background.setAttribute('aria-hidden', 'false')
    document.documentElement.style.overflow = 'clip'
    document.body.append(background, portalOwner, firstTrigger, secondTrigger)
    firstTrigger.focus()

    const tree = (first: boolean, second: boolean) => (
      <ClientHostProvider portalOwner={portalOwner} background={background}>
        {first && <Portal key="first" surface="drawer" label="First"><button>First close</button></Portal>}
        {second && <Portal key="second" surface="modal" label="Second"><button>Second close</button></Portal>}
      </ClientHostProvider>
    )
    const view = render(tree(true, false))
    secondTrigger.focus()
    view.rerender(tree(true, true))
    const secondClose = portalOwner.querySelector<HTMLButtonElement>('[data-iota-surface="modal"] button')
    expect(document.activeElement).toBe(secondClose)

    view.rerender(tree(false, true))
    expect(document.activeElement).toBe(secondClose)
    view.rerender(tree(false, false))
    expect(document.activeElement).toBe(secondTrigger)
    expect(document.documentElement.style.overflow).toBe('clip')
    expect(background.inert).toBe(true)
    expect(background.getAttribute('aria-hidden')).toBe('false')
    view.unmount()
    document.documentElement.style.overflow = ''
  })
})
