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
  terminal: true,
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

// A profit bridge with explicit per-stage tones: totals read neutral (navy) and
// deductions read negative (red) instead of the flow-direction green a decrease
// would otherwise take. Exercises the opt-in Encoding.tone channel.
const tonedBridgePanel: Panel = {
  id: 'result-bridge', kind: 'cascade', title: 'Underwriting result', semantics: 'reconciliation', frame: 'bridge',
  total: 1740000,
  encoding: { label: 'stage', value: 'balance', cut: 'movement', cutLabel: 'movementLabel', final: 'reconciled', tone: 'tone' },
  format: {
    balance: { kind: 'money', currency: 'USD', minorUnits: false, precision: 0 },
    movement: { kind: 'money', currency: 'USD', minorUnits: false, precision: 0 },
  },
  presentation: { bridgeLayout: 'waterfall' },
  terminal: true,
  actions: [],
}

const tonedBridgeFrame: Frame = {
  columns: [
    { name: 'stage', type: 'string' }, { name: 'balance', type: 'number' },
    { name: 'movement', type: 'number' }, { name: 'movementLabel', type: 'string' },
    { name: 'reconciled', type: 'bool' }, { name: 'tone', type: 'string' },
  ],
  rows: [
    ['Gross premium', 3120000, 0, '', false, 'neutral'],
    ['Net of claims', 2310000, 810000, 'Claims paid', false, 'negative'],
    ['Net of acquisition', 1980000, 330000, 'Acquisition cost', false, 'negative'],
    ['Underwriting result', 1740000, 240000, 'Operating expenses', true, 'neutral'],
  ],
}

// The official underwriting-result bridge as the backend emits it: five running
// totals that each deduct from the earned premium, then an explicit final=true
// closing row that RESTATES the last running total (zero movement, cut=0). The
// waterfall must draw that closing row exactly once as the navy total — the
// zero-delta final row must not also render as a spurious "+0" duplicate bar
// immediately before it.
const officialResultPanel: Panel = {
  id: 'official-result-bridge', kind: 'cascade', title: 'Андеррайтинговый результат', semantics: 'reconciliation', frame: 'bridge',
  total: 66.34,
  encoding: {
    label: 'stage', value: 'balance', cut: 'movement', cutLabel: 'movementLabel',
    final: 'reconciled', tone: 'tone', split: 'split', splitLabel: 'splitLabel',
  },
  format: {
    balance: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 2 },
    movement: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 2 },
  },
  presentation: { bridgeLayout: 'waterfall' },
  terminal: true,
  actions: [],
}

const officialResultFrame: Frame = {
  columns: [
    { name: 'stage', type: 'string' }, { name: 'balance', type: 'number' },
    { name: 'movement', type: 'number' }, { name: 'movementLabel', type: 'string' },
    { name: 'reconciled', type: 'bool' }, { name: 'tone', type: 'string' },
    { name: 'split', type: 'number' }, { name: 'splitLabel', type: 'string' },
  ],
  rows: [
    ['Заработанная премия', 178.30, 0, '', false, 'neutral', 0, ''],
    ['Аквизиционные расходы', 150.00, 28.30, 'Аквизиционные расходы', false, 'negative', 0, ''],
    ['Расходы на ведение дела', 120.00, 30.00, 'Расходы на ведение дела', false, 'negative', 0, ''],
    // The one split stage: part of the payout is met from the product's own
    // reserve, the rest overflows beyond it. Both halves are the same cost, so
    // they share one bar and one label instead of a caption under the axis.
    ['Выплаты', 95.00, 25.00, 'Выплаты', false, 'negative', 6.10, 'сверх резерва'],
    ['Перестрахование', 80.00, 15.00, 'Перестрахование', false, 'negative', 0, ''],
    ['Движение резервов', 66.34, 13.66, 'Движение резервов', false, 'negative', 0, ''],
    // Closing row restates the running total, but as a floating-point sum it
    // lands an epsilon off (like the real backend frame) — the tolerance, not
    // strict equality, must suppress the "+0" duplicate bar.
    ['Андеррайтинговый результат', 66.34 + 1e-5, 0, '', true, 'neutral', 0, ''],
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

// The panel note lives behind the header's ⓘ, not in a caption band above the
// plot: a chart card's caption and info are merged there, and the bubble is the
// only place either is readable on screen. Both themes, opened on mount so the
// bubble itself is the subject.
const notedPanel: Panel = {
  ...cascadePanel,
  id: 'noted-bridge',
  caption: 'Rolled up by contributing year, so the total differs from the event-based composition next to it.',
  info: 'Also the gross RNP / UPR — a management calculation under the A/B/C rules, not the official net RNP.',
}

function OpenInfoTip({ children }: { children: React.ReactNode }) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    let cancelled = false
    let attempts = 0
    const open = () => {
      if (cancelled) return
      const button = ref.current?.querySelector<HTMLElement>('.lens-info-tip-button')
      if (button) {
        button.click()
        return
      }
      if (attempts++ < 60) window.requestAnimationFrame(open)
    }
    void window.document.fonts.ready.then(() => {
      window.requestAnimationFrame(() => window.requestAnimationFrame(open))
    })
    return () => { cancelled = true }
  }, [])
  return <div ref={ref}>{children}</div>
}

function InfoTipStory({ theme }: { theme: 'light' | 'dark' }) {
  const document = storyDocument(notedPanel, { bridge: cascadeFrame })
  return (
    <div className="lens-root" data-theme={theme}>
      <DocumentProvider initialDocument={document}>
        <DashboardRuntimeProvider locale="en">
          <OpenInfoTip><CascadePanel panel={notedPanel} /></OpenInfoTip>
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const PanelInfoTipLight: Story = () => <InfoTipStory theme="light" />
PanelInfoTipLight.storyName = 'Panel info tip light'

export const PanelInfoTipDark: Story = () => <InfoTipStory theme="dark" />
PanelInfoTipDark.storyName = 'Panel info tip dark'

export const CascadeFinalStage: Story = () => {
  const document = storyDocument(cascadePanel, { bridge: cascadeFrame })
  return <Runtime document={document}><CascadePanel panel={cascadePanel} /></Runtime>
}

// Waterfall projection: deduction bars read red, opening/closing totals navy.
export const WaterfallSemanticTone: Story = () => {
  const document = storyDocument(tonedBridgePanel, { bridge: tonedBridgeFrame })
  return <Runtime document={document}><CascadePanel panel={tonedBridgePanel} /></Runtime>
}

// Regression guard for the duplicate closing column: an explicit final=true
// running-total row that restates the last stage must render once as the navy
// total, preceded by the navy opening and five red deduction bars — no "+0".
export const WaterfallClosingTotal: Story = () => {
  const document = storyDocument(officialResultPanel, { bridge: officialResultFrame })
  return <Runtime document={document}><CascadePanel panel={officialResultPanel} /></Runtime>
}

// A split on a late column: the callout must flip to the inner side rather than
// run off the right edge of the plot. Also covers the guards — a split equal to
// the whole movement, and one larger than it, both leave the bar undivided.
export const WaterfallSplitCallout: Story = () => {
  const frame: Frame = {
    ...officialResultFrame,
    rows: [
      ['Заработанная премия', 178.30, 0, '', false, 'neutral', 0, ''],
      ['Аквизиционные расходы', 150.00, 28.30, 'Аквизиционные расходы', false, 'negative', 28.30, 'вся сумма'],
      ['Расходы на ведение дела', 120.00, 30.00, 'Расходы на ведение дела', false, 'negative', 44.00, 'больше движения'],
      ['Выплаты', 95.00, 25.00, 'Выплаты', false, 'negative', 0, ''],
      ['Перестрахование', 80.00, 15.00, 'Перестрахование', false, 'negative', 0, ''],
      ['Движение резервов', 66.34, 13.66, 'Движение резервов', false, 'negative', 5.20, 'сверх резерва'],
      ['Андеррайтинговый результат', 66.34 + 1e-5, 0, '', true, 'neutral', 0, ''],
    ],
  }
  const document = storyDocument(officialResultPanel, { bridge: frame })
  return <Runtime document={document}><CascadePanel panel={officialResultPanel} /></Runtime>
}

// Two totals in one cascade: the statutory underwriting result is a checkpoint
// the reader recognises, and the remaining reserve movements carry on from it
// to the pre-tax result. The checkpoint stands on zero where it was declared
// (not hoisted to the end) and renders hollow, so the one solid navy column
// stays unambiguously the finish.
export const WaterfallCheckpointTotal: Story = () => {
  const frame: Frame = {
    ...officialResultFrame,
    rows: [
      ['Заработанная премия', 200.41, 0, '', false, 'neutral', 0, ''],
      ['Исходящее перестрахование', 185.73, 14.68, 'Исходящее перестрахование', false, 'negative', 0, ''],
      ['Страховые выплаты', 180.10, 5.63, 'Страховые выплаты', false, 'negative', 0, ''],
      ['Операционные расходы', 133.92, 46.18, 'Операционные расходы', false, 'negative', 0, ''],
      ['Андеррайтинговый результат', 133.92 + 1e-5, 0, '', true, 'neutral', 0, ''],
      ['Изменение РЗУ', 116.71, 17.21, 'Изменение РЗУ', false, 'negative', 0, ''],
      ['Изменение РПНУ', 104.37, 12.34, 'Изменение РПНУ', false, 'negative', 0, ''],
      ['Изменение резерва катастроф', 106.05, -1.68, 'Изменение резерва катастроф', false, 'positive', 0, ''],
      ['Результат до налогообложения', 106.05, 0, '', true, 'neutral', 0, ''],
    ],
  }
  const document = storyDocument(officialResultPanel, { bridge: frame })
  return <Runtime document={document}><CascadePanel panel={officialResultPanel} /></Runtime>
}

// Cascade-list projection of the same toned bridge: track fill and value text
// follow the per-stage tone.
export const CascadeSemanticTone: Story = () => {
  const panel: Panel = { ...tonedBridgePanel, id: 'result-cascade', presentation: undefined }
  const document = storyDocument(panel, { bridge: tonedBridgeFrame })
  return <Runtime document={document}><CascadePanel panel={panel} /></Runtime>
}

const navigableBridgeFrame: Frame = {
  columns: [...tonedBridgeFrame.columns, { name: 'detailUrl', type: 'string' }],
  rows: tonedBridgeFrame.rows.map((row, index) => [...row, `/analytics/result/${index}`]),
}

// The same cascade list once each stage opens something. Its whole affordance is
// a pointer state, so this story exists to be hovered rather than compared: at
// rest an activatable stage and an inert one are the same geometry to the pixel,
// which is the point — the bleed the plate needs is cancelled by the padding
// that carries it, so switching a cascade to navigable moves nothing on screen
// until a pointer or a Tab arrives.
export const CascadeStagesNavigate: Story = () => {
  const panel: Panel = {
    ...tonedBridgePanel,
    id: 'navigable-cascade',
    frame: 'navigable-bridge',
    presentation: undefined,
    actions: [{ kind: 'navigate', urlSource: { kind: 'field', name: 'detailUrl' }, params: [], payload: {} }],
  }
  const document = storyDocument(panel, { 'navigable-bridge': navigableBridgeFrame })
  return <Runtime document={document}><CascadePanel panel={panel} /></Runtime>
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
