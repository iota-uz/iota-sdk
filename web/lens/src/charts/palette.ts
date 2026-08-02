/**
 * The token every collapsed remainder ("Other", the folded tail of a top-N) is
 * painted with. A remainder is not a category, so it must not draw a categorical
 * colour: it is the neutral the palette reserves, and the panel and its legend
 * have to agree on which neutral that is.
 */
export const remainderColorToken = '--lens-text-faint'

/**
 * Value of `remainderColorToken` when no mounted `.lens-root` can be read
 * (server render, a detached frame transform in a test). Kept equal to the
 * token's own declaration in `styles.css`, where both themes state the same
 * neutral — a remainder is grey on paper, on a light card and on a dark one.
 */
export const remainderColorFallback = '#94a3b8'

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

export const fallbackSeries = [
  '#2563eb', '#0d9488', '#d97706', '#7c3aed', '#dc2626',
  '#0284c7', '#db2777', '#65a30d', '#9333ea', '#64748b',
] as const

export function stablePaletteIndex(key: string, size: number): number {
  if (size <= 0) return 0
  let hash = 0x811c9dc5
  for (let index = 0; index < key.length; index += 1) {
    hash ^= key.charCodeAt(index)
    hash = Math.imul(hash, 0x01000193)
  }
  return (hash >>> 0) % size
}
