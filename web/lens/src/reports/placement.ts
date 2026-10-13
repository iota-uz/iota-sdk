import { unwrap } from 'solid-js/store'
import type { EffectiveContext } from '../contract/types'
import { PlacementSchema } from './contract/schemas'
import { parseReportDefinition, type ReportDefinition, type ReportPlacement } from './draft'

/** A library insertion inherits; an insertion from an opened analysis preserves its selection. */
export function placeLibraryReport(id: string, reportId: string): ReportPlacement {
  return PlacementSchema.parse({ id, reportId })
}

export function placeOpenedReport(id: string, definition: ReportDefinition, context: EffectiveContext): ReportPlacement {
  const report = parseReportDefinition(definition)
  const source = context.input.source
  if (!source || source.id !== report.source.id || source.version !== report.source.version || source.view !== report.view
    || context.conflicts.length || context.unsupported.length) throw new Error('Opened analysis is not placeable')
  return PlacementSchema.parse({ id, reportId: report.id, filters: {
    period: structuredClone(unwrap(context.period.selection)), constraints: structuredClone(unwrap(context.intersection)),
  } })
}
