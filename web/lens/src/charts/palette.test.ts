import { describe, expect, it } from 'vitest'
import {
  fallbackSeries, paletteAssignment, remainderColor, remainderColorFallback,
  remainderColorToken, stablePaletteIndex,
} from './palette'

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

describe('palette assignment within one figure', () => {
  // The three slices of the profitability cost donut. The first two hash to the
  // same slot, and they are adjacent in the ring, which is how 97.6% of that
  // figure came to draw as one unbroken red arc.
  const costCategories = ['Операционные расходы', 'Исходящее перестрахование', 'Аквизиционные расходы']
  const size = fallbackSeries.length

  it('records that the hash alone collides on the case this exists for', () => {
    expect(stablePaletteIndex(costCategories[0]!, size)).toBe(stablePaletteIndex(costCategories[1]!, size))
  })

  it('gives every category in one figure a slot of its own', () => {
    const assigned = paletteAssignment(costCategories, size)
    expect(new Set(assigned.values()).size).toBe(costCategories.length)
  })

  it('keeps the hashed slot for the first claimant, so a colour is stable where it can be', () => {
    const assigned = paletteAssignment(costCategories, size)
    expect(assigned.get(costCategories[0]!)).toBe(stablePaletteIndex(costCategories[0]!, size))
  })

  it('is decided by the frame’s order, so plot and legend agree', () => {
    const once = paletteAssignment(costCategories, size)
    const again = paletteAssignment(costCategories, size)
    expect([...again]).toEqual([...once])
  })

  it('falls back to the bare hash once every slot is claimed', () => {
    const labels = Array.from({ length: size + 2 }, (_, index) => `category-${index}`)
    const assigned = paletteAssignment(labels, size)
    expect(assigned.size).toBe(labels.length)
    expect(new Set(assigned.values()).size).toBe(size)
  })

  it('has nothing to say about an empty palette', () => {
    expect(paletteAssignment(costCategories, 0).size).toBe(0)
  })
})
