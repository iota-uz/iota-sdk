import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

/**
 * The runtime has one keyboard-focus ring, and this is the test that keeps it
 * that way. It is a stylesheet test rather than a rendered one on purpose: the
 * defect it guards against is a *new rule* somewhere in six thousand lines that
 * answers "where am I?" in its own private way — an accent-100 halo here, an
 * inset shadow there, an `outline-none` that leaves a transparent outline and a
 * 1.03:1 wash standing in for it. That is invisible to any test that renders one
 * component, and obvious here.
 */
const styles = readFileSync('src/styles.css', 'utf8')

/** The sheet without its comments, so a rule's selector is only its selector. */
const declarations = styles.replace(/\/\*[\s\S]*?\*\//g, '')

/** Rules whose selector mentions focus, paired with their declarations. */
function focusRules(): Array<{ selector: string; body: string }> {
  return [...declarations.matchAll(/(?<selector>[^{}]*:focus[^{}]*)\{(?<body>[^}]*)\}/g)]
    .map((match) => ({
      selector: (match.groups?.selector ?? '').trim(),
      body: match.groups?.body ?? '',
    }))
    .filter(({ selector }) => !selector.startsWith('@'))
}

/**
 * Focus treatments that are deliberately not the control ring. Each is a
 * surface rather than a control, and each states its reason in the sheet.
 */
const sanctionedExceptions = [
  // Programmatically focused dialogs: ringing the whole card reads as chrome.
  '.lens-drawer:focus',
  // A drill level marked after a programmatic focus move — a region, not a control.
  '.lens-explore-level:focus',
]

describe('one keyboard-focus ring', () => {
  it('draws every ring from the ring tokens', () => {
    const drawn = focusRules().filter(({ body }) => /outline:|box-shadow:/.test(body))
    const offVocabulary = drawn.filter(({ selector, body }) => (
      !sanctionedExceptions.includes(selector)
      && !(body.includes('var(--lens-focus-ring-color)') && body.includes('var(--lens-focus-ring-width)'))
    ))
    expect(offVocabulary.map(({ selector }) => selector)).toEqual([])
  })

  it('never stands a background, border or shadow in for the ring alone', () => {
    // A focus rule may tint (a container reporting that a child is active); it
    // may not do so *instead* of a ring. `outline-none` is the tell: it suppresses
    // the real ring and leaves a transparent 2px outline where the reader expects
    // one — which is exactly how the table search field ended up unreadable.
    const suppressors = focusRules().filter(({ body }) => /lens-outline-none|outline:\s*none/.test(body))
    expect(suppressors.map(({ selector }) => selector)).toEqual(['.lens-drawer:focus'])
  })

  it('covers form controls, not only buttons and links', () => {
    const shared = focusRules().find(({ selector }) => selector.startsWith('.lens-root :is('))
    expect(shared?.selector).toContain('input')
    expect(shared?.selector).toContain('select')
    expect(shared?.selector).toContain('textarea')
    // :focus-visible, never :focus: a ring on every pointer click is noise, and
    // noise is what gets a ring removed.
    expect(shared?.selector).toContain(':focus-visible')
  })

  it('keeps the inset variant able to win against the shared rule', () => {
    // A `.class:focus-visible` rule scores (0,2,0) and the shared ring scores
    // (0,4,0), so the three inset rules this replaced were dead on arrival: they
    // described a ring the browser never drew. The variant has to match that
    // score and follow it in source order.
    const ring = declarations.indexOf(".lens-root :is(button, a, input, select, textarea, summary, [role='option'], [tabindex])")
    const inset = declarations.indexOf('.lens-root :is(.lens-chart-host-drillable, .lens-table-scroll, .lens-table-overflow-edge)')
    expect(ring).toBeGreaterThan(-1)
    expect(inset).toBeGreaterThan(ring)
    // Nothing may go back to a bare per-element focus rule for these three.
    for (const element of ['.lens-chart-host-drillable', '.lens-table-scroll', '.lens-table-overflow-edge']) {
      expect(declarations).not.toContain(`${element}:focus-visible {`)
    }
  })

  it('states the ring once, in the token block', () => {
    expect(styles).toContain('--lens-focus-ring-color: var(--lens-accent-600);')
    expect(styles).toContain('--lens-focus-ring-width: 2px;')
    expect(styles).toContain('--lens-focus-ring-offset: 2px;')
  })
})
