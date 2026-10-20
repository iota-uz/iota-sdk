import type { EffectiveContext } from '../contract/types'
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
