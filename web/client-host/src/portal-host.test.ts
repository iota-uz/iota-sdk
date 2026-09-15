// @vitest-environment jsdom
import { describe, expect, it } from 'vitest'
import { PortalRegistry } from './portal-host'

describe('PortalRegistry concurrency', () => {
  it('coordinates shared scroll/background locks across registries', () => {
    document.documentElement.style.overflow = 'clip'
    const background = document.createElement('main')
    const firstOwner = document.createElement('div')
    const secondOwner = document.createElement('div')
    document.body.append(background, firstOwner, secondOwner)
    const first = new PortalRegistry(firstOwner, background)
    const second = new PortalRegistry(secondOwner, background)
    const closeFirst = first.mount({ id: 'first', surface: 'modal', restore: null })
    const closeSecond = second.mount({ id: 'second', surface: 'drawer', restore: null })

    closeFirst()
    expect(document.documentElement.style.overflow).toBe('hidden')
    expect(background.inert).toBe(true)
    closeSecond()
    expect(document.documentElement.style.overflow).toBe('clip')
    expect(background.inert).toBe(false)
    document.documentElement.style.overflow = ''
  })

  it('ignores toast order for focus and preserves a removed parent restore target', () => {
    document.body.replaceChildren()
    const owner = document.createElement('div')
    const trigger = document.createElement('button')
    const parentControl = document.createElement('button')
    document.body.append(trigger, owner)
    trigger.focus()
    const registry = new PortalRegistry(owner)
    const closeParent = registry.mount({ id: 'parent', surface: 'modal', restore: trigger })
    owner.append(parentControl)
    parentControl.focus()
    const closeChild = registry.mount({ id: 'child', surface: 'drawer', restore: parentControl })
    const closeToast = registry.mount({ id: 'toast', surface: 'toast', restore: parentControl })

    closeToast()
    expect(document.activeElement).toBe(parentControl)
    parentControl.remove()
    closeParent()
    closeChild()
    expect(document.activeElement).toBe(trigger)
  })
})
