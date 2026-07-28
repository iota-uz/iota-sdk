import type { Story } from '@ladle/react'
import type { DashboardDocument, Frame, Level, Panel } from './contract'
import { PrintReportView } from './print'
import type { PrintReport, PrintSection } from './runtime/print'
import { DashboardRuntimeProvider, DocumentProvider } from './runtime'
import './styles.css'

/**
 * The printed report, rendered on canvas exactly as the print engine receives
 * it. Printing is otherwise unreviewable — the browser's print dialog blocks
 * the page — so this story is where its typography, chapter structure and
 * page furniture are inspected and held still by visual regression.
 */

const money = { kind: 'money', currency: 'UZS', minorUnits: false, precision: 1, compact: true } as const
const percent = { kind: 'percent', minorUnits: false, precision: 1 } as const

function frame(columns: Array<string>, rows: Array<Array<unknown>>): Frame {
  return {
    columns: columns.map((name) => ({ name, type: name === 'label' ? 'string' : 'number' })),
    rows,
  }
}

const heroPanel: Panel = {
  id: 'hero-result',
  kind: 'stat',
  title: 'Underwriting result',
  semantics: 'evidence',
  frame: 'hero-result:root',
  encoding: { label: 'label', value: 'value' },
  format: { value: money },
  actions: [],
  caption: 'Earned basis · after acquisition, claims and operating expenses',
  status: { label: 'Actual', tone: 'positive' },
}

const heroRatio: Panel = {
  id: 'hero-combined',
  kind: 'stat',
  title: 'Combined ratio',
  semantics: 'evidence',
  frame: 'hero-combined:root',
  encoding: { label: 'label', value: 'value' },
  format: { value: percent },
  actions: [],
  caption: 'Claims and expenses against earned premium',
}

const heroReserve: Panel = {
  id: 'hero-reserve',
  kind: 'stat',
  title: 'IBNR / RPNU',
  semantics: 'evidence',
  frame: 'hero-reserve:root',
  encoding: { label: 'label', value: 'value' },
  format: { value: money },
  actions: [],
  status: { label: 'Proxy', tone: 'warning' },
  caption: 'Management estimate pending the actuarial run',
  // Only reachable by hovering on screen: quality, the ⓘ explanation, and the
  // trend line beside the value.
  confidence: 'proxy',
  info: 'Chain-ladder over paid claims; the quarter shown may differ from the reporting date.',
  sparkline: { values: [2.1, 2.4, 2.9, 3.2, 3.4, 3.6, 3.9, 4.1] },
}

const backlogFlow: Panel = {
  id: 'claims-backlog',
  kind: 'metric_flow',
  title: 'Claims handling',
  semantics: 'reconciliation',
  frame: 'claims-backlog:root',
  encoding: { id: 'key', value: 'value' },
  format: { value: { kind: 'number', minorUnits: false, precision: 0 } },
  actions: [],
  metricFlow: {
    stages: [
      { key: 'opening', label: 'Opening backlog', role: 'input', confidence: 'proxy', caption: 'Reconstructed from decision dates' },
      { key: 'incoming', label: 'Claims reported', role: 'add', confidence: 'verified' },
      { key: 'closed', label: 'Claims decided', role: 'subtract', confidence: 'verified' },
      { key: 'closing', label: 'Closing backlog', role: 'result', confidence: 'proxy' },
    ],
    reconcile: { tolerance: 0 },
  },
}

const reserveHierarchy: Panel = {
  id: 'reserve-composition',
  kind: 'metric_hierarchy',
  title: 'Technical reserves',
  semantics: 'partition',
  frame: 'reserve-composition:root',
  encoding: { id: 'key', value: 'value', share: 'share' },
  format: { value: money, share: percent },
  actions: [],
  metricHierarchy: {
    rows: [
      { key: 'total', label: 'Technical reserves', confidence: 'requires_reconciliation' },
      { key: 'upr', label: 'Unearned premium', parent: 'total' },
      { key: 'rbns', label: 'Reported but not settled', parent: 'total' },
      { key: 'ibnr', label: 'Incurred but not reported', parent: 'total', availability: 'config_required' },
      { key: 'rest', label: 'Unallocated', parent: 'total', unallocated: true },
    ],
    reconcile: { tolerance: 0 },
  },
}

const premiumPanel: Panel = {
  id: 'premium-mix',
  kind: 'donut',
  title: 'Portfolio GWP',
  semantics: 'partition',
  frame: 'premium-mix:root',
  encoding: { label: 'label', value: 'value' },
  format: { value: money },
  actions: [],
  caption: 'Written premium split by recognition at the period end.',
}

const bridgePanel: Panel = {
  id: 'result-bridge',
  kind: 'cascade',
  title: 'How the result is formed',
  semantics: 'reconciliation',
  frame: 'result-bridge:root',
  encoding: { label: 'label', value: 'value', cut: 'cut', final: 'final', tone: 'tone' },
  format: { value: money, cut: money },
  presentation: { bridgeLayout: 'waterfall' },
  actions: [],
  caption: 'Every deduction is charged in the period it belongs to.',
  metricRelationship: {
    source: { key: 'earned', label: 'Earned premium' },
    target: { key: 'result', label: 'Underwriting result' },
    type: 'derivation',
    note: 'Claims are read from the accounting ledger, capped at the premium of their own product.',
  },
}

const productsPanel: Panel = {
  id: 'product-margin',
  kind: 'table',
  title: 'Margin before shared expenses',
  semantics: 'evidence',
  frame: 'product-margin:root',
  encoding: { label: 'product', value: 'result' },
  format: { start: money, claims: money, result: money, ratio: percent, delta: money, delta_pct: percent },
  columns: [
    { field: 'product', label: 'Product', cell: { kind: 'plain' }, badgeField: 'hint' },
    { field: 'start', label: 'Earned premium, UZS', align: 'right', cell: { kind: 'plain' } },
    { field: 'claims', label: 'Claims paid, UZS', align: 'right', cell: { kind: 'plain' } },
    { field: 'ratio', label: 'Loss ratio', align: 'right', cell: { kind: 'plain' } },
    { field: 'result', label: 'Result, UZS', align: 'right', cell: { kind: 'bar' } },
    {
      field: 'delta',
      label: 'vs previous, UZS',
      align: 'right',
      cell: { kind: 'delta', secondaryField: 'delta_pct', layout: 'stacked' },
    },
  ],
  actions: [],
}

const claimsPanel: Panel = {
  id: 'claims-years',
  kind: 'line',
  title: 'Claims paid by year',
  semantics: 'series',
  frame: 'claims-years:root',
  encoding: { label: 'label', value: 'value' },
  format: { value: money },
  actions: [],
}

const dashboard: DashboardDocument = {
  version: '1.0.0',
  snapshotId: 'print-story-snapshot',
  meta: { dashboardId: 'profitability', title: 'Profitability', generatedAt: '2026-07-19T09:30:00Z', locale: 'en' },
  layout: {
    rows: [
      {
        heading: 'Summary',
        panels: [
          { panelId: 'hero-result', span: 4 },
          { panelId: 'hero-combined', span: 4 },
          { panelId: 'hero-reserve', span: 4 },
        ],
      },
      {
        heading: 'Premium',
        panels: [{
          panelId: 'premium-mix',
          span: 6,
          group: {
            id: 'premium',
            kind: 'metrics',
            span: 12,
            caption: 'Written premium, before any reinsurance is ceded.',
          },
        }],
      },
      { heading: 'Result formation', panels: [{ panelId: 'result-bridge', span: 12 }, { panelId: 'product-margin', span: 12 }] },
      {
        heading: 'Claims and reserves',
        panels: [
          { panelId: 'claims-years', span: 12 },
          { panelId: 'claims-backlog', span: 6 },
          { panelId: 'reserve-composition', span: 6 },
        ],
      },
    ],
  },
  panels: [
    heroPanel, heroRatio, heroReserve, premiumPanel, bridgePanel, productsPanel, claimsPanel,
    backlogFlow, reserveHierarchy,
  ],
  frames: {},
  drill: { inlineDepth: 0, edges: {} },
  perspectives: [],
  filters: [{
    id: 'period',
    kind: 'period',
    period: { startParam: 'from', endParam: 'to', value: { start: '2026-01-01', end: '2026-06-30' } },
  }],
  endpoints: {},
  i18n: {},
  theme: {
    palette: { accent: '#2563eb' },
    series: {
      Earned: '#2563eb',
      Unearned: '#60a5fa',
      'Incoming reinsurance': '#a855f7',
      'Reissue credits': '#f59e0b',
      'Motor third party': '#2563eb',
      Property: '#60a5fa',
      Cargo: '#a855f7',
      Travel: '#f59e0b',
      Accident: '#dc2626',
    },
  },
  header: { title: 'Profitability', subtitle: 'Management view · earned basis' },
}

function section(panel: Panel, rows: Frame, overrides: Partial<PrintSection> = {}): PrintSection {
  const level: Level = { path: [], label: panel.title, children: [], perspectives: [] }
  return {
    id: `${panel.id}:${overrides.breadcrumb?.join('/') ?? 'root'}`,
    document: dashboard,
    panel,
    level,
    frame: rows,
    path: [],
    breadcrumb: [panel.title],
    depth: 0,
    root: true,
    ...overrides,
  }
}

const report: PrintReport = {
  document: dashboard,
  sections: [
    section(heroPanel, frame(['label', 'value'], [['Underwriting result', 18_400_000_000]])),
    section(heroRatio, frame(['label', 'value'], [['Combined ratio', 88.3]])),
    section(heroReserve, frame(['label', 'value'], [['IBNR / RPNU', 4_100_000_000]])),
    section(premiumPanel, frame(['label', 'value'], [
      ['Earned', 178_880_000_000],
      ['Unearned', 96_240_000_000],
      ['Incoming reinsurance', 21_100_000_000],
      ['Reissue credits', 2_400_000_000],
    ])),
    section(premiumPanel, frame(['label', 'value'], [
      ['Motor third party', 96_100_000_000],
      ['Property', 42_300_000_000],
      ['Cargo', 24_800_000_000],
      ['Travel', 9_600_000_000],
      ['Accident', 6_080_000_000],
    ]), {
      root: false,
      breadcrumb: ['Portfolio GWP', 'Earned', 'By product'],
    }),
    section(bridgePanel, {
      columns: [
        { name: 'label', type: 'string' },
        { name: 'value', type: 'number' },
        { name: 'cut', type: 'number' },
        { name: 'final', type: 'bool' },
        { name: 'tone', type: 'string' },
      ],
      rows: [
        ['Earned premium', 178_880_000_000, 0, false, 'neutral'],
        ['Outward reinsurance', 166_400_000_000, -12_480_000_000, false, 'negative'],
        ['Acquisition cost', 141_900_000_000, -24_500_000_000, false, 'negative'],
        ['Claims paid', 110_430_000_000, -31_470_000_000, false, 'negative'],
        ['Operating expenses', 18_400_000_000, -92_030_000_000, false, 'negative'],
        ['Underwriting result', 18_400_000_000, 0, true, 'positive'],
      ],
    }),
    section(productsPanel, {
      columns: [
        { name: 'product', type: 'string' },
        { name: 'start', type: 'number' },
        { name: 'claims', type: 'number' },
        { name: 'ratio', type: 'number' },
        { name: 'result', type: 'number' },
        { name: 'delta', type: 'number' },
        { name: 'delta_pct', type: 'number' },
        { name: 'hint', type: 'string' },
      ],
      rows: [
        ['Motor third party', 96_100_000_000, 21_400_000_000, 22.3, 74_700_000_000, 6_200_000_000, 9.1, ''],
        ['Property', 42_300_000_000, 6_120_000_000, 14.5, 36_180_000_000, -1_450_000_000, -3.9, ''],
        ['Cargo', 24_800_000_000, 2_940_000_000, 11.9, 21_860_000_000, 940_000_000, 4.5, ''],
        ['Travel', 9_600_000_000, 890_000_000, 9.3, 8_710_000_000, -220_000_000, -2.5, 'Discontinued product'],
        ['Accident', 6_080_000_000, 7_320_000_000, 120.4, -1_240_000_000, -3_100_000_000, -142.0, 'Loss ratio above 100%'],
      ],
    }),
    section(backlogFlow, {
      columns: [{ name: 'key', type: 'string' }, { name: 'value', type: 'number' }],
      rows: [
        ['opening', 1_240],
        ['incoming', 3_180],
        ['closed', 2_910],
        ['closing', 1_510],
      ],
    }),
    section(reserveHierarchy, {
      columns: [
        { name: 'key', type: 'string' },
        { name: 'value', type: 'number' },
        { name: 'share', type: 'number' },
      ],
      rows: [
        ['total', 96_240_000_000, 100],
        ['upr', 55_410_000_000, 57.6],
        ['rbns', 24_700_000_000, 25.7],
        ['ibnr', 0, 0],
        ['rest', 16_130_000_000, 16.7],
      ],
    }),
    section(claimsPanel, frame(['label', 'value'], [
      ['2021', 11_200_000_000],
      ['2022', 14_900_000_000],
      ['2023', 22_600_000_000],
      ['2024', 28_400_000_000],
      ['2025', 31_470_000_000],
    ])),
  ],
  warnings: ['Claims performance › By adjuster: query request failed'],
  truncated: false,
}

function Printed() {
  return (
    <div className="lens-root">
      <DocumentProvider initialDocument={dashboard}>
        <DashboardRuntimeProvider locale="en">
          <article className="lens-print-report" aria-hidden="false" data-preview="true" lang="en">
            <PrintReportView report={report} />
          </article>
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const CoverAndContents: Story = () => <Printed />
CoverAndContents.storyName = 'Composed report'
