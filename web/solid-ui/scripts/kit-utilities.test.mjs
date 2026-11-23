import { describe, expect, it } from 'vitest'
import { lowerKitUtilities, zeroSpecificity } from './kit-utilities.mjs'

describe('zeroSpecificity', () => {
  it.each([
    ['.hidden', ':where(.hidden)'],
    ['.hover\\:bg-primary:hover', ':where(.hover\\:bg-primary:hover)'],
    ['.placeholder\\:text-gray::placeholder', ':where(.placeholder\\:text-gray)::placeholder'],
    ['.before\\:content-\\[\\"x\\"\\]::before', ':where(.before\\:content-\\[\\"x\\"\\])::before'],
    ['.space-x-2>:not(:last-child)', ':where(.space-x-2>:not(:last-child))'],
    [':where(.already)', ':where(.already)'],
  ])('%s → %s', (selector, want) => {
    expect(zeroSpecificity(selector)).toBe(want)
  })
})

describe('lowerKitUtilities', () => {
  it('lowers only the utilities layer, media variants included', () => {
    const css = [
      '@layer theme{:root{--color-primary:#000}}',
      '@layer utilities{.hidden{display:none}.a,.b::after{color:red}@media (min-width:64rem){.lg\\:flex{display:flex}}}',
      '@keyframes spin{to{transform:rotate(1turn)}}',
    ].join('')
    const out = lowerKitUtilities(css)
    expect(out).toContain(':root{--color-primary:#000}')
    expect(out).toContain(':where(.hidden){display:none}')
    expect(out).toContain(':where(.a),:where(.b)::after{color:red}')
    expect(out).toContain('@media (min-width:64rem){:where(.lg\\:flex){display:flex}}')
    expect(out).toContain('@keyframes spin{to{transform:rotate(1turn)}}')
  })
})
