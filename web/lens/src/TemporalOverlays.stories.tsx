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

/** The same readings with the year before them, for the comparison ghost. */
const comparisonFrame: Frame = {
  columns: [...frame.columns, { name: 'previous', type: 'number' }],
  rows: frame.rows.map((row, index) => [...row, values[index]! - 9 - (index % 3)]),
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

/**
 * The shape /analytics/trends actually draws: a *category* axis of year keys
 * with three ratio series, a break-even threshold and a year still running.
 *
 * A category axis resolves both ends of a one-category band to the same point,
 * so a band there is drawn through a bar's sense of where a category starts and
 * ends. This is the story that proves it — the time-axis stories cannot.
 */
const ratioYears = ['2022', '2023', '2024', '2025', '2026']
const ratioSeries: Array<[string, number[]]> = [
  ['Loss ratio', [23.7, 53.4, 14.7, 18.9, 2.8]],
  ['Expense ratio', [44.0, 87.1, 68.3, 44.0, 30.1]],
  ['Combined ratio', [67.7, 140.4, 83.0, 62.9, 32.9]],
]
const ratioFrame: Frame = {
  columns: [
    { name: 'year', type: 'string' },
    { name: 'series', type: 'string' },
    { name: 'value', type: 'number' },
  ],
  rows: ratioYears.flatMap((year, index) => ratioSeries.map(([name, readings]) => [year, name, readings[index]!])),
}

const ratioPanel: Panel = {
  id: 'temporal', kind: 'line', title: 'Key ratios', semantics: 'series', frame: 'temporal:frame',
  encoding: { category: 'year', series: 'series', value: 'value' },
  format: { value: { kind: 'percent', minorUnits: false, precision: 1 } },
  presentation: { legend: 'below' },
  temporal: {
    referenceLines: [{ value: 100, label: 'Combined-ratio break-even' }],
    period: { category: '2026', state: 'ytd', label: 'Year to date' },
  },
  actions: [], terminal: true,
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

function ActivateControl({ kind }: { kind: 'regression' | 'average' | 'dismiss-overlay' }) {
  useEffect(() => runWhenReady(() => {
    if (kind === 'regression') {
      const button = document.querySelector<HTMLButtonElement>('.lens-temporal-toggle')
      if (!button) return false
      button.click()
      return true
    }
    if (kind === 'dismiss-overlay') {
      const entry = document.querySelector<HTMLButtonElement>('.lens-chart-overlay-legend .lens-chart-legend-toggle')
      if (!entry) return false
      entry.click()
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

function TemporalStory({ storyPanel, storyFrame, control }: { storyPanel: Panel; storyFrame?: Frame; control?: 'regression' | 'average' | 'dismiss-overlay' }) {
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
    period: { category: '2026-12-01T00:00:00Z', state: 'annualized', annualizedField: 'annualized' },
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

/**
 * Every treatment at once, with the legend that names them.
 *
 * This is the story the vocabulary is judged on: a threshold, an event, a
 * projection, an unfinished period and a fitted line all on one plot, each in
 * its own idiom, each with an entry beneath the data rows of the legend. Before
 * this they were four grey dashes and a stack of repeated labels.
 */
export const OverlayVocabulary: Story = () => (
  <TemporalStory
    control="regression"
    storyPanel={panel({
      regression: { field: 'regression', label: 'Trend' },
      referenceLines: [{ value: 100, label: 'Operating threshold' }],
      annotations: [{ at: '2026-04-01T00:00:00Z', label: 'Method changed' }],
      period: { category: '2026-12-01T00:00:00Z', state: 'annualized', annualizedField: 'annualized' },
    }, 'Overlay vocabulary')}
  />
)
OverlayVocabulary.storyName = 'Overlay vocabulary'

/**
 * The same panel with the threshold switched off from the legend.
 *
 * An overlay entry is not a series toggle: the plot loses the mark and nothing
 * else moves — no denominator changes, no other entry restyles — and the entry
 * stays listed so the mark can be brought back.
 */
export const OverlayDismissed: Story = () => (
  <TemporalStory
    control="dismiss-overlay"
    storyPanel={panel({
      referenceLines: [{ value: 100, label: 'Operating threshold' }],
      annotations: [{ at: '2026-04-01T00:00:00Z', label: 'Method changed' }],
    }, 'Overlay switched off')}
  />
)
OverlayDismissed.storyName = 'Overlay switched off'

/**
 * The combined-ratio panel: a category axis, three series, one threshold and a
 * year that has not finished. Its 100% line is the one that never drew.
 */
export const CategoryAxisOverlays: Story = () => (
  <TemporalStory storyFrame={ratioFrame} storyPanel={ratioPanel} />
)
CategoryAxisOverlays.storyName = 'Category-axis overlays'

/** The comparison ghost, named in the legend instead of drifting unlabelled. */
export const ComparisonGhost: Story = () => (
  <TemporalStory
    storyFrame={comparisonFrame}
    storyPanel={{
      ...panel({}, 'Comparison ghost'),
      encoding: { category: 'month', series: 'series', value: 'value', previous: 'previous' },
    }}
  />
)
ComparisonGhost.storyName = 'Comparison ghost'

export const ForecastConfidence: Story = () => (
  <TemporalStory
    storyFrame={computedForecastFrame as Frame}
    storyPanel={panel({ forecast: {
      start: '2026-06-01T00:00:00Z', valueField: 'forecast', lowerField: 'forecast_lower', upperField: 'forecast_upper', label: 'Server-computed forecast',
    } }, 'Forecast confidence')}
  />
)
ForecastConfidence.storyName = 'Forecast confidence'

/**
 * The header under pressure, which is where it used to break.
 *
 * A half-width card carrying a long name, a total badge and both temporal
 * controls: the cluster is wider than the space left beside the title, so it
 * wraps to its own row. Before, `justify-end` sent the overflow leftwards
 * instead — the controls painted over the last characters of the title and
 * completely over the ⓘ («СТРАХОВЫЕ ВЫПЛАТ|Регрессия»), the badge ellipsized to
 * a single letter, and the ⓘ was reparented into the icon cluster where it read
 * as another button.
 */
export const CrowdedPanelHeader: Story = () => {
  const storyPanel: Panel = {
    ...panel({
      regression: { field: 'regression', label: 'Trend' },
      movingAverages: [{ window: 3, field: 'sma_3', label: 'SMA 3' }],
    }, 'Insurance claims paid on motor third-party liability'),
    caption: 'Claims settled in the period, gross of reinsurance recoveries.',
    total: 1_842_500,
  }
  const crowded: DashboardDocument = {
    ...storyDocument(storyPanel),
    layout: { rows: [{ panels: [{ panelId: storyPanel.id, span: 6 }] }] },
  }
  return (
    <div className="lens-root" data-theme="light">
      <DocumentProvider initialDocument={crowded}>
        <DashboardRuntimeProvider locale="en">
          <DashboardPanels />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}
CrowdedPanelHeader.storyName = 'Crowded panel header'
