import type { Story } from '@ladle/react'
import type { DashboardDocument, Frame, GeoJSONFeatureCollection, Panel } from './contract'
import { DashboardPanels } from './DashboardPanels'
import { MapPanel } from './panels/MapPanel'
import type { PanelRegistry } from './panels/registry'
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

function mapPanel(id: string, title: string, deferred = false, mapGeometry = geometry): Panel {
  return {
    id, kind: 'map', title, semantics: 'series', frame: `${id}:frame`, deferred,
    encoding: { id: 'code', label: 'name', value: 'policies' },
    format: { policies: { kind: 'number', minorUnits: false, precision: 0 } },
    presentation: { colorBy: 'rank' },
    map: {
      source: { inline: mapGeometry }, featureProperty: 'code', labelProperty: 'name',
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

function StateMapPanel({ panel }: { panel: Panel }) {
  return panel.id === 'map-loading'
    ? <MapPanel panel={panel} frame={{ isLoading: true, isStale: false, error: null, retry: () => undefined }} />
    : <MapPanel panel={panel} />
}

const registry: PanelRegistry = { map: StateMapPanel }

export const StateMatrix: Story = () => (
  <div className="lens-root">
    <DocumentProvider initialDocument={dashboardDocument} fetcher={fetcher}>
      <DashboardRuntimeProvider locale="en" fetcher={fetcher}><DashboardPanels registry={registry} /></DashboardRuntimeProvider>
    </DocumentProvider>
  </div>
)
StateMatrix.storyName = 'State matrix'

const denseRegionSpecs = [
  ['karakalpakstan', 'Республика Каракалпакстан', 0, 2.5, 4, 5],
  ['khorezm', 'Хорезмская область', 1, 2, 3, 2.8],
  ['navoi', 'Навоийская область', 3, 2.2, 6, 5],
  ['bukhara', 'Бухарская область', 3, 1.1, 5, 2.2],
  ['kashkadarya', 'Кашкадарьинская область', 4, 0, 6, 1.1],
  ['surkhandarya', 'Сурхандарьинская область', 5, -1, 6.5, 0],
  ['samarkand', 'Самаркандская область', 5, 1.1, 6.5, 2.2],
  ['jizzakh', 'Джизакская область', 6.2, 1.4, 7.1, 2.3],
  ['syrdarya', 'Сырдарьинская область', 7, 1.6, 7.8, 2.3],
  ['tashkent-region', 'Ташкентская область', 7.7, 1.8, 9, 2.7],
  ['tashkent-city', 'город Ташкент', 8, 1.9, 8.2, 2.1],
  ['namangan', 'Наманганская область', 8.7, 2.1, 9.7, 2.8],
  ['fergana', 'Ферганская область', 8.9, 1.5, 10, 2.1],
  ['andijan', 'Андижанская область', 9.5, 2, 10.3, 2.6],
] as const

const denseGeometry: GeoJSONFeatureCollection = {
  type: 'FeatureCollection',
  features: denseRegionSpecs.map(([code, name, x0, y0, x1, y1]) => ({
    type: 'Feature',
    properties: { code, name },
    geometry: { type: 'Polygon', coordinates: [[[x0, y0], [x1, y0], [x1, y1], [x0, y1], [x0, y0]]] },
  })),
}

const densePanel = mapPanel('dense-map', 'Регион', false, denseGeometry)
const denseDocument: DashboardDocument = {
  ...dashboardDocument,
  snapshotId: 'synthetic-dense-map',
  meta: { ...dashboardDocument.meta, dashboardId: 'synthetic-dense-map', title: 'Dense regional map', locale: 'ru' },
  header: undefined,
  layout: { rows: [{ panels: [{ panelId: densePanel.id, span: 12 }] }] },
  panels: [densePanel],
  frames: {
    'dense-map:frame': {
      columns: regions.columns,
      rows: denseRegionSpecs.map(([code, name], index) => [code, name, index + 1]),
    },
  },
}

export const DenseRegionLabels: Story = () => (
  <div className="lens-root" style={{ width: 500 }}>
    <DocumentProvider initialDocument={denseDocument} fetcher={fetcher}>
      <DashboardRuntimeProvider locale="ru" fetcher={fetcher}><DashboardPanels registry={registry} /></DashboardRuntimeProvider>
    </DocumentProvider>
  </div>
)
DenseRegionLabels.storyName = 'Dense region labels'
