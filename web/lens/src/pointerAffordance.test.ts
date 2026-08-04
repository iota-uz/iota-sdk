import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { sourceFiles } from './test/sourceFiles'

/**
 * Where "this answers a click" is allowed to be said.
 *
 * Preflight is off, so a `<button>` keeps the UA's arrow and only anchors read
 * as clickable for free. The sheet answers that with one list of every control
 * that takes a click (search `lens-cursor-pointer`), and its comment says why:
 * kept in one place, the affordance cannot drift control by control.
 *
 * It drifted anyway. A cascade stage declared its own `style={{ cursor:
 * 'pointer' }}` in TSX and never joined the list — and because the inline
 * declaration satisfied the author that the row looked clickable, it was the one
 * activatable surface in the runtime with no hover state at all: the pointer
 * changed shape and nothing else on the page moved.
 *
 * That is what this test is for. An inline cursor is not a small style; it is a
 * control opting out of the vocabulary, and the opt-out is invisible in review
 * because it reads as the fix.
 */
const inlineCursor = /cursor\s*:\s*['"`]/

describe('pointer affordance', () => {
  it('is declared in the stylesheet, never inline', () => {
    const offenders = sourceFiles('src')
      .filter((file) => inlineCursor.test(readFileSync(file, 'utf8')))
    expect(offenders, 'add the control to the `lens-cursor-pointer` list in styles.css instead').toEqual([])
  })

  it('lists the interactive waterfall column and cascade stage there', () => {
    // Both are divs the panel makes activatable, so neither gets the affordance
    // from its tag; a regex that stopped matching would make the case above
    // pass on an empty sheet.
    const styles = readFileSync('src/styles.css', 'utf8').replace(/\/\*[\s\S]*?\*\//g, '')
    const list = /(?<selector>[^{}]*)\{\s*@apply lens-cursor-pointer;\s*\}/.exec(styles)?.groups?.selector
    expect(list, 'the shared cursor rule is gone or no longer applies only the cursor').toBeDefined()
    expect(list).toContain(".lens-waterfall-column[role='button']")
    expect(list).toContain(".lens-cascade-stage[role='button']")
  })
})
