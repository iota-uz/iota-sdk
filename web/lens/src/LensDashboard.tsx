import fixture from '../fixtures/small.json'
import { parseDocument, type DashboardDocument } from './contract'
import { PlaceholderPanel } from './PlaceholderPanel'
import { DashboardRuntimeProvider, DocumentProvider } from './runtime'

export interface LensDashboardProps {
  src?: string
  locale?: string
  theme?: 'light' | 'dark'
  csrf?: string
  fetcher?: typeof fetch
  initialDocument?: DashboardDocument
}

const bundledFixture = parseDocument(fixture)

export function LensDashboard({ src, locale = 'en', theme = 'light', csrf, fetcher, initialDocument = bundledFixture }: LensDashboardProps) {
  return (
    <div className="lens-root" data-theme={theme} lang={locale}>
      <DocumentProvider src={src} initialDocument={initialDocument} csrf={csrf} fetcher={fetcher}>
        <DashboardRuntimeProvider locale={locale} csrf={csrf} fetcher={fetcher}>
          <PlaceholderPanel src={src} />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}
