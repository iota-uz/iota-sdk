import { createContext, useContext, type JSX } from 'solid-js'
import type { EffectiveContext, GroupUnavailable, LayoutGroup } from '../contract/types'
import type { Capabilities } from './contract/types'
import { reportPlacementActions, type ReportOperation, type ReportVersionStatus } from './version'

/** What a reader can do with a placement. Each is a host operation; the SDK never stores anything. */
export type ReportPlacementAction = 'open' | 'edit' | 'configure' | 'copy' | 'remove'

/** The handlers a host supports. A missing handler hides its menu item. */
export type ReportPlacementActionHandlers = Partial<Record<ReportPlacementAction, (placementId: string) => Promise<void> | void>>

/**
 * The host knows which revision each placement was opened with and which is
 * latest (report.Resolved.Version or compareReportVersion); the SDK only
 * renders that state. Placement ids are the report group ids on the wire.
 * Every member is optional, so a host can adopt one capability at a time.
 */
export interface ReportPlacementHost {
  /** Read reactively: return a signal-backed value to update an open notice. */
  status?: (placementId: string) => ReportVersionStatus | undefined
  /** Adopt the newer definition; the document is refreshed after it resolves. */
  update?: (placementId: string) => Promise<void> | void
  /**
   * Record that the reader keeps the opened version. status should stop
   * returning update-available afterwards; the local dismissal is immediate
   * feedback only and does not survive a document refresh.
   */
  keep?: (placementId: string) => void
  /**
   * Why a placement cannot be shown, when the host learned it after the
   * document was composed. The wire's own group.unavailable takes precedence.
   */
  unavailable?: (placementId: string) => GroupUnavailable | undefined
  /** The placement's source capabilities; drill/save/export are offered only where they hold. */
  capabilities?: (placementId: string) => Pick<Capabilities, ReportOperation> | undefined
  /** Card menu handlers: open/edit the report, configure the card, copy it independently, remove it. */
  actions?: ReportPlacementActionHandlers
}

const ReportPlacementContext = createContext<ReportPlacementHost>()

export function ReportPlacementProvider(props: { host?: ReportPlacementHost; children: JSX.Element }) {
  return <ReportPlacementContext.Provider value={props.host}>{props.children}</ReportPlacementContext.Provider>
}

export function useReportPlacementHost() { return useContext(ReportPlacementContext) }

/**
 * Why a placement's figures are withheld, or undefined when they may be shown.
 * The wire marks a placement the server could not resolve on (re)open; the
 * host marks one that became incompatible, deleted or inaccessible while the
 * document was open. Either way it is unavailable rather than silently stale.
 */
export function reportPlacementUnavailable(host: ReportPlacementHost | undefined, group: Pick<LayoutGroup, 'id' | 'unavailable'>): GroupUnavailable | undefined {
  if (group.unavailable) return group.unavailable
  const marked = host?.unavailable?.(group.id)
  if (marked) return marked
  return host?.status?.(group.id) === 'incompatible' ? 'report-incompatible' : undefined
}

/** Whether a placement's figures may be shown. */
export function reportPlacementAvailable(host: ReportPlacementHost | undefined, group: Pick<LayoutGroup, 'id' | 'unavailable'>): boolean {
  return reportPlacementUnavailable(host, group) === undefined
}

const actionOrder: ReportPlacementAction[] = ['open', 'edit', 'configure', 'copy', 'remove']

/**
 * The card menu items a placement offers, in menu order. An item needs its
 * host handler; editing or copying the report also needs the source to allow
 * saving under the card's effective context. A deleted or forbidden report
 * leaves only removal: nothing else can act on a report that cannot be read.
 * An incompatible one can still be opened or edited by someone who can fix it.
 */
export function reportPlacementMenu(
  host: ReportPlacementHost | undefined,
  placementId: string,
  options: { context?: Pick<EffectiveContext, 'conflicts' | 'unsupported' | 'unsupportedOperations'>; unavailable?: GroupUnavailable } = {},
): ReportPlacementAction[] {
  const handlers = host?.actions
  if (!handlers) return []
  const capabilities = host.capabilities?.(placementId)
  const saving = capabilities ? reportPlacementActions(capabilities, options.unavailable ? undefined : options.context).save : false
  const allowed = (action: ReportPlacementAction): boolean => {
    if (!handlers[action]) return false
    if (options.unavailable === 'report-deleted' || options.unavailable === 'report-forbidden') return action === 'remove'
    if (options.unavailable === 'report-incompatible' && (action === 'configure' || action === 'copy')) return false
    if (action === 'edit' || action === 'copy') return saving
    return true
  }
  return actionOrder.filter(allowed)
}
