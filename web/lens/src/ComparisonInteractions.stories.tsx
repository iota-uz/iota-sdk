import type { Story } from '@ladle/react'
import type { DashboardDocument, Panel } from './contract'
import { LensDashboard } from './LensDashboard'
import './styles.css'

const stat: Panel = {
  id: 'premium', kind: 'stat', title: 'Written premium', semantics: 'series', frame: 'premium:frame',
  encoding: { value: 'premium' },
  format: {
    premium: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0, compact: true },
    premium_delta: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0, compact: true },
    premium_delta_percent: { kind: 'percent', minorUnits: false, precision: 1 },
  },
  trend: { percent: 0, absoluteField: 'premium_delta', percentField: 'premium_delta_percent', label: 'vs previous period' },
  terminal: true, actions: [],
}

const line: Panel = {
  id: 'monthly', kind: 'line', title: 'Premium by month', semantics: 'series', frame: 'monthly:frame',
  encoding: { category: 'month', value: 'premium', previous: 'previous_premium' },
  format: { premium: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0, compact: true } },
  actions: [{
    kind: 'cross_filter', urlTemplate: '/sales', params: [], payload: {},
    filter: { dimension: 'month', value: { kind: 'field', name: 'month' } },
  }],
}

const comparisonTable: Panel = {
  id: 'regions', kind: 'table', title: 'Premium by region', semantics: 'evidence', frame: 'regions:frame',
  encoding: { id: 'region', label: 'region', value: 'premium' },
  format: {
    previous_premium: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0, compact: true },
    premium: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0, compact: true },
    premium_delta: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0, compact: true },
    premium_delta_percent: { kind: 'percent', minorUnits: false, precision: 1 },
  },
  columns: [
    { field: 'region', label: 'Region', cell: { kind: 'plain' } },
    { field: 'previous_premium', label: 'Before', align: 'right', cell: { kind: 'plain' } },
    { field: 'premium', label: 'After', align: 'right', cell: { kind: 'plain' } },
    { field: 'premium_delta', label: 'Δ', align: 'right', cell: { kind: 'delta', secondaryField: 'premium_delta_percent', layout: 'stacked' } },
  ],
  actions: [{
    kind: 'cross_filter', urlTemplate: '/sales', params: [], payload: {},
    filter: { dimension: 'region', value: { kind: 'field', name: 'region' } },
  }],
}

const comparisonDocument: DashboardDocument = {
  version: '1.0.0', snapshotId: 'comparison-interactions',
  meta: { dashboardId: 'comparison', title: 'Sales comparison', generatedAt: '2026-07-31T00:00:00Z', locale: 'en' },
  layout: { rows: [
    { panels: [{ panelId: 'premium', span: 4 }] },
    { heading: 'Current and prior period', panels: [{ panelId: 'monthly', span: 12 }] },
    { heading: 'Before / after / change', panels: [{ panelId: 'regions', span: 12 }] },
  ] },
  panels: [stat, line, comparisonTable],
  frames: {
    'premium:frame': {
      columns: [
        { name: 'premium', type: 'number' }, { name: 'premium_delta', type: 'number' },
        { name: 'premium_delta_percent', type: 'number' },
      ],
      rows: [[4_860_000_000, 540_000_000, 12.5]],
    },
    'monthly:frame': {
      columns: [
        { name: 'month', type: 'time' }, { name: 'premium', type: 'number' },
        { name: 'previous_premium', type: 'number' },
      ],
      rows: [
        ['2026-01-01', 620_000_000, 560_000_000], ['2026-02-01', 710_000_000, 650_000_000],
        ['2026-03-01', 780_000_000, 690_000_000], ['2026-04-01', 850_000_000, 760_000_000],
        ['2026-05-01', 900_000_000, 810_000_000], ['2026-06-01', 1_000_000_000, 870_000_000],
      ],
    },
    'regions:frame': {
      columns: [
        { name: 'region', type: 'string' }, { name: 'previous_premium', type: 'number' },
        { name: 'premium', type: 'number' }, { name: 'premium_delta', type: 'number' },
        { name: 'premium_delta_percent', type: 'number' },
      ],
      rows: [
        ['Tashkent', 2_100_000_000, 2_430_000_000, 330_000_000, 15.7],
        ['Samarkand', 980_000_000, 1_040_000_000, 60_000_000, 6.1],
        ['Fergana', 760_000_000, 690_000_000, -70_000_000, -9.2],
      ],
    },
  },
  drill: { inlineDepth: 0, edges: {} }, perspectives: [],
  filters: [{
    id: 'compare', kind: 'compare', label: 'Compare',
    compare: {
      modeParam: 'compare', startParam: 'compare_start', endParam: 'compare_end', compareTo: 'period',
      value: { mode: 'previous_period' },
    },
  }],
  activeFilters: [
    { dimension: 'region', label: 'Region: Tashkent', value: 'tashkent', removeUrl: '/sales' },
    { dimension: 'product', label: 'Product: OSAGO', value: 'osago', removeUrl: '/sales?_f=region%3Atashkent' },
  ],
  resetFiltersUrl: '/sales', endpoints: {},
  i18n: { 'lens.filters.reset_all': 'Reset all', 'lens.chart.reset_zoom': 'Reset zoom' },
  theme: { palette: {}, series: {} },
}

export const PreviousPeriod: Story = () => (
  <div style={{ width: 1080 }}>
    <LensDashboard initialDocument={comparisonDocument} theme="light" />
  </div>
)
PreviousPeriod.storyName = 'Previous period'
