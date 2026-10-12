import { render } from '@solidjs/testing-library'
import { expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import type { ChartInput } from '../charts/adapter'
import { buildChartOption } from '../charts/echarts/options'
import { buildEChartsTheme } from '../charts/echarts/theme'
import { parseDocument, type Frame, type Panel } from '../contract'
import type { PrintSection } from '../runtime/print'
import { PrintReportView } from './PrintReport'

const { inputs } = vi.hoisted(() => ({ inputs: [] as ChartInput[] }))
vi.mock('../panels/ChartHost', () => ({ ChartHost: ({ input }: { input: ChartInput }) => {
  inputs.push(input)
  return <div data-testid="printed-chart" />
} }))
vi.mock('../runtime', async (original) => ({
  ...await original<typeof import('../runtime')>(),
  useTranslate: () => (_key: string, fallback: string) => fallback,
}))

// Falsely green if only the evidence table is checked while the printed plot chooses another palette.
it('prints the chart and evidence swatches from the same served series pins', () => {
  const document = parseDocument(fixture)
  const panel: Panel = {
    id: 'printed-series', kind: 'bar', title: 'Sales', semantics: 'series', frame: 'sales',
    encoding: { category: 'category', series: 'series', value: 'value' }, format: {}, actions: [],
  }
  const frame: Frame = {
    columns: [{ name: 'category', type: 'string' }, { name: 'series', type: 'string' }, { name: 'value', type: 'number' }],
    rows: [['Jan', 'Alpha', 10], ['Jan', 'Beta', 20], ['Feb', 'Alpha', 30], ['Feb', 'Beta', 40]],
    colors: ['#123456', '#654321', '#123456', '#654321'],
  }
  document.panels = [panel]
  document.layout = { rows: [{ panels: [{ panelId: panel.id, span: 12 }] }] }
  const section: PrintSection = {
    id: 'sales', document, panel, frame, level: { path: [], label: 'Sales', children: [], perspectives: [] },
    path: [], breadcrumb: ['Sales'], depth: 0, root: true,
  }
  const view = render(() => <PrintReportView report={{ document, sections: [section], warnings: [], truncated: false }} />)
  expect(view.getByTestId('printed-chart')).toBeInTheDocument()
  expect(inputs.at(-1)?.seriesColor?.('Alpha', 0)).toBe('#123456')
  expect(inputs.at(-1)?.seriesColor?.('Beta', 1)).toBe('#654321')
  expect([...view.container.querySelectorAll<HTMLElement>('.lens-print-swatch')].map(el => el.style.backgroundColor))
    .toEqual(['rgb(18, 52, 86)', 'rgb(101, 67, 33)', 'rgb(18, 52, 86)', 'rgb(101, 67, 33)'])
})

// Falsely green if both categories share one pin or if audit colours bypass the actual adapter options.
it('prints partition audit swatches with the category colours used by every ring', () => {
  const document = parseDocument(fixture)
  const panel: Panel = {
    id: 'printed-rings', kind: 'radial', title: 'Partition', semantics: 'partition', frame: 'rings',
    encoding: { id: 'id', category: 'category', label: 'label', series: 'ring', value: 'value' }, format: {}, actions: [],
    radial: { mode: 'partition', rings: [{ key: 'one', label: 'First', total: 100 }, { key: 'two', label: 'Second', total: 100 }] },
  }
  const frame: Frame = {
    columns: [{ name: 'id', type: 'string' }, { name: 'category', type: 'string' }, { name: 'label', type: 'string' }, { name: 'ring', type: 'string' }, { name: 'value', type: 'number' }],
    rows: [['a', 'Category A', 'Alpha', 'one', 10], ['b', 'Category B', 'Beta', 'one', 90], ['a', 'Category A', 'Alpha', 'two', 40], ['b', 'Category B', 'Beta', 'two', 60]],
    colors: ['#123456', '#654321', '#123456', '#654321'],
  }
  document.panels = [panel]
  document.layout = { rows: [{ panels: [{ panelId: panel.id, span: 12 }] }] }
  const section: PrintSection = {
    id: 'rings', document, panel, frame, level: { path: [], label: 'Partition', children: [], perspectives: [] },
    path: [], breadcrumb: ['Partition'], depth: 0, root: true,
  }
  const view = render(() => <PrintReportView report={{ document, sections: [section], warnings: [], truncated: false }} />)
  const input = inputs.at(-1)!
  const option = buildChartOption(input, buildEChartsTheme(window.document.createElement('div'), input.theme))
  const marks = option.series as Array<{ data: Array<{ itemStyle: { color: string } }> }>
  const colours = marks.flatMap(ring => ring.data.map(mark => {
    const swatch = window.document.createElement('span')
    swatch.style.backgroundColor = mark.itemStyle.color
    return swatch.style.backgroundColor
  }))
  expect(new Set(colours).size).toBe(2)
  expect([...view.container.querySelectorAll<HTMLElement>('.lens-print-swatch')].map(el => el.style.backgroundColor)).toEqual(colours)
})
