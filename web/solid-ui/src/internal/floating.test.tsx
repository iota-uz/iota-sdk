// @vitest-environment jsdom
import { createSignal } from 'solid-js'
import { render } from 'solid-js/web'
import { afterEach, describe, expect, it } from 'vitest'
import { createFloatingPlacement } from './floating'
import type { FloatingPlacement } from './floating'

let dispose: (() => void) | undefined

afterEach(() => {
  dispose?.()
  dispose = undefined
  document.body.replaceChildren()
})

describe('createFloatingPlacement', () => {
  it('uses the anchor document viewport instead of the global document', async () => {
    Object.defineProperty(document.documentElement, 'clientWidth', { configurable: true, value: 1000 })
    const frame = document.createElement('iframe')
    document.body.append(frame)
    const owner = frame.contentDocument!
    Object.defineProperty(owner.documentElement, 'clientWidth', { configurable: true, value: 300 })
    Object.defineProperty(owner.documentElement, 'clientHeight', { configurable: true, value: 500 })
    let placement!: () => FloatingPlacement

    dispose = render(() => {
      const [open] = createSignal(true)
      let anchor: HTMLButtonElement | undefined
      let floating: HTMLDivElement | undefined
      const state = createFloatingPlacement(open, () => anchor, () => floating)
      placement = state.placement
      return <>
        <button ref={(element) => {
          anchor = element
          element.getBoundingClientRect = () => ({ left: 250, right: 270, top: 20, bottom: 40, width: 20, height: 20, x: 250, y: 20, toJSON: () => ({}) })
        }} />
        <div ref={(element) => {
          floating = element
          element.getBoundingClientRect = () => ({ left: 0, right: 100, top: 0, bottom: 100, width: 100, height: 100, x: 0, y: 0, toJSON: () => ({}) })
        }} />
      </>
    }, owner.body)

    await Promise.resolve()
    await Promise.resolve()
    expect(placement().align).toBe('end')
  })
})
