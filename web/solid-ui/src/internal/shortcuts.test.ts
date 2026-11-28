import { afterEach, describe, expect, it } from 'vitest'
import { isShortcutClaimed } from './shortcuts'

const press = (target: EventTarget) => {
  const event = new KeyboardEvent('keydown', { key: 'k', metaKey: true, bubbles: true, cancelable: true })
  let claimed: boolean | null = null
  document.addEventListener('keydown', (e) => { claimed = isShortcutClaimed(e, 'mod+k') }, { once: true })
  target.dispatchEvent(event)
  return claimed
}

describe('isShortcutClaimed', () => {
  afterEach(() => { document.body.innerHTML = '' })

  it('is claimed inside a surface that claims it, not outside it', () => {
    document.body.innerHTML = '<div data-claim-shortcuts="mod+s mod+k"><button id="in"></button></div><button id="out"></button>'
    expect(press(document.getElementById('in')!)).toBe(true)
    expect(press(document.getElementById('out')!)).toBe(false)
  })

  it('with nothing focused, a claiming surface on the page holds it', () => {
    document.body.innerHTML = '<div data-claim-shortcuts="mod+k"></div>'
    expect(press(document.body)).toBe(true)
    document.body.innerHTML = '<div data-claim-shortcuts="mod+z"></div>'
    expect(press(document.body)).toBe(false)
  })

  it('an event another handler took is claimed', () => {
    document.body.innerHTML = '<button id="b"></button>'
    const b = document.getElementById('b')!
    b.addEventListener('keydown', (e) => e.preventDefault(), { once: true })
    expect(press(b)).toBe(true)
  })
})
