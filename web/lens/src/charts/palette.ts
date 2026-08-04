import { PALETTE_NEUTRAL, PALETTE_SERIES } from '../contract/palette'

/**
 * The token every collapsed remainder ("Other", the folded tail of a top-N) is
 * painted with. A remainder is not a category, so it must not draw a categorical
 * colour: it is the neutral the palette reserves, and the panel and its legend
 * have to agree on which neutral that is.
 */
export const remainderColorToken = '--lens-text-faint'

/**
 * Value of `remainderColorToken` when no mounted `.lens-root` can be read
 * (server render, a detached frame transform in a test). Generated from
 * `pkg/lens/color.Neutral`, which is also what `styles.css` declares for the
 * token in both themes — a remainder is grey on paper, on a light card and on
 * a dark one.
 */
export const remainderColorFallback = PALETTE_NEUTRAL

/**
 * Resolves the reserved remainder neutral from the live token, so a host that
 * retunes the palette retunes the remainder with it. ECharts paints on canvas
 * and cannot resolve `var()`, which is why this is read rather than emitted.
 */
export function remainderColor(source?: Element | null): string {
  if (typeof globalThis.document === 'undefined' || typeof getComputedStyle !== 'function') {
    return remainderColorFallback
  }
  const root = source?.closest?.('.lens-root') ?? globalThis.document.querySelector('.lens-root')
  if (!root) return remainderColorFallback
  return getComputedStyle(root).getPropertyValue(remainderColorToken).trim() || remainderColorFallback
}

/**
 * The palette drawn when the document carries no `theme.palette` of its own,
 * which is every document today: no host populates it. Generated from
 * `pkg/lens/color`, so the colours are declared once, in Go — assignment stays
 * here (see `paletteAssignment`) because the two hashes are not the same
 * function and moving it would repaint existing dashboards.
 */
export const fallbackSeries = PALETTE_SERIES

export function stablePaletteIndex(key: string, size: number): number {
  if (size <= 0) return 0
  let hash = 0x811c9dc5
  for (let index = 0; index < key.length; index += 1) {
    hash ^= key.charCodeAt(index)
    hash = Math.imul(hash, 0x01000193)
  }
  return (hash >>> 0) % size
}

/**
 * Palette index per label for one chart, with collisions inside that chart
 * resolved.
 *
 * `stablePaletteIndex` alone buys stability across panels — a product keeps its
 * colour wherever it appears — but a hash makes no promise that two *different*
 * labels land on different slots, and with ten colours three categories collide
 * about a quarter of the time. On the profitability cost donut «Операционные
 * расходы» and «Исходящее перестрахование» both hashed to `#dc2626` and are
 * adjacent in the ring, so 97.6% of the figure drew as one unbroken red arc and
 * the two legend swatches were identical: a three-category donut reading as two.
 *
 * Distinctness within one figure is the property a categorical palette exists
 * for, so it wins. Each label still asks for its hashed slot first and almost
 * always gets it; a label that finds its slot taken by an earlier one probes
 * forward to the next free slot. Stability therefore yields only where it must,
 * and only for the later of the two labels. Once every slot is spoken for the
 * hash resolves alone again — more categories than colours is a producer
 * problem no assignment can fix.
 *
 * Order matters and is the frame's own: the same rows in the same order give
 * the same assignment on every render, on the plot and in the legend alike.
 */
export function paletteAssignment(labels: readonly string[], size: number): Map<string, number> {
  const assignment = new Map<string, number>()
  if (size <= 0) return assignment
  const taken = new Set<number>()
  for (const label of labels) {
    if (assignment.has(label)) continue
    const preferred = stablePaletteIndex(label, size)
    let index = preferred
    for (let probe = 0; probe < size && taken.has(index); probe += 1) {
      index = (preferred + probe + 1) % size
    }
    assignment.set(label, index)
    taken.add(index)
  }
  return assignment
}
