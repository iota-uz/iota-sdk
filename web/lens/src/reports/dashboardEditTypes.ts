import type { Accessor } from 'solid-js'
import type { DashboardDocument } from '../contract/types'
import type { Composition, Placement } from './contract/types'
import type { SaveIntent } from './draft'
import type { CompositionSessionHost } from './editing'
import type { ReportLeaveGuard } from './guard'

/**
 * A host-driven edit mode over the live dashboard. The host owns the draft
 * (createReportDraft, which persists through its own save), whether the board
 * is being edited, and what happens after a save or cancel; the SDK edits the
 * draft in place and renders the cards from it.
 */
export interface LensDashboardEditing extends Omit<CompositionSessionHost, 'theme'> {
  /** Whether the board is in edit mode. */
  editing: Accessor<boolean>
  setEditing: (editing: boolean) => void
  /** In-app navigation guard; one with browser-unload protection is created when omitted. */
  guard?: ReportLeaveGuard
  /**
   * The document of a placement the open document does not contain yet (added
   * in this session), so its card shows live figures before the save. The
   * catalog's preview of the picked report is used when omitted.
   */
  placementDocument?: (placement: Placement, options: { signal: AbortSignal }) => Promise<DashboardDocument>
  /** After the host acknowledged a save; edit mode has ended. A host typically refreshes the document here. */
  onSaved?: (composition: Composition, intent: SaveIntent) => void
  /** After the reader cancelled; the draft is back at its saved state and edit mode has ended. */
  onCancel?: () => void
  /** Show the built-in Edit action (true by default); false when the host renders its own toggle. */
  toggle?: boolean
}

/** Which parts of the dashboard header the SDK renders. Every member defaults to the full chrome. */
export interface LensDashboardChrome {
  /** The document title. Hide it when the host page renders its own heading. */
  title?: boolean
  /** The document's own description line after the filters. */
  description?: boolean
  /** The whole header (title, actions and filters). An edit mode keeps its action row. */
  header?: boolean
  /** compact folds report periods, restoring saved filters, filter defaults and comparison into a "More" menu. */
  toolbar?: 'full' | 'compact'
  /**
   * How a composed report's placements read. `cards` (default) frames each placement with its
   * own title, filters, menu and calculation time; `flat` renders only the panels, so the
   * dashboard header carries the one shared set of filters and actions. A document with a single
   * placement (one report opened on its own) keeps its card, whose menu is the report's actions.
   */
  placements?: 'cards' | 'flat'
}
