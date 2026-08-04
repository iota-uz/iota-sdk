import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

/**
 * Where a row's slack goes, guarded in the sheet rather than in a render.
 *
 * A row stretches every card to its tallest sibling. That is deliberate and
 * stays. What must not stretch is the content: a content root that keeps its
 * size and only moves (`flex: 1` plus a vertical centring) spends every extra
 * pixel of card height moving itself down, which is how a stat's delta chip
 * came to float in the middle of an over-tall card, and how the gauge dial and
 * the relationship diagram still did after the stat was fixed one kind at a
 * time.
 *
 * jsdom has no layout, so this is a stylesheet test on purpose: the defect it
 * guards is the *shape* of the rule — one contract keyed on the content that
 * cannot spend the height, not a growing list of `data-panel-kind` exceptions,
 * and never the blanket form that stops a plot or a table from filling the card
 * it was given.
 */
const styles = readFileSync('src/styles.css', 'utf8')
const declarations = styles.replace(/\/\*[\s\S]*?\*\//g, '')

/** Every rule in the sheet, selector and body. */
function rules(): Array<{ selector: string; body: string }> {
  return [...declarations.matchAll(/(?<selector>[^{}]*)\{(?<body>[^{}]*)\}/g)]
    .map((match) => ({ selector: (match.groups?.selector ?? '').trim(), body: match.groups?.body ?? '' }))
    .filter(({ selector }) => !selector.startsWith('@') && selector !== '')
}

/** The rules that stop a panel body from taking the row's slack. */
function slackRules(): Array<{ selector: string; body: string }> {
  return rules().filter(({ selector, body }) => selector.includes('.lens-panel-body')
    && /flex-none|flex:\s*none/.test(body))
}

describe('the row’s slack collects at the bottom of the card', () => {
  it('names the content that cannot spend the height, not the panel kinds', () => {
    const covered = slackRules().map(({ selector }) => selector).join(' ')
    // A figure whose parts never change size: it is drawn once and then only
    // moved, so the body must take the height it needs and no more.
    expect(covered).toContain('.lens-stat-content')
    expect(covered).toContain('.lens-gauge')
    expect(covered).toContain('.lens-relationship')
    // The same fault painted: the flow's stage slabs stretched to 725px each
    // around one line of text.
    expect(covered).toContain('.lens-flow')
    // Keyed on the content, so the same figure drawn inside a metrics group
    // card — which carries no panel kind of its own — is covered too.
    expect(covered).not.toMatch(/data-panel-kind/)
    expect(covered).not.toMatch(/\.lens-panel-stat\s*>/)
  })

  it('never stops a body that fills from filling', () => {
    // The blanket form — every body top-aligned — costs a plot in an 800px card
    // some 430px of chart to a hole, because a chart host falls back to its
    // 19rem floor. Nothing here may match a body unconditionally.
    for (const { selector } of slackRules()) {
      for (const part of selector.split(',').map((entry) => entry.trim()).filter(Boolean)) {
        expect(part).toMatch(/:has\(/)
      }
    }
  })
})
