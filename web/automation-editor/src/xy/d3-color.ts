// Stands in for d3-color in every bundle of the editor (the package, the fixture, the automations module):
// its parser and the table of named colors were 4 KB gzip of the canvas chunk (#1431) and are never run.
// d3-color reaches the chunk only through d3-interpolate and d3-transition, which @xyflow/system uses for
// one thing: the zoom tween of an animated viewport change, whose values are numbers. With `color` never
// matching, d3's generic interpolators take a string as a string; a color is never interpolated, and if a
// change ever asks for it, the rgb, hsl, lab, hcl and cubehelix constructors fail loudly instead of drawing
// something wrong. scripts/check-chunk.mjs fails if the real d3-color comes back.

/** Never a color: `interpolate(a, b)` and `transition.attr/style` treat every string as a string. */
export function color(_value?: unknown): null {
  return null
}

function unavailable(name: string): never {
  throw new Error(`d3-color ${name} is not bundled with the automation editor (src/xy/d3-color.ts)`)
}

export function rgb(..._args: unknown[]): never {
  return unavailable('rgb')
}
export function hsl(..._args: unknown[]): never {
  return unavailable('hsl')
}
export function lab(..._args: unknown[]): never {
  return unavailable('lab')
}
export function hcl(..._args: unknown[]): never {
  return unavailable('hcl')
}
export function cubehelix(..._args: unknown[]): never {
  return unavailable('cubehelix')
}
