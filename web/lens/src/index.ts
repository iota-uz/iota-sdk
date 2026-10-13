// The public stylesheet is a first-class package artifact. Vite's library
// build only emits style.css for CSS reachable from the entry graph; consumers
// still opt into it explicitly through `@iota-uz/sdk/lens/styles.css`.
import './styles.css'

export {
  SDK_IDENTITY,
  SDK_PROTOCOL_VERSION,
  SDK_RELEASE_VERSION,
  SDK_SOURCE_COMMIT,
  SDKIdentityMismatchError,
  assertSDKIdentity,
} from '@iota-uz/sdk/client-host'

export { LensDashboard, type LensDashboardProps, type LensToolbarContext } from './LensDashboard'
export { LensDashboardElement, registerLensDashboardElement } from './element'
export {
  CONTRACT_VERSION,
  ContractVersionMismatchError,
  DashboardDocumentSchema,
  parseDocument,
  type DashboardDocument,
} from './contract'
export type { LensThemeMode } from './runtime'

export { createReportDraft, copyReportDefinition, duplicateReportPlacement, parseReportDefinition, parseReportComposition } from './reports/draft'
export type { ReportDefinition, ReportComposition, ReportPlacement, ReportSourceDescriptor, ReportDraftHost, SaveIntent } from './reports/draft'

export { createFilterContextDraft, encodeFilterBoard, decodeFilterBoard, filterBoardURL, filterContextParam } from './reports/filterDraft'
export type { ContextBoard, ContextFilters, ContextPeriod, ContextInput, EffectiveContext, ContextEditor } from './contract/types'
export type { FilterDefaultsHost } from './reports/filterHost'
export { placeOpenedReport, placeLibraryReport } from './reports/placement'
