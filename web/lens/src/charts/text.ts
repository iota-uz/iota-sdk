/**
 * How wide a string is when the chart draws it.
 *
 * Character count was the runtime's proxy for width, and it is a bad one on
 * this product's data: «Ш» and «і» count the same, Cyrillic product names run
 * to five times the width of a quarter label, and a rule calibrated on one is
 * wrong about the other. Canvas measures the string with the font it will
 * actually be painted in, which is the only measurement that can decide whether
 * a label fits the space it is offered.
 */

let measuringContext: CanvasRenderingContext2D | null | undefined

function context(): CanvasRenderingContext2D | null {
  if (measuringContext !== undefined) return measuringContext
  try {
    measuringContext = typeof globalThis.document === 'undefined'
      ? null
      : globalThis.document.createElement('canvas').getContext('2d')
  } catch {
    // jsdom, and any host without a 2D backend. The estimate below stands in.
    measuringContext = null
  }
  return measuringContext
}

/**
 * Mean glyph advance as a fraction of the em, for environments with no canvas
 * (the server render, jsdom under test). It is deliberately generous: a fit
 * rule that guesses narrow drops labels that would have fitted, and a rule that
 * guesses wide clips them — and this fallback only ever runs where nothing is
 * painted, so erring towards "keep the label" is free.
 */
export const averageGlyphWidthRatio = 0.58

/** Width of `value` in pixels at `fontSize`, in the runtime's own font. */
export function measureTextWidth(value: string, fontSize: number, fontFamily: string): number {
  const fallback = value.length * fontSize * averageGlyphWidthRatio
  const ctx = context()
  if (!ctx) return fallback
  ctx.font = `${fontSize}px ${fontFamily}`
  const measured = ctx.measureText(value).width
  return Number.isFinite(measured) && measured > 0 ? measured : fallback
}

/** Test seam: forget the cached context so a stubbed canvas takes effect. */
export function resetTextMeasurement(): void {
  measuringContext = undefined
}
