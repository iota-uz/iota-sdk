import type { Story } from '@ladle/react'
import type { DashboardDocument, Frame, GeoJSONFeatureCollection, Panel } from './contract'
import { DashboardPanels } from './DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider } from './runtime'
import './styles.css'

const geometry: GeoJSONFeatureCollection = {
  type: 'FeatureCollection',
  features: [
    { type: 'Feature', properties: { code: 'northwest', name: 'Northwest' }, geometry: { type: 'Polygon', coordinates: [[[0, 2], [2, 2], [2, 4], [0, 4], [0, 2]]] } },
    { type: 'Feature', properties: { code: 'northeast', name: 'Northeast' }, geometry: { type: 'Polygon', coordinates: [[[2, 2], [4, 2], [4, 4], [2, 4], [2, 2]]] } },
    { type: 'Feature', properties: { code: 'southwest', name: 'Southwest' }, geometry: { type: 'Polygon', coordinates: [[[0, 0], [2, 0], [2, 2], [0, 2], [0, 0]]] } },
    { type: 'Feature', properties: { code: 'southeast', name: 'Southeast' }, geometry: { type: 'Polygon', coordinates: [[[2, 0], [4, 0], [4, 2], [2, 2], [2, 0]]] } },
  ],
}

const regions: Frame = {
  columns: [{ name: 'code', type: 'string' }, { name: 'name', type: 'string' }, { name: 'policies', type: 'number' }],
  rows: [
    ['northwest', 'Northwest', 82], ['northeast', 'Northeast', 54],
    ['southwest', 'Southwest', 31], ['southeast', 'Southeast', 67],
  ],
}

function mapPanel(id: string, title: string, deferred = false): Panel {
  return {
    id, kind: 'map', title, semantics: 'series', frame: `${id}:frame`, deferred,
    encoding: { id: 'code', label: 'name', value: 'policies' },
    format: { policies: { kind: 'number', minorUnits: false, precision: 0 } },
    map: {
      source: { inline: geometry }, featureProperty: 'code', labelProperty: 'name',
      attribution: '© Example contributors · Synthetic boundaries (ODbL)',
    },
    terminal: true, actions: [],
  }
}

const panels = [
  mapPanel('map-data', 'Data'),
  mapPanel('map-empty', 'Empty'),
  mapPanel('map-loading', 'Loading', true),
  mapPanel('map-error', 'Error', true),
  mapPanel('map-stale', 'Stale', true),
]

const dashboardDocument: DashboardDocument = {
  version: '1.0.0', snapshotId: 'synthetic-map-states',
  meta: { dashboardId: 'synthetic-map', title: 'Choropleth states', generatedAt: '2026-07-31T12:00:00Z', locale: 'en' },
  header: { title: 'Synthetic choropleth', subtitle: 'Exact region-key joins · no external boundary data' },
  layout: {
    rows: [
      { panels: panels.slice(0, 3).map(({ id }) => ({ panelId: id, span: 4 })) },
      { panels: panels.slice(3).map(({ id }) => ({ panelId: id, span: 6 })) },
    ],
  },
  panels,
  frames: {
    'map-data:frame': regions,
    'map-empty:frame': { ...regions, rows: [] },
    'map-stale:frame': regions,
  },
  drill: { inlineDepth: 0, edges: {} }, perspectives: [],
  endpoints: { panel: '/story/map-panel' }, i18n: {},
  theme: { palette: { accent: '#2563eb', muted: '#dbeafe' }, series: {} },
}

const fetcher: typeof fetch = () => {
  const encoder = new TextEncoder()
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const panelId of ['map-error', 'map-stale']) {
        controller.enqueue(encoder.encode(`${JSON.stringify({
          panelId, result: { error: { error: 'unavailable', message: 'Regional data is unavailable' } },
        })}\n`))
      }
      // `map-loading` intentionally remains unfinished while its error and
      // stale siblings have already flushed their independent results.
    },
  })
  return Promise.resolve(new Response(stream, { status: 200, headers: { 'Content-Type': 'application/x-ndjson' } }))
}

export const StateMatrix: Story = () => (
  <div className="lens-root">
    <DocumentProvider initialDocument={dashboardDocument} fetcher={fetcher}>
      <DashboardRuntimeProvider locale="en" fetcher={fetcher}><DashboardPanels /></DashboardRuntimeProvider>
    </DocumentProvider>
  </div>
)
StateMatrix.storyName = 'State matrix'
