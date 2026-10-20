
import type { JSX } from 'solid-js'
import { FilterDefaultsProvider, type FilterDefaultsHost } from './reports/filterHost'
import { ReportPlacementProvider, type ReportPlacementHost } from './reports/placementHost'
import fixture from '../fixtures/panels-v1.json'
import { parseDocument, type DashboardDocument } from './contract'
import { DashboardPanels } from './DashboardPanels'
import { HostRenderersProvider, type HostPanelRenderers } from './panels/hostRenderers'
import type { PanelRegistry } from './panels/registry'
import type { CalendarDate } from './controls'
import { DashboardRuntimeProvider, DocumentProvider, type LensThemeMode } from './runtime'

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
  theme?: LensThemeMode
  csrf?: string
  fetcher?: typeof fetch
  initialDocument?: DashboardDocument
  /** Fixed calendar "today" for deterministic stories and visual regression. */
  filterToday?: CalendarDate
}

export interface LensToolbarContext {
  document: DashboardDocument
}

const bundledFixture = parseDocument(fixture)

export function LensDashboard(props: LensDashboardProps) {
  const locale = () => props.locale ?? 'en'
  const theme = () => props.theme ?? 'light'
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
              <DocumentProvider src={props.src} initialDocument={document()} csrf={props.csrf} fetcher={props.fetcher}>
                <DashboardRuntimeProvider locale={locale()} csrf={props.csrf} fetcher={props.fetcher} fallback={fallback()}>
                  {() => <DashboardPanels filterToday={props.filterToday} registry={props.registry} toolbar={props.toolbar} />}
                </DashboardRuntimeProvider>
              </DocumentProvider>
            </HostRenderersProvider>
          </ReportPlacementProvider>
        </FilterDefaultsProvider>
      </div>
    </div>
  )
}
