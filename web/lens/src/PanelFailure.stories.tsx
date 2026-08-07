import type { Story } from '@ladle/react'
import { useEffect } from 'react'
import type { DashboardDocument, FailureReason, Panel } from './contract'
import { StatPanel } from './panels'
import { DashboardRuntimeProvider, DocumentProvider, useDrill } from './runtime'
import './styles.css'

/**
 * What a panel says when it could not answer.
 *
 * The panel matrix covers the generic failure; this covers the sentences a
 * classified failure earns instead. They are the only user-visible difference
 * between a slice too large to finish, a load the reader themselves abandoned,
 * and a fault nobody can name, so each one is worth a picture.
 */

const cases: Array<{ label: string; reason?: FailureReason }> = [
  { label: 'timeout', reason: 'timeout' },
  { label: 'canceled', reason: 'canceled' },
  { label: 'unclassified' },
]

function failurePanel(id: string): Panel {
  return {
    id,
    kind: 'stat',
    title: 'Earned premium',
    semantics: 'series',
    frame: `${id}-frame`,
    encoding: { id: 'id', value: 'value' },
    format: {
      value: { kind: 'money', currency: 'USD', minorUnits: true, precision: 0 },
      delta: { kind: 'percent', minorUnits: false, precision: 1 },
    },
    drillRoot: 'root',
    actions: [],
  } as Panel
}

function failureDocument(id: string): DashboardDocument {
  return {
    version: '1.0.0',
    snapshotId: id,
    meta: { dashboardId: 'panel-failure', title: 'Panel failure', generatedAt: '2026-07-19T00:00:00Z', locale: 'en' },
    layout: { rows: [{ panels: [{ panelId: id, span: 12 }] }] },
    panels: [failurePanel(id)],
    frames: {},
    drill: {
      inlineDepth: 0,
      edges: {
        root: {
          path: ['root'],
          label: 'All regions',
          children: [{ key: 'root/north', path: ['root', 'root/north'], label: 'North', target: 'north' }],
          perspectives: [],
        },
        north: { path: ['root', 'root/north'], label: 'North', children: [], perspectives: [] },
      },
    },
    perspectives: [],
    endpoints: { query: '/story/query' },
    i18n: {},
    theme: { palette: { accent: '#2563eb', muted: '#94a3b8' }, series: {} },
  }
}

function TriggerQuery({ panelId }: { panelId: string }) {
  const { drillInto } = useDrill()
  useEffect(() => {
    drillInto('root/north', panelId)
  }, [drillInto, panelId])
  return null
}

function FailureCell({ label, reason }: { label: string; reason?: FailureReason }) {
  const id = `failure-${label}`
  const document = failureDocument(id)
  // The classification is the server's, so the story fails the request the way
  // the server does: the same JSON body, reason included or deliberately not.
  const fetcher: typeof fetch = () => Promise.resolve(new Response(
    JSON.stringify({ error: 'internal', message: 'panel execution failed', ...(reason ? { reason } : {}) }),
    { status: 500, headers: { 'Content-Type': 'application/json' } },
  ))
  return (
    <div className="lens-story-cell">
      <span className="lens-story-cell-label">{label}</span>
      <DocumentProvider initialDocument={document} fetcher={fetcher}>
        <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
          <TriggerQuery panelId={id} />
          <StatPanel panel={document.panels[0]!} />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

function FailureMatrix({ theme }: { theme: 'light' | 'dark' }) {
  return (
    <div className="lens-root lens-story-matrix" data-theme={theme}>
      {/* Three equal columns, declared here rather than in the stylesheet: the
          matrix grid next door is shaped for a label column plus one column per
          panel state, and this story has neither. */}
      <div style={{ display: 'grid', gap: '16px', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))' }}>
        {cases.map(({ label, reason }) => <FailureCell key={label} label={label} reason={reason} />)}
      </div>
    </div>
  )
}

export const Light: Story = () => <FailureMatrix theme="light" />
Light.storyName = 'Failure reasons - light'

export const Dark: Story = () => <FailureMatrix theme="dark" />
Dark.storyName = 'Failure reasons - dark'
