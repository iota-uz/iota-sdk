import { createContext, useContext, type JSX } from 'solid-js'
import type { LayoutGroup } from '../contract/types'
import type { ReportVersionStatus } from './version'

/**
 * The host knows which revision each placement was opened with and which is
 * latest (report.Resolved.Version or compareReportVersion); the SDK only
 * renders that state. Placement ids are the report group ids on the wire.
 */
export interface ReportPlacementHost {
  /** Read reactively: return a signal-backed value to update an open notice. */
  status: (placementId: string) => ReportVersionStatus | undefined
  /** Adopt the newer definition; the document is refreshed after it resolves. */
  update?: (placementId: string) => Promise<void> | void
  /**
   * Record that the reader keeps the opened version. status should stop
   * returning update-available afterwards; the local dismissal is immediate
   * feedback only and does not survive a document refresh.
   */
  keep?: (placementId: string) => void
}

const ReportPlacementContext = createContext<ReportPlacementHost>()

export function ReportPlacementProvider(props: { host?: ReportPlacementHost; children: JSX.Element }) {
  return <ReportPlacementContext.Provider value={props.host}>{props.children}</ReportPlacementContext.Provider>
}

export function useReportPlacementHost() { return useContext(ReportPlacementContext) }

/**
 * Whether a placement's figures may be shown. The wire marks a placement the
 * server could not resolve on (re)open; the host marks one that became
 * incompatible while the document was open. Either way it is unavailable
 * rather than silently stale.
 */
export function reportPlacementAvailable(host: ReportPlacementHost | undefined, group: Pick<LayoutGroup, 'id' | 'unavailable'>): boolean {
  return !group.unavailable && host?.status(group.id) !== 'incompatible'
}
