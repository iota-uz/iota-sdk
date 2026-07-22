import fixture from '../fixtures/panels-v1.json'
import { parseDocument, type DashboardDocument } from './contract'
import { DashboardPanels } from './DashboardPanels'
import type { CalendarDate } from './controls'
import { DashboardRuntimeProvider, DocumentProvider, type LensThemeMode } from './runtime'

export interface LensDashboardProps {
  src?: string
  /**
   * Markup the server already rendered inside the mount point (the templ
   * skeleton). React clears the container on mount, so the runtime re-inserts
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

export function LensDashboard({
  src, locale = 'en', theme = 'light', csrf, fetcher, fallbackHTML, initialDocument = bundledFixture, filterToday,
}: LensDashboardProps) {
  // The fallback is this application's own server-rendered skeleton, echoed
  // back verbatim; it never carries request data.
  const fallback = fallbackHTML
    ? <div aria-hidden="true" dangerouslySetInnerHTML={{ __html: fallbackHTML }} />
    : undefined
  return (
    <div className="lens-root" data-theme={theme} lang={locale}>
      <DocumentProvider src={src} initialDocument={initialDocument} csrf={csrf} fetcher={fetcher}>
        <DashboardRuntimeProvider locale={locale} csrf={csrf} fetcher={fetcher} fallback={fallback}>
          <DashboardPanels filterToday={filterToday} />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}
