
import { createEffect, createSignal, onCleanup, type JSX } from 'solid-js'
import { FilterDefaultsProvider, type FilterDefaultsHost } from './reports/filterHost'
import { ReportPlacementProvider, type ReportPlacementHost } from './reports/placementHost'
import fixture from '../fixtures/panels-v1.json'
import { parseDocument, type DashboardDocument } from './contract'
import { DashboardPanels } from './DashboardPanels'
import { HostRenderersProvider, type HostPanelRenderers } from './panels/hostRenderers'
import type { PanelRegistry } from './panels/registry'
import type { CalendarDate } from './controls'
import { DashboardRuntimeProvider, DocumentProvider, detectAppTheme, observeAppTheme, type LensThemeSetting } from './runtime'
import type { LensDashboardChrome, LensDashboardEditing } from './reports/dashboardEditTypes'
import { DrillHostProvider } from './explore/drillHost'
import type { InvestigationHost } from './explore/investigation'

export interface LensDashboardProps {
  filterDefaultsHost?: FilterDefaultsHost
  /**
   * Reusable report placements, keyed by placement id: version state,
   * availability, source capabilities and the card menu's handlers.
   */
  reportPlacementHost?: ReportPlacementHost
  /**
   * Host renderers for panels whose document names one (panel.renderer). The
   * SDK frame keeps title, states, retry, export and the placement menu.
   */
  renderers?: HostPanelRenderers
  /** Replaces the built-in component of a panel kind. */
  registry?: PanelRegistry
  toolbar?: (context: LensToolbarContext) => JSX.Element
  src?: string
  /**
   * Markup the server already rendered inside the mount point (the templ
   * skeleton). The mount clears the container on boot, so the runtime re-inserts
   * it while the first document is in flight.
   */
  fallbackHTML?: string
  locale?: string
  /**
   * `light` or `dark` fixes the board's theme. `auto` follows the host
   * application: the `dark` class on `<html>`, or the system colour scheme when
   * the page declares neither, and switches without a reload.
   */
  theme?: LensThemeSetting
  csrf?: string
  fetcher?: typeof fetch
  initialDocument?: DashboardDocument
  /** Fixed calendar "today" for deterministic stories and visual regression. */
  filterToday?: CalendarDate
  /**
   * In-place editing of a composed dashboard. The cards are drawn from the
   * draft, so a layout change re-renders the live cards immediately; the
   * host persists through the draft and decides when editing starts.
   */
  edit?: LensDashboardEditing
  /** Hide parts of the header a host page already renders, or collapse the toolbar. */
  chrome?: LensDashboardChrome
  /**
   * Saving an exploration as a report. Offered on the current drill level of
   * a report placement whose source can save the slice; explained otherwise.
   * The host persists the report and adds it to dashboard drafts.
   */
  investigation?: InvestigationHost
  /**
   * Adds the `lens-return` context to links that leave Lens for host pages
   * (operational records), so the host can render a Back link that restores
   * the levels, filters and origin. See readReturnContext and backURL.
   */
  returnContext?: boolean
}

export interface LensToolbarContext {
  document: DashboardDocument
}

const bundledFixture = parseDocument(fixture)

export function LensDashboard(props: LensDashboardProps) {
  const locale = () => props.locale ?? 'en'
  const [appTheme, setAppTheme] = createSignal(detectAppTheme())
  createEffect(() => {
    if (props.theme !== 'auto') return
    setAppTheme(detectAppTheme())
    onCleanup(observeAppTheme(setAppTheme))
  })
  const theme = () => (props.theme === 'auto' ? appTheme() : props.theme ?? 'light')
  const document = () => props.initialDocument ?? (props.src ? undefined : bundledFixture)
  // The fallback is this application's own server-rendered skeleton, echoed
  // back verbatim; it never carries request data.
  const fallback = () => props.fallbackHTML
    ? <div aria-hidden="true" innerHTML={props.fallbackHTML} />
    : undefined
  return (
    <div class="lens-root" data-theme={theme()} lang={locale()}>
      {/* The React client-host boundary is not required for the Solid mount; the
          theme still travels on this wrapper so portalled overlays inherit it. */}
      <div data-theme={theme()}>
        <FilterDefaultsProvider host={props.filterDefaultsHost}>
          <ReportPlacementProvider host={props.reportPlacementHost}>
            <HostRenderersProvider renderers={props.renderers}>
              <DrillHostProvider host={{ investigation: props.investigation, returnContext: props.returnContext }}>
                <DocumentProvider src={props.src} initialDocument={document()} csrf={props.csrf} fetcher={props.fetcher}>
                  <DashboardRuntimeProvider locale={locale()} csrf={props.csrf} fetcher={props.fetcher} fallback={fallback()}>
                    {() => (
                      <DashboardPanels
                        chrome={props.chrome}
                        csrf={props.csrf}
                        edit={props.edit}
                        fetcher={props.fetcher}
                        filterToday={props.filterToday}
                        locale={props.locale}
                        registry={props.registry}
                        theme={theme()}
                        toolbar={props.toolbar}
                      />
                    )}
                  </DashboardRuntimeProvider>
                </DocumentProvider>
              </DrillHostProvider>
            </HostRenderersProvider>
          </ReportPlacementProvider>
        </FilterDefaultsProvider>
      </div>
    </div>
  )
}
