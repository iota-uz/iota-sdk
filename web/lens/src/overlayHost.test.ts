import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { sourceFiles } from './test/sourceFiles'

/**
 * Body-level overlay hosts, and the one mistake they keep making.
 *
 * `useOverlayContainer` builds a `fixed inset-0` div so a portalled popover can
 * inherit the dashboard's theme variables. That div is also, by default, a
 * transparent hit target over the entire viewport — which is harmless for a
 * click-opened overlay that wants outside clicks, and fatal for a hover-opened
 * one: the layer takes the pointer the instant the overlay mounts, the trigger
 * gets `mouseleave` under a cursor that never moved, the overlay closes,
 * the layer goes away, and the whole thing runs again. It flickers, and every
 * click aimed at the surface underneath lands on the layer.
 *
 * That bug has been found and fixed three times — on the info tip, on the panel
 * menu, and on the waterfall tip. The rule this test enforces is what stops a
 * fourth: **naming your overlay host is declaring it non-blocking**. Pass an
 * extra class and it must opt out of hit testing, with the floating card itself
 * opting back in. A host that genuinely wants to swallow the page — a drawer, a
 * panel overlay, a filter popover that closes on an outside click — passes no
 * class and stays on the default.
 */
const styles = readFileSync('src/styles.css', 'utf8')

/** The sheet without its comments, so a rule is only its rule. */
const declarations = styles.replace(/\/\*[\s\S]*?\*\//g, '')

/** Every host class named at a `useOverlayContainer` call site. */
function namedHostClasses(): Array<string> {
  const names = new Set<string>()
  for (const file of sourceFiles('src')) {
    const source = readFileSync(file, 'utf8')
    for (const match of source.matchAll(/useOverlayContainer\(\s*[^)]*?'(?<name>lens-[\w-]*overlay-root)'/g)) {
      const name = match.groups?.name
      if (name) names.add(name)
    }
  }
  return [...names].sort()
}

/**
 * Everything the sheet declares for exactly `.className`, joined. A class is
 * routinely stated more than once — its own block, plus a shared rule it sits in
 * a selector list with — so reading only the first match answers about whichever
 * rule happens to come first in the file.
 */
function declarationsFor(className: string): string | undefined {
  const bodies = [...declarations.matchAll(/(?<selector>[^{}]*)\{(?<body>[^{}]*)\}/g)]
    .filter(({ groups }) => (groups?.selector ?? '')
      .split(',')
      .some((one) => one.trim() === `.${className}`))
    .map(({ groups }) => groups?.body ?? '')
  return bodies.length > 0 ? bodies.join('\n') : undefined
}

describe('body-level overlay hosts', () => {
  it('names at least the hosts this rule was written for', () => {
    // A regex that silently matches nothing would make every case below vacuous.
    expect(namedHostClasses()).toEqual(expect.arrayContaining([
      'lens-info-tip-overlay-root',
      'lens-menu-overlay-root',
      'lens-waterfall-tip-overlay-root',
    ]))
  })

  it.each(namedHostClasses())('%s opts out of hit testing', (name) => {
    const body = declarationsFor(name)
    expect(body, `${name} is passed to useOverlayContainer but styled nowhere`).toBeDefined()
    expect(body).toMatch(/pointer-events:\s*none/)
  })

  it.each([
    'lens-info-tip-bubble',
    'lens-export-menu',
    'lens-waterfall-tip',
  ])('%s opts back in', (card) => {
    // The host must not block; the card on it must still answer a pointer, or
    // its own buttons and its selectable text are dead.
    expect(declarationsFor(card), `${card} sits on a non-blocking host and must re-enable pointer events`)
      .toMatch(/pointer-events:\s*auto|lens-pointer-events-auto/)
  })
})
