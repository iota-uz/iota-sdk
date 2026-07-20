import type { Story } from '@ladle/react'
import { useEffect, useRef } from 'react'
import type { DashboardDocument, Frame, Panel } from './contract'
import { CascadePanel, ExportButton, TablePanel } from './panels'
import { DashboardRuntimeProvider, DocumentProvider, useDrill, useExport, usePanelFrame, usePanelPagination } from './runtime'
import './styles.css'

const cascadePanel: Panel = {
  id: 'margin-bridge', kind: 'cascade', title: 'Margin bridge', semantics: 'reconciliation', frame: 'bridge',
  total: 1840000,
  encoding: { label: 'stage', value: 'balance', cut: 'movement', cutLabel: 'movementLabel', final: 'reconciled' },
  format: {
    balance: { kind: 'money', currency: 'USD', minorUnits: false, precision: 0 },
    movement: { kind: 'money', currency: 'USD', minorUnits: false, precision: 0 },
  },
  actions: [],
}

const cascadeFrame: Frame = {
  columns: [
    { name: 'stage', type: 'string' }, { name: 'balance', type: 'number' },
    { name: 'movement', type: 'number' }, { name: 'movementLabel', type: 'string' },
    { name: 'reconciled', type: 'bool' },
  ],
  rows: [
    ['Gross margin', 3120000, 0, '', false],
    ['After claims', 2310000, 810000, 'Claims paid', false],
    ['Operating margin', 1840000, 470000, 'Operating expenses', true],
  ],
}

const tablePanel: Panel = {
  id: 'evidence', kind: 'table', title: 'Policy evidence', semantics: 'evidence', frame: 'evidence',
  encoding: { id: 'policyId', label: 'policyholder', value: 'premium' },
  format: {
    premium: { kind: 'money', currency: 'USD', minorUnits: false, precision: 0 },
    effectiveAt: { kind: 'date', minorUnits: false, layout: '2006-01-02' },
  },
  drillRoot: 'evidence',
  actions: [{
    kind: 'navigate_to_leaf', urlTemplate: '/policies/{policyId}',
    params: [{ name: 'policyId', source: { kind: 'field', name: 'policyId' } }], payload: {}, preserveQuery: true,
  }],
}

const columnsPanel: Panel = {
  id: 'profitability', kind: 'table', title: 'Profitability by client', semantics: 'evidence', frame: 'profitability',
  encoding: { id: 'clientId', label: 'client' },
  format: {
    earned: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0 },
    growth: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0 },
    growthPct: { kind: 'percent', minorUnits: false, precision: 1 },
  },
  columns: [
    {
      field: 'client', label: 'Client', cell: { kind: 'plain' },
      action: { kind: 'navigate_to_leaf', urlSource: { kind: 'field', name: 'detailUrl' }, params: [], payload: {} },
    },
    { field: 'earned', label: 'Earned premium', align: 'right', cell: { kind: 'bar' } },
    { field: 'growth', label: 'YoY growth', align: 'right', cell: { kind: 'delta', secondaryField: 'growthPct' } },
  ],
  actions: [],
}

const columnsFrame: Frame = {
  columns: [
    { name: 'clientId', type: 'string' }, { name: 'client', type: 'string' },
    { name: 'earned', type: 'number' }, { name: 'growth', type: 'number' },
    { name: 'growthPct', type: 'number' }, { name: 'detailUrl', type: 'string' },
    { name: 'internalNote', type: 'string' },
  ],
  rows: [
    ['1', 'Orion Services', 4_820_000_000, 610_000_000, 14.5, '/clients/1', 'hidden'],
    ['2', 'Northstar Supply', 3_140_000_000, -220_000_000, -6.7, '/clients/2', 'hidden'],
    ['3', 'Meridian Works', 1_760_000_000, 90_000_000, 5.1, '/clients/3', 'hidden'],
  ],
}

function storyDocument(panel: Panel, frames: Record<string, Frame>, endpoints: DashboardDocument['endpoints'] = {}): DashboardDocument {
  return {
    version: '1.0.0', snapshotId: 'story-snapshot',
    meta: { dashboardId: 'panels-v2', title: 'Panels v2', generatedAt: '2026-07-19T00:00:00Z', locale: 'en' },
    layout: { rows: [{ panels: [{ panelId: panel.id, span: 12 }] }] }, panels: [panel], frames,
    drill: {
      inlineDepth: 0,
      edges: panel.drillRoot ? {
        evidence: { path: ['evidence'], label: 'Policy evidence', children: [], frame: 'evidence', encoding: panel.encoding, perspectives: [] },
      } : {},
    },
    perspectives: [], endpoints, i18n: {}, theme: { palette: {}, series: {} },
  }
}

function Runtime({ document, fetcher, children }: { document: DashboardDocument; fetcher?: typeof fetch; children: React.ReactNode }) {
  return (
    <div className="lens-root">
      <DocumentProvider initialDocument={document}>
        <DashboardRuntimeProvider locale="en" fetcher={fetcher}>{children}</DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const CascadeFinalStage: Story = () => {
  const document = storyDocument(cascadePanel, { bridge: cascadeFrame })
  return <Runtime document={document}><CascadePanel panel={cascadePanel} /></Runtime>
}

function OpenEvidence({ emptyPage }: { emptyPage?: boolean }) {
  const drill = useDrill()
  const pagination = usePanelPagination()
  const frame = usePanelFrame(tablePanel.id)
  const opened = useRef(false)
  useEffect(() => { drill.drillInto('evidence', tablePanel.id) }, [drill])
  useEffect(() => {
    if (emptyPage && frame.page?.number === 1 && !opened.current) {
      opened.current = true
      void pagination.loadPage(tablePanel.id, 2)
    }
  }, [emptyPage, frame.page?.number, pagination])
  return <TablePanel panel={tablePanel} />
}

function tableResponse(page: number, emptyPage = false): Response {
  const rows = page === 2 && emptyPage ? [] : page === 1 ? [
    ['PL-1042', 'Orion Services', 284000, '2026-07-01T00:00:00Z', true],
    ['PL-1098', 'Northstar Supply', 197000, '2026-07-08T00:00:00Z', false],
  ] : [['PL-1131', 'Meridian Works', 163000, '2026-07-12T00:00:00Z', true]]
  return new Response(JSON.stringify({
    frames: { evidence: { columns: [
      { name: 'policyId', type: 'string' }, { name: 'policyholder', type: 'string' },
      { name: 'premium', type: 'number' }, { name: 'effectiveAt', type: 'time' }, { name: 'active', type: 'bool' },
    ], rows } },
    page: { number: page, size: 2 },
  }), { headers: { 'Content-Type': 'application/json' } })
}

function TableStory({ emptyPage = false }: { emptyPage?: boolean }) {
  const document = storyDocument(tablePanel, {}, { query: '/story/query' })
  const fetcher: typeof fetch = (_input, init) => {
    const request = JSON.parse(typeof init?.body === 'string' ? init.body : '{}') as { page: number }
    return Promise.resolve(tableResponse(request.page, emptyPage))
  }
  return <Runtime document={document} fetcher={fetcher}><OpenEvidence emptyPage={emptyPage} /></Runtime>
}

export const TablePaginationAndLeafActions: Story = () => <TableStory />
export const TableEmptyPage: Story = () => <TableStory emptyPage />

export const TableColumns: Story = () => {
  const document = storyDocument(columnsPanel, { profitability: columnsFrame })
  return <Runtime document={document}><TablePanel panel={columnsPanel} /></Runtime>
}

function AutoExport() {
  const action = useExport('export-story')
  const started = useRef(false)
  useEffect(() => {
    if (!started.current) {
      started.current = true
      void action.run()
    }
  }, [action])
  return <ExportButton panelId="export-story" />
}

function ExportStory({ mode }: { mode: 'idle' | 'pending' | 'retry' }) {
  const panel = { ...cascadePanel, id: 'export-story', frame: 'export-frame' }
  const document = storyDocument(panel, { 'export-frame': cascadeFrame }, { export: '/story/export' })
  const fetcher: typeof fetch = () => mode === 'retry'
    ? Promise.resolve(new Response(JSON.stringify({ error: 'snapshot_gone', message: 'snapshot expired' }), {
        status: 410, headers: { 'Content-Type': 'application/json' },
      }))
    : new Promise<Response>(() => undefined)
  return <Runtime document={document} fetcher={fetcher}>{mode === 'idle' ? <ExportButton panelId={panel.id} /> : <AutoExport />}</Runtime>
}

export const ExportIdle: Story = () => <ExportStory mode="idle" />
export const ExportPending: Story = () => <ExportStory mode="pending" />
export const ExportSnapshotRetry: Story = () => <ExportStory mode="retry" />
