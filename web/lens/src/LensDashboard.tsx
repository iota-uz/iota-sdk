/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import fixture from '../fixtures/panels-v1.json'
import { parseDocument, type DashboardDocument } from './contract'
import { DashboardPanels } from './DashboardPanels'
import type { CalendarDate } from './controls'
import { DashboardRuntimeProvider, DocumentProvider, type LensThemeMode } from './runtime'
import type { JSX } from 'solid-js'

// The runtime providers keep their React signatures during the migration; these
// typed bridges let the mount compile against the pending Solid runtime.
const DocumentProviderBridge = DocumentProvider as unknown as (props: {
  src?: string
  initialDocument?: DashboardDocument
  csrf?: string
  fetcher?: typeof fetch
  children: JSX.Element
}) => JSX.Element
const RuntimeProviderBridge = DashboardRuntimeProvider as unknown as (props: {
  locale: string
  csrf?: string
  fetcher?: typeof fetch
  fallback?: JSX.Element
  children: JSX.Element
}) => JSX.Element

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
        <DocumentProviderBridge src={props.src} initialDocument={document()} csrf={props.csrf} fetcher={props.fetcher}>
          <RuntimeProviderBridge locale={locale()} csrf={props.csrf} fetcher={props.fetcher} fallback={fallback()}>
            <DashboardPanels filterToday={props.filterToday} />
          </RuntimeProviderBridge>
        </DocumentProviderBridge>
      </div>
    </div>
  )
}
