/**
 * Queries for a chart's legend, scoped to the legend.
 *
 * A legend row's accessible name is the series name, and so is the name of
 * every other control that mentions that series — the mark key beside it, the
 * facet chip above it. Reaching for `getByRole('button', { name: /Broker/ })`
 * therefore breaks the moment any of them changes, and answering that with
 * `getAllByRole(...)[1]` only hides which control the test meant. These
 * helpers say which one: a row of the data legend, or a member of the mark key.
 */

/** The data legend — the rows that are slices of the frame. */
function legendList(root: ParentNode): Element {
  const list = [...root.querySelectorAll('.lens-chart-legend')]
    .find((element) => !element.classList.contains('lens-chart-overlay-legend'))
  if (!list) throw new Error('no chart legend rendered')
  return list
}

function rowIn(list: ParentNode, name: string): HTMLButtonElement {
  const rows = [...list.querySelectorAll<HTMLButtonElement>('.lens-chart-legend-toggle')]
  const label = (row: Element) => row.querySelector('.lens-chart-legend-label')?.textContent?.trim() ?? ''
  const exact = rows.filter((row) => label(row) === name)
  const matches = exact.length > 0 ? exact : rows.filter((row) => label(row).includes(name))
  if (matches.length === 0) {
    throw new Error(`no legend row named "${name}"; rendered: ${rows.map(label).join(', ') || '(none)'}`)
  }
  if (matches.length > 1) {
    throw new Error(`"${name}" matches ${matches.length} legend rows: ${matches.map(label).join(', ')}`)
  }
  return matches[0]!
}

/** The data-legend row for a series, by its rendered label. */
export function legendRow(root: ParentNode, name: string): HTMLButtonElement {
  return rowIn(legendList(root), name)
}

/** A member of the mark key — a threshold, an event, a forecast, the ghost. */
export function markRow(root: ParentNode, name: string): HTMLButtonElement {
  const key = root.querySelector('.lens-chart-overlay-legend')
  if (!key) throw new Error('no chart mark key rendered')
  return rowIn(key, name)
}

/** Every data-legend label, in render order. */
export function legendLabels(root: ParentNode): string[] {
  return [...legendList(root).querySelectorAll('.lens-chart-legend-label')]
    .map((node) => node.textContent?.trim() ?? '')
}

/** Every data-legend value cell, in render order. */
export function legendValues(root: ParentNode): string[] {
  return [...legendList(root).querySelectorAll('.lens-chart-legend-value')]
    .map((node) => node.textContent?.trim() ?? '')
}

/** Each row's swatch ink, in render order, as the browser resolved it. */
export function legendSwatches(root: ParentNode): string[] {
  return [...legendList(root).querySelectorAll<HTMLElement>('.lens-chart-legend-mark')]
    .map((node) => getComputedStyle(node).backgroundColor)
}
