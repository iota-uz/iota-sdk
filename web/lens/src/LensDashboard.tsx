
import fixture from '../fixtures/panels-v1.json'
import { parseDocument, type DashboardDocument } from './contract'
import { DashboardPanels } from './DashboardPanels'
import type { CalendarDate } from './controls'
import { DashboardRuntimeProvider, DocumentProvider, type LensThemeMode } from './runtime'

export interface LensDashboardProps {
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
        <DocumentProvider src={props.src} initialDocument={document()} csrf={props.csrf} fetcher={props.fetcher}>
          <DashboardRuntimeProvider locale={locale()} csrf={props.csrf} fetcher={props.fetcher} fallback={fallback()}>
            {() => <DashboardPanels filterToday={props.filterToday} />}
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>
    </div>
  )
}
