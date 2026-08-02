import { describe, expect, it } from 'vitest'
import { remainderColor, remainderColorFallback, remainderColorToken } from './palette'

describe('the reserved remainder neutral', () => {
  it('reads the live token from the dashboard root', () => {
    const root = document.createElement('div')
    root.className = 'lens-root'
    root.style.setProperty(remainderColorToken, 'rgb(1, 2, 3)')
    document.body.append(root)
    const host = document.createElement('div')
    root.append(host)

    expect(remainderColor(host)).toBe('rgb(1, 2, 3)')

    root.remove()
  })

  it('falls back to the token’s declared value with no root to read', () => {
    expect(remainderColor(document.createElement('div'))).toBe(remainderColorFallback)
  })
})
