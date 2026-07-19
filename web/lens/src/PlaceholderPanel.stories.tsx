import type { Story } from '@ladle/react'
import { useMemo, useRef } from 'react'
import fixture from '../fixtures/small.json'
import { parseDocument } from './contract'
import { PlaceholderPanel } from './PlaceholderPanel'
import { DashboardRuntimeProvider, DocumentProvider, useDashboard, useDrill, usePanelFrame } from './runtime'
import './styles.css'

const fixtureDocument = parseDocument(fixture)

export const PlaceholderStat: Story = () => (
  <div className="lens-root">
    <DocumentProvider initialDocument={fixtureDocument}>
      <DashboardRuntimeProvider locale="en">
        <PlaceholderPanel />
      </DashboardRuntimeProvider>
    </DocumentProvider>
  </div>
)

PlaceholderStat.storyName = 'Placeholder stat panel'

const pendingDocument = parseDocument({
  ...fixture,
  snapshotId: 'pending-model',
  panels: [{ ...fixture.panels[0], drillRoot: 'root' }],
  drill: {
    inlineDepth: 0,
    edges: {
      root: {
        path: ['root'],
        label: 'Total',
        children: [],
        perspectives: [],
      },
    },
  },
})

function PendingControls() {
  const { navigation } = useDashboard()
  const drill = useDrill()
  const frame = usePanelFrame('total')
  return (
    <div className="lens-mb-4 lens-flex lens-items-center lens-gap-3">
      <button
        type="button"
        className="lens-rounded-control lens-bg-accent-600 lens-px-3 lens-py-2 lens-text-sm lens-font-semibold lens-text-white"
        onClick={() => navigation.path.length === 0 ? drill.drillInto('root', 'total') : frame.retry()}
      >
        Refresh
      </button>
      <span className="lens-text-sm lens-text-muted">
        {frame.isLoading ? 'Refreshing with stale data' : frame.error ? 'Refresh failed — use Retry in the panel' : 'Ready'}
      </span>
    </div>
  )
}

function PendingModelDemo() {
  const requests = useRef(0)
  const fetcher = useMemo<typeof fetch>(() => async () => {
    requests.current += 1
    await new Promise((resolve) => window.setTimeout(resolve, 4000))
    if (requests.current === 2) {
      return new Response(JSON.stringify({ error: 'internal', message: 'The demo refresh failed' }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' },
      })
    }
    return new Response(JSON.stringify({
      frames: {
        'explore:pending': {
          columns: pendingDocument.frames['panel:total']?.columns ?? [],
          rows: [['Total', requests.current === 1 ? 43 : 44]],
        },
      },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }, [])

  return (
    <div className="lens-root">
      <DocumentProvider initialDocument={pendingDocument} fetcher={fetcher}>
        <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
          <PendingControls />
          <PlaceholderPanel />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const PendingModel: Story = () => <PendingModelDemo />
PendingModel.storyName = 'Pending model: stale, error, retry'
