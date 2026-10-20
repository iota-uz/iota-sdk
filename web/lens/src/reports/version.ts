import type { DashboardDocument, EffectiveContext, LayoutGroup } from '../contract/types'
import type { Capabilities, Definition, SourceDescriptor, VersionStatus } from './contract/types'

export type ReportVersionStatus = VersionStatus

/** One side of a version comparison: the definition and the source descriptor it resolved against. */
export interface ReportRevision {
  definition: Pick<Definition, 'revision' | 'source' | 'view'>
  source: Pick<SourceDescriptor, 'id' | 'version' | 'revision'> & { capabilities: Pick<Capabilities, 'views'> }
}

/**
 * Mirrors report.CompareVersion. A latest definition that cannot run on the
 * latest source contract or view is incompatible; any other revision change
 * is an update the reader may adopt. The opened result is never altered here.
 */
export function compareReportVersion(opened: ReportRevision, latest: ReportRevision): ReportVersionStatus {
  const definition = latest.definition
  if (definition.source.id !== latest.source.id || definition.source.version !== latest.source.version
    || !latest.source.capabilities.views.includes(definition.view)) return 'incompatible'
  if (definition.revision !== opened.definition.revision
    || definition.source.id !== opened.definition.source.id || definition.source.version !== opened.definition.source.version
    || latest.source.revision !== opened.source.revision || latest.source.version !== opened.source.version) return 'update-available'
  return 'current'
}

export type ReportOperation = 'drill' | 'save' | 'export'
export type ReportActions = Record<ReportOperation, boolean>

/**
 * The actions a placement may offer. An operation is available only when the
 * source declares it and the effective context reports no restriction that
 * the source cannot carry into that operation; the UI hides the rest rather
 * than offering a broader fallback.
 */
export function reportPlacementActions(capabilities: Pick<Capabilities, ReportOperation>, context?: Pick<EffectiveContext, 'conflicts' | 'unsupported' | 'unsupportedOperations'>): ReportActions {
  const blocked = Boolean(context && (context.conflicts.length || context.unsupported.length))
  const allowed = (operation: ReportOperation) => capabilities[operation] && !blocked && !(context?.unsupportedOperations?.[operation]?.length)
  return { drill: allowed('drill'), save: allowed('save'), export: allowed('export') }
}

/**
 * The document without the figures of unavailable placements, for surfaces
 * that cannot render the explanation in place (print). Each withheld group is
 * returned once, in layout order, so the caller can say what was left out.
 */
export function withoutUnavailablePlacements(document: DashboardDocument, unavailable: (group: LayoutGroup) => boolean): { document: DashboardDocument; withheld: Array<LayoutGroup> } {
  const withheld = new Map<string, LayoutGroup>()
  const rows = document.layout.rows.map((row) => ({
    ...row,
    panels: row.panels.filter((item) => {
      const group = (item.groups ?? []).find((candidate) => unavailable(candidate))
      if (!group) return true
      if (!withheld.has(group.id)) withheld.set(group.id, group)
      return false
    }),
  })).filter((row) => row.panels.length > 0)
  if (withheld.size === 0) return { document, withheld: [] }
  const kept = new Set(rows.flatMap((row) => row.panels.map((item) => item.panelId)))
  const removed = new Set(document.layout.rows.flatMap((row) => row.panels.map((item) => item.panelId)).filter((id) => !kept.has(id)))
  return {
    document: { ...document, layout: { ...document.layout, rows }, panels: document.panels.filter((panel) => !removed.has(panel.id)) },
    withheld: [...withheld.values()],
  }
}
