import type { Theme } from '../contract'
import type { PrintReport, PrintSection } from '../runtime/print'
import { indexOf, sectionPanel, text } from './values'

/**
 * One colour per name, for the whole report.
 *
 * On screen a panel is looked at alone, so colouring its rows by position is
 * enough. Bound into one document that stops being true: «Операционные
 * расходы» was green in the result bridge and blue in the expense ring three
 * pages later, and a reader who has learned the first legend is misled by the
 * second. So the report assigns a colour to a *name* once, in reading order,
 * and every figure that mentions that name again uses it.
 *
 * A producer that pinned a colour to a label keeps it: an authored
 * `theme.series[label]` always wins.
 */
export function buildLabelPalette(report: PrintReport): Map<string, string> {
  const theme = report.document.theme
  const palette = Object.values(theme.palette).filter((color) => color.trim() !== '')
  const assigned = new Map<string, string>()
  if (palette.length === 0) return assigned
  let next = 0
  for (const section of report.sections) {
    for (const label of labelsOf(section)) {
      if (!label || assigned.has(label)) continue
      const authored = theme.series[label]
      if (authored) {
        assigned.set(label, theme.palette[authored] ?? authored)
        continue
      }
      assigned.set(label, palette[next % palette.length]!)
      next += 1
    }
  }
  return assigned
}

function labelsOf(section: PrintSection): Array<string> {
  const frame = section.frame
  if (!frame) return []
  const panel = sectionPanel(section)
  const index = indexOf(frame, panel.encoding.label ?? panel.encoding.category ?? panel.encoding.id)
  if (index < 0) return []
  return frame.rows.map((row) => text(row[index]).trim())
}

/**
 * The document theme as the printed report reads it: the shared label palette
 * folded into `theme.series`, with the panel-positional entries dropped so a
 * chart and the table under it cannot disagree about which colour a row is.
 */
export function printTheme(theme: Theme, labels: Map<string, string>): Theme {
  if (labels.size === 0) return theme
  const series: Record<string, string> = {}
  for (const [key, value] of Object.entries(theme.series)) {
    // `panelId:index` entries pin a colour to a row position, which is exactly
    // what the shared palette replaces.
    if (!key.includes(':')) series[key] = value
  }
  for (const [label, color] of labels) series[label] = color
  return { ...theme, series }
}
