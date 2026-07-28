import type { Availability, Confidence } from '../contract'
import type { PrintFigure } from './outline'
import { qualityFootnote, type Translate } from './quality'
import { indexOf, sectionPanel, text } from './values'

/**
 * Footnotes for a printed chapter.
 *
 * A dashboard keeps its qualifications behind hover: the ⓘ explaining how a
 * figure is obtained, the chip saying it is an estimate, the small badge on a
 * table row saying that row is unmatched. Paper has no hover, and a reader
 * cannot ask. So each qualification is numbered where it applies and printed in
 * full under the first figure that carries it — the same page, not the back of
 * the report, and once per chapter rather than under every figure that repeats
 * the same caveat.
 */

export interface PrintFootnote {
  number: number
  text: string
}

export interface FigureFootnotes {
  /** Numbers printed as markers beside this figure's heading. */
  markers: Array<number>
  /** Notes printed in full under this figure — those it introduced. */
  notes: Array<PrintFootnote>
}

/** Every quality a figure declares structurally, before any frame overrides. */
function declaredQualities(figure: PrintFigure): Array<{ confidence?: Confidence; availability?: Availability }> {
  const panel = sectionPanel(figure.section)
  const declared: Array<{ confidence?: Confidence; availability?: Availability }> = [panel]
  for (const stage of panel.metricFlow?.stages ?? []) declared.push(stage)
  for (const row of panel.metricHierarchy?.rows ?? []) declared.push(row)
  if (panel.metricRelationship) {
    declared.push(panel.metricRelationship.source, panel.metricRelationship.target)
  }
  return declared
}

/** Producer hints attached to individual table rows, shown on screen as a badge. */
function badgeHints(figure: PrintFigure): Array<string> {
  const panel = sectionPanel(figure.section)
  const frame = figure.section.frame
  if (!frame || !panel.columns?.length) return []
  const hints: Array<string> = []
  for (const column of panel.columns) {
    const index = indexOf(frame, column.badgeField)
    if (index < 0) continue
    for (const row of frame.rows) {
      const hint = text(row[index]).trim()
      if (hint && hint !== '—') hints.push(hint)
    }
  }
  return hints
}

/**
 * Assigns footnote numbers over a chapter's figures in printing order. The same
 * caveat carried by three figures is one number, printed once.
 */
export function buildChapterFootnotes(
  figures: Array<PrintFigure>,
  translate: Translate,
): Map<string, FigureFootnotes> {
  const numbers = new Map<string, number>()
  const result = new Map<string, FigureFootnotes>()
  for (const figure of figures) {
    const panel = sectionPanel(figure.section)
    const texts: Array<string> = []
    const info = panel.info?.trim()
    if (info) texts.push(info)
    for (const declared of declaredQualities(figure)) {
      const note = qualityFootnote(declared, translate)
      if (note) texts.push(note)
    }
    texts.push(...badgeHints(figure))

    const entry: FigureFootnotes = { markers: [], notes: [] }
    for (const value of texts) {
      const existing = numbers.get(value)
      if (existing !== undefined) {
        if (!entry.markers.includes(existing)) entry.markers.push(existing)
        continue
      }
      const number = numbers.size + 1
      numbers.set(value, number)
      entry.markers.push(number)
      entry.notes.push({ number, text: value })
    }
    if (entry.markers.length > 0) result.set(figure.section.id, entry)
  }
  return result
}
