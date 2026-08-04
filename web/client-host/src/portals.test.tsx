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
})
