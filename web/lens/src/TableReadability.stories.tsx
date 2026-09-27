import type { Story } from '@ladle/react'
import type { DashboardDocument, Frame, Panel } from './contract'
import { TablePanel } from './panels'
import { DashboardRuntimeProvider, DocumentProvider } from './runtime'
import './styles.css'

const panel: Panel = {
  id: 'claims-products', kind: 'table', title: 'Claims by product', semantics: 'evidence', frame: 'panel:claims-products', deferred: true,
  encoding: { id: 'id', label: 'product', value: 'paid' },
  format: {
    claims: { kind: 'number', minorUnits: false, precision: 0 },
    paid: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0 },
    average: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0 },
  },
  columns: [
    { field: 'product', label: 'Product', cell: { kind: 'plain' }, clamp: 2, widthPx: 280 },
    { field: 'claims', label: 'Claims', align: 'right', cell: { kind: 'plain' } },
    { field: 'paid', label: 'Paid', align: 'right', cell: { kind: 'bar' }, heat: true, total: true },
    { field: 'share', label: 'Share', align: 'right', cell: { kind: 'plain' }, shareOf: 'paid', total: true },
    { field: 'average', label: 'Average severity', align: 'right', cell: { kind: 'plain' }, sampleSizeField: 'claims', minSampleSize: 5 },
  ],
  table: { searchable: true },
  terminal: true,
  actions: [],
}

const data: Frame = {
  columns: [
    { name: 'id', type: 'string' }, { name: 'product', type: 'string' },
    { name: 'claims', type: 'number' }, { name: 'paid', type: 'number' }, { name: 'share', type: 'number' }, { name: 'average', type: 'number' },
  ],
  rows: [
    ['1', 'Compulsory motor third-party liability insurance for vehicle owners', 435, 4_036_800_000, 86.4, 9_280_000],
    ['2', 'Comprehensive motor insurance', 2, 58_000_000, 1.2, 29_000_000],
    ['3', 'Employer civil liability insurance', 12, 336_000_000, 7.2, 28_000_000],
    ['4', 'Property insurance against fire and natural hazards', 7, 91_000_000, 1.9, 13_000_000],
    ['5', 'Travel insurance', 1, 4_500_000, 0.1, 4_500_000],
    ['6', 'Cargo insurance', 9, 144_000_000, 3.1, 16_000_000],
  ],
}

const document_: DashboardDocument = {
  version: '1.0.0', snapshotId: 'table-readability',
  meta: { dashboardId: 'table-readability', title: 'Claims performance', generatedAt: '2026-07-31T00:00:00Z', locale: 'en' },
  layout: { rows: [{ panels: [{ panelId: panel.id, span: 12 }] }] },
  panels: [panel], frames: {},
  drill: { inlineDepth: 0, edges: {} }, perspectives: [], endpoints: { panel: '/story/panel', export: '/story/export' }, i18n: {}, theme: { palette: {}, series: {} },
}

const fetcher: typeof fetch = () => Promise.resolve(new Response(`${JSON.stringify({
  panelId: 'claims-products',
  result: {
    frames: { 'panel:claims-products': data },
    calculation: { durationMs: 42, cacheHit: true, calculatedAt: '2026-07-31T00:00:00Z' },
    summary: {
      filteredRows: 6,
      totalRows: 6,
      values: { paid: 4_670_300_000, share: 100 },
      fullValues: { paid: 4_670_300_000, share: 100 },
    },
  },
})}\n${JSON.stringify({ complete: true })}\n`, { status: 200, headers: { 'Content-Type': 'application/x-ndjson' } }))

function Scene({ narrow = false }: { narrow?: boolean }) {
  return (
    <div className="lens-root lens-story-shell" style={{ width: narrow ? 600 : 1000 }}>
      <DocumentProvider initialDocument={document_}>
        <DashboardRuntimeProvider fetcher={fetcher} locale="en">
          <TablePanel panel={panel} />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const Wide: Story = () => <Scene />
export const Narrow: Story = () => <Scene narrow />
