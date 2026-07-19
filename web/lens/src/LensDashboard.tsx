import { useEffect, useState } from 'react'
import fixture from '../fixtures/small.json'
import type { LensDocument } from './document'
import { PlaceholderPanel } from './PlaceholderPanel'

export interface LensDashboardProps {
  src?: string
  locale?: string
  theme?: 'light' | 'dark'
  csrf?: string
}

type LoadState =
  | { status: 'ready'; document: LensDocument }
  | { status: 'loading' }
  | { status: 'error'; message: string }

const bundledFixture = fixture as LensDocument

export function LensDashboard({ src, locale = 'en', theme = 'light', csrf }: LensDashboardProps) {
  const [state, setState] = useState<LoadState>(() => ({
    status: 'ready',
    document: bundledFixture,
  }))

  useEffect(() => {
    if (!src) {
      setState({ status: 'ready', document: bundledFixture })
      return
    }

    const controller = new AbortController()
    setState({ status: 'loading' })

    void fetch(src, {
      credentials: 'same-origin',
      headers: csrf ? { 'X-CSRF-Token': csrf } : undefined,
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) {
          throw new Error(`request failed with ${response.status}`)
        }
        return (await response.json()) as LensDocument
      })
      .then((document) => setState({ status: 'ready', document }))
      .catch((error: unknown) => {
        if (controller.signal.aborted) return
        setState({ status: 'error', message: error instanceof Error ? error.message : 'request failed' })
      })

    return () => controller.abort()
  }, [csrf, src])

  return (
    <div className="lens-root" data-theme={theme} lang={locale}>
      {state.status === 'loading' && <div className="lens-placeholder-state">Loading dashboard…</div>}
      {state.status === 'error' && (
        <div className="lens-placeholder-state" role="alert">
          Unable to load Lens document: {state.message}
        </div>
      )}
      {state.status === 'ready' && <PlaceholderPanel document={state.document} locale={locale} src={src} />}
    </div>
  )
}
