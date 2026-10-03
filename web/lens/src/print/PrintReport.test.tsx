import { render } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import type { ChartInput } from '../charts/adapter'
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
  const view = render(<PrintReportView report={{ document, sections: [section], warnings: [], truncated: false }} />)
  expect(view.getByTestId('printed-chart')).toBeInTheDocument()
  expect(inputs.at(-1)?.seriesColor?.('Alpha', 0)).toBe('#123456')
  expect(inputs.at(-1)?.seriesColor?.('Beta', 1)).toBe('#654321')
  expect([...view.container.querySelectorAll<HTMLElement>('.lens-print-swatch')].map(el => el.style.backgroundColor))
    .toEqual(['rgb(18, 52, 86)', 'rgb(101, 67, 33)', 'rgb(18, 52, 86)', 'rgb(101, 67, 33)'])
})
