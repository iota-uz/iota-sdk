import type { Story } from '@ladle/react'
import type { DashboardDocument, Frame, Panel } from './contract'
import { LensDashboard } from './LensDashboard'
import './styles.css'

const number = { kind: 'number', minorUnits: false, precision: 0, decimalSeparator: '.' } as const

const panels: Record<'histogram' | 'boxplot' | 'heatmap', Panel> = {
  histogram: {
    id: 'histogram', kind: 'histogram', title: 'Claim settlement days', semantics: 'series', frame: 'histogram:frame',
    encoding: { category: 'bucket', value: 'claims' }, format: { claims: number }, terminal: true, actions: [],
  },
  boxplot: {
    id: 'boxplot', kind: 'boxplot', title: 'Settlement distribution by product', semantics: 'series', frame: 'boxplot:frame',
    encoding: { category: 'product', lower: 'min', q1: 'q1', median: 'median', q3: 'q3', upper: 'max' },
    format: { min: number, q1: number, median: number, q3: number, max: number }, terminal: true, actions: [],
  },
  heatmap: {
    id: 'heatmap', kind: 'heatmap', title: 'Claims by weekday and hour', semantics: 'series', frame: 'heatmap:frame',
    encoding: { category: 'hour', series: 'weekday', value: 'claims' }, format: { claims: number }, terminal: true,
    presentation: { dataLabels: true }, actions: [],
  },
}

const frames: Record<string, Frame> = {
  'histogram:frame': {
    columns: [{ name: 'bucket', type: 'string' }, { name: 'claims', type: 'number' }],
    rows: [['0–7', 84], ['8–14', 142], ['15–30', 215], ['31–60', 128], ['61–90', 56], ['90+', 31]],
  },
  'boxplot:frame': {
    columns: [
      { name: 'product', type: 'string' }, { name: 'min', type: 'number' }, { name: 'q1', type: 'number' },
      { name: 'median', type: 'number' }, { name: 'q3', type: 'number' }, { name: 'max', type: 'number' },
    ],
    rows: [['OSAGO', 1, 8, 19, 42, 146], ['KASKO', 2, 12, 31, 68, 210], ['Travel', 0, 3, 7, 14, 48]],
  },
  'heatmap:frame': {
    columns: [{ name: 'hour', type: 'string' }, { name: 'weekday', type: 'string' }, { name: 'claims', type: 'number' }],
    rows: ['Mon', 'Tue', 'Wed', 'Thu', 'Fri'].flatMap((day, dayIndex) =>
      ['08:00', '12:00', '16:00', '20:00'].map((hour, hourIndex) => [hour, day, 4 + dayIndex * 3 + hourIndex * 5])),
  },
}

function documentFor(kind: keyof typeof panels): DashboardDocument {
  const panel = panels[kind]
  return {
    version: '1.0.0', snapshotId: `distribution-${kind}`,
    meta: { dashboardId: `distribution-${kind}`, title: 'Insurance distributions', generatedAt: '2026-07-31T00:00:00Z', locale: 'en' },
    layout: { rows: [{ panels: [{ panelId: panel.id, span: 12 }] }] }, panels: [panel], frames,
    drill: { inlineDepth: 0, edges: {} }, perspectives: [], endpoints: {}, i18n: {}, theme: { palette: {}, series: {} },
  }
}

function Scene({ kind }: { kind: keyof typeof panels }) {
  return <div style={{ width: 820 }}><LensDashboard initialDocument={documentFor(kind)} theme="light" /></div>
}

export const Histogram: Story = () => <Scene kind="histogram" />
export const BoxPlot: Story = () => <Scene kind="boxplot" />
export const Heatmap: Story = () => <Scene kind="heatmap" />
