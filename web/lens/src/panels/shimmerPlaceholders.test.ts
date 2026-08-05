import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

/**
 * What a loading placeholder is allowed to measure itself against.
 *
 * A percentage width is only a width when the box it is stated against has one.
 * The strip's figure slot does not: `.lens-stat-metric-value` is a flex item
 * that shrinks and never grows, so before its figure arrives it holds no text
 * and is zero pixels wide — and `width: 60%` of zero is zero. The placeholder
 * disappeared, the reserved line height stayed, and every loading KPI strip
 * read as a row of names over an empty gap, which is the "no data" it exists to
 * distinguish itself from.
 *
 * jsdom has no layout, so the pixels are asserted in the VR lane ("a loading
 * KPI cell shows a placeholder the size of the figure it stands in for"). What
 * is held here is the shape of the mistake: a placeholder may size itself in
 * percent only inside a container that is known to have a width of its own.
 */
const declarations = readFileSync('src/styles.css', 'utf8').replace(/\/\*[\s\S]*?\*\//g, '')

function rules(): Array<{ selector: string; body: string }> {
  return [...declarations.matchAll(/(?<selector>[^{}]*)\{(?<body>[^{}]*)\}/g)]
    .map((match) => ({ selector: (match.groups?.selector ?? '').trim(), body: match.groups?.body ?? '' }))
    .filter(({ selector }) => !selector.startsWith('@') && selector !== '')
}

/** `width`, not `max-width` or `min-width`. */
function width(body: string): string | undefined {
  return /(?:^|[;{\s])width:\s*([^;]+)/.exec(body)?.[1]?.trim()
}

/**
 * Containers whose own width is settled by the grid or the card around them, so
 * a percentage stated inside them resolves to something. `.lens-skeleton-card`
 * is a column flex container: width is its cross axis, and its children stretch
 * to the card rather than to their own contents.
 */
const definiteContainers = ['.lens-skeleton-card']

describe('loading placeholders', () => {
  it('sizes the strip’s figure placeholder in its own units, not the empty slot’s', () => {
    const slot = rules().find(({ selector }) => selector === '.lens-stat-metric-value')?.body ?? ''
    const placeholder = rules().find(({ selector }) => selector === '.lens-stat-metric-value-shimmer')?.body ?? ''
    expect(slot).not.toBe('')
    expect(placeholder).not.toBe('')

    // Either the slot claims width of its own while it is empty, or the
    // placeholder brings its own. One of the two has to be true; the sheet
    // shipped with neither.
    const slotClaimsWidth = /(?:^|[;{\s])flex(?:-grow)?:\s*[1-9]/.test(slot)
      || /@apply[^;]*\blens-(?:flex-1|grow)\b/.test(slot)
    const placeholderWidth = width(placeholder)
    expect(placeholderWidth).toBeDefined()
    expect(slotClaimsWidth || !placeholderWidth?.endsWith('%')).toBe(true)
  })

  it('states a percentage width only inside a container that has one', () => {
    const percentages = rules().filter(({ selector, body }) => (
      selector.includes('shimmer') && (width(body)?.endsWith('%') ?? false)
    ))

    for (const { selector } of percentages) {
      for (const part of selector.split(',').map((entry) => entry.trim()).filter(Boolean)) {
        // The ancestor may carry an attribute or state of its own
        // (`.lens-skeleton-card[data-kind="stat"]`); the container class is
        // what says the width is settled.
        const ancestors = part.split(/\s*>\s*|\s+/).slice(0, -1)
        expect(ancestors.some((ancestor) => definiteContainers.some((container) => ancestor.startsWith(container))))
          .toBe(true)
      }
    }
  })
})
