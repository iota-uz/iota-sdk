import type { Story } from '@ladle/react'
import { useEffect } from 'react'
import type { DashboardDocument, Frame, Panel, PanelTemporal } from './contract'
import { DashboardPanels } from './DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider } from './runtime'
import computedForecastFrame from '../fixtures/linear-forecast.json'
import './styles.css'

const values = [82, 86, 84, 91, 95, 93, 99, 104, 108, 106, 112, 116]
const frame: Frame = {
  columns: [
    { name: 'month', type: 'time' },
    { name: 'series', type: 'string' },
    { name: 'value', type: 'number' },
    { name: 'regression', type: 'number' },
    { name: 'sma_3', type: 'number' },
    { name: 'sma_7', type: 'number' },
    { name: 'sma_12', type: 'number' },
    { name: 'annualized', type: 'number' },
    { name: 'forecast', type: 'number' },
    { name: 'forecast_lower', type: 'number' },
    { name: 'forecast_upper', type: 'number' },
  ],
  rows: values.map((value, index) => {
    const trend = 81 + index * 3.1
    const mean = (window: number) => index + 1 < window
      ? null
      : values.slice(index + 1 - window, index + 1).reduce((sum, item) => sum + item, 0) / window
    const forecast = index >= 9 ? 108 + (index - 8) * 4 : null
    return [
      `2026-${String(index + 1).padStart(2, '0')}-01T00:00:00Z`,
      'Observed', value, trend, mean(3), mean(7), mean(12),
      index === 11 ? 124 : null,
      forecast, forecast === null ? null : forecast - 5, forecast === null ? null : forecast + 6,
    ]
  }),
}

function panel(temporal: PanelTemporal, title: string): Panel {
  return {
    id: 'temporal', kind: 'line', title, semantics: 'series', frame: 'temporal:frame',
    encoding: { category: 'month', series: 'series', value: 'value' },
    format: {
      month: { kind: 'date', layout: 'Jan', minorUnits: false },
      value: { kind: 'number', minorUnits: false, precision: 0 },
    },
    temporal,
    actions: [], terminal: true,
  }
}

function storyDocument(storyPanel: Panel, storyFrame: Frame = frame): DashboardDocument {
  return {
    version: '1.0.0', snapshotId: `temporal-${storyPanel.title}`,
    meta: { dashboardId: 'temporal-overlays', title: storyPanel.title, generatedAt: '2026-07-31T00:00:00Z', locale: 'en' },
    header: { title: 'Time-series readability', subtitle: 'Server-declared analytical overlays' },
    layout: { rows: [{ panels: [{ panelId: storyPanel.id, span: 9 }] }] },
    panels: [storyPanel], frames: { 'temporal:frame': storyFrame },
    drill: { inlineDepth: 0, edges: {} }, perspectives: [], endpoints: {}, i18n: {},
    theme: { palette: {}, series: { 'temporal:0': '#2563eb' } },
  }
}

function ActivateControl({ kind }: { kind: 'regression' | 'average' }) {
  useEffect(() => runWhenReady(() => {
    if (kind === 'regression') {
      const button = document.querySelector<HTMLButtonElement>('.lens-temporal-toggle')
      if (!button) return false
      button.click()
      return true
    }
    const select = document.querySelector<HTMLSelectElement>('.lens-temporal-select')
    if (!select) return false
    select.value = '3'
    select.dispatchEvent(new Event('change', { bubbles: true }))
    return true
  }), [kind])
  return null
}

function runWhenReady(action: () => boolean): () => void {
  let cancelled = false
  const run = () => {
    if (cancelled || action()) return
    window.requestAnimationFrame(run)
  }
  window.requestAnimationFrame(() => window.requestAnimationFrame(run))
  return () => { cancelled = true }
}

function TemporalStory({ storyPanel, storyFrame, control }: { storyPanel: Panel; storyFrame?: Frame; control?: 'regression' | 'average' }) {
  const temporalDocument = storyDocument(storyPanel, storyFrame)
  return (
    <div className="lens-root" data-theme="light">
      <DocumentProvider initialDocument={temporalDocument}>
        <DashboardRuntimeProvider locale="en">
          <DashboardPanels />
          {control && <ActivateControl kind={control} />}
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const Regression: Story = () => (
  <TemporalStory
    control="regression"
    storyPanel={panel({ regression: { field: 'regression', label: 'Trend' } }, 'Linear regression')}
  />
)
Regression.storyName = 'Regression'

export const MovingAverage: Story = () => (
  <TemporalStory
    control="average"
    storyPanel={panel({ movingAverages: [
      { window: 3, field: 'sma_3', label: 'SMA 3' },
      { window: 7, field: 'sma_7', label: 'SMA 7' },
      { window: 12, field: 'sma_12', label: 'SMA 12' },
    ] }, 'Moving average')}
  />
)
MovingAverage.storyName = 'Moving average'

export const ReferenceLines: Story = () => (
  <TemporalStory storyPanel={panel({ referenceLines: [
    { value: 100, label: 'Operating threshold' },
    { value: 110, label: 'Stretch threshold' },
  ] }, 'Reference lines')} />
)
ReferenceLines.storyName = 'Reference lines'

export const IncompletePeriod: Story = () => (
  <TemporalStory storyPanel={panel({
    period: { category: '2026-12-01T00:00:00Z', state: 'annualized', label: 'Estimate', annualizedField: 'annualized' },
  }, 'Incomplete period')} />
)
IncompletePeriod.storyName = 'Incomplete period'

export const TimeAnnotations: Story = () => (
  <TemporalStory storyPanel={panel({ annotations: [
    { at: '2026-04-01T00:00:00Z', label: 'Method changed' },
    { at: '2026-08-01T00:00:00Z', label: 'New process' },
  ] }, 'Time annotations')} />
)
TimeAnnotations.storyName = 'Time annotations'

export const ForecastConfidence: Story = () => (
  <TemporalStory
    storyFrame={computedForecastFrame as Frame}
    storyPanel={panel({ forecast: {
      start: '2026-06-01T00:00:00Z', valueField: 'forecast', lowerField: 'forecast_lower', upperField: 'forecast_upper', label: 'Server-computed forecast',
    } }, 'Forecast confidence')}
  />
)
ForecastConfidence.storyName = 'Forecast confidence'
