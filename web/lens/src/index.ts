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
export { LensMessagesProvider } from './runtime'

export { createReportDraft, copyReportDefinition, duplicateReportPlacement, parseReportDefinition, parseReportComposition } from './reports/draft'
export type { ReportDefinition, ReportComposition, ReportPlacement, ReportSourceDescriptor, ReportDraftHost, SaveIntent } from './reports/draft'

export { createFilterContextDraft, encodeFilterBoard, decodeFilterBoard, filterBoardURL, filterContextParam } from './reports/filterDraft'
export type { ContextBoard, ContextFilters, ContextPeriod, ContextInput, EffectiveContext, ContextEditor } from './contract/types'
export type { FilterDefaultsHost } from './reports/filterHost'
export { placeOpenedReport, placeLibraryReport } from './reports/placement'
export { compareReportVersion, reportPlacementActions, withoutUnavailablePlacements } from './reports/version'
export type { ReportVersionStatus, ReportRevision, ReportOperation, ReportActions } from './reports/version'
export { ReportVersionNotice } from './reports/ReportVersionNotice'
export { ReportPlacementProvider, reportPlacementAvailable, reportPlacementMenu, reportPlacementUnavailable, useReportPlacementHost } from './reports/placementHost'
export type { ReportPlacementAction, ReportPlacementActionHandlers, ReportPlacementHost } from './reports/placementHost'
export { placementFreshness, formatCalculatedAt } from './reports/ReportPlacementStatus'
export type { PlacementFreshness } from './reports/ReportPlacementStatus'
export { reportUnavailableCopy } from './reports/unavailable'
export type { HostPanelRenderer, HostPanelRendererProps, HostPanelRenderers } from './panels/hostRenderers'
export type { PanelRegistry, PanelComponent } from './panels/registry'
export type { PanelFrameState } from './runtime'
export { ReportPicker, type ReportPickerProps } from './reports/ReportPicker'
export { ReportDestinationPicker, type ReportDestinationPickerProps } from './reports/ReportDestinationPicker'
export { appendReportPicks, createReportPickerSelection, parseCatalogPage, parseDestinations } from './reports/picker'
export type {
  ReportCatalogEntry, ReportCatalogHost, ReportCatalogPage, ReportCatalogQuery, ReportCatalogScope, ReportDestination,
  ReportDestinationChoice, ReportDestinationConfirmation, ReportDestinationHost, ReportPick, ReportPickerConfirmation,
} from './reports/picker'
