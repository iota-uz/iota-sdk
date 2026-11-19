import type { Frame, Panel, TableSummary } from '../contract'

/**
 * Footer totals for a table whose complete selection arrived inline in the
 * document. Server-loaded levels and pages carry their own `summary`; an inline
 * frame is the whole selection, so its `total` columns are the sum of its rows.
 * Returns undefined when the panel declares no total column or has no frame.
 */
export function inlineTableSummary(panel: Panel, frame: Frame | undefined): TableSummary | undefined {
  if (panel.kind !== 'table' || panel.deferred || !frame || !panel.columns) return undefined
  const values: Record<string, number> = {}
  for (const column of panel.columns) {
    if (column.total !== true) continue
    const index = frame.columns.findIndex(({ name }) => name === column.field)
    if (index < 0) continue
    let total = 0
    for (const row of frame.rows) {
      const value = row[index]
      if (typeof value === 'number' && Number.isFinite(value)) total += value
    }
    values[column.field] = total
  }
  if (Object.keys(values).length === 0) return undefined
  return { values, filteredRows: frame.rows.length, totalRows: frame.rows.length }
}
