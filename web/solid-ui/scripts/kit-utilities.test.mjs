import { describe, expect, it } from 'vitest'
import { isVariant, lowerKitUtilities, zeroSpecificity } from './kit-utilities.mjs'

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

describe('isVariant', () => {
  it.each([
    ['.hidden', false],
    ['.space-x-2>:not(:last-child)', false],
    ['.\\[--gap\\:4px\\]', false],
    ['.lg\\:block', true],
    ['.hover\\:bg-primary:hover', true],
    ['.dark\\:text-white:where(.dark,.dark *)', true],
  ])('%s → %s', (selector, want) => {
    expect(isVariant(selector)).toBe(want)
  })
})

describe('lowerKitUtilities', () => {
  // Base utilities lose to the host's (so a host `lg:block` beats the kit's
  // `.hidden`); variants keep their weight (so a kit-only `lg:block` still
  // beats the host's `.hidden` at its breakpoint).
  it('lowers base utilities of the utilities layer only, variants keep their specificity', () => {
    const css = [
      '@layer theme{:root{--color-primary:#000}}',
      '@layer utilities{.hidden{display:none}.a,.b::after{color:red}.hover\\:x:hover{color:blue}',
      '@media (min-width:64rem){.lg\\:flex{display:flex}}}',
      '@keyframes spin{to{transform:rotate(1turn)}}',
    ].join('')
    const out = lowerKitUtilities(css)
    expect(out).toContain(':root{--color-primary:#000}')
    expect(out).toContain(':where(.hidden){display:none}')
    expect(out).toContain(':where(.a),:where(.b)::after{color:red}')
    expect(out).toContain('.hover\\:x:hover{color:blue}')
    expect(out).toContain('@media (min-width:64rem){.lg\\:flex{display:flex}}')
    expect(out).toContain('@keyframes spin{to{transform:rotate(1turn)}}')
  })
})
