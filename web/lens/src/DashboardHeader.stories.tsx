import type { Story } from '@ladle/react'
import type { DashboardDocument, Filter, Panel } from './contract'
import { LensDashboard } from './LensDashboard'
import type { LensThemeMode } from './runtime'
import './styles.css'

/**
 * The dashboard header, which is three rows with one job each: what page this
 * is and what can be done with it, what slice is on screen, and what is
 * currently narrowing it.
 *
 * These stories exist because the header used to be two columns of identically
 * weighted controls — a period tray, a comparison box, a freshness stamp, a
 * recompute button, a labelled copy button and an export menu, eight bordered
 * rectangles on one baseline. What is covered here is the separation that
 * replaced it: state wears the chip weight and lives in the scope sentence,
 * actions keep the secondary weight and stay in the action row.
 */

const storyToday = { year: 2026, month: 7, day: 22 }

const periodFilter: Filter = {
  id: 'period',
  kind: 'period',
  label: 'Period',
  period: {
    startParam: 'ActualRangeStart',
    endParam: 'ActualRangeEnd',
    value: { start: '2026-01-01', end: '2026-07-22' },
    allowEmpty: true,
    presets: [
      { id: 'year-2024', label: '2024', value: { start: '2024-01-01', end: '2024-12-31' } },
      { id: 'year-2025', label: '2025', value: { start: '2025-01-01', end: '2025-12-31' } },
      { id: 'year-2026', label: '2026', value: { start: '2026-01-01', end: '2026-12-31' } },
    ],
  },
}

const compareFilter: Filter = {
  id: 'compare',
  kind: 'compare',
  label: 'Compare with',
  compare: {
    modeParam: 'compare',
    startParam: 'compare_start',
    endParam: 'compare_end',
    compareTo: 'period',
    value: { mode: 'year_ago' },
  },
}

const regionFacet: Filter = {
  id: 'facet-region',
  kind: 'facet',
  label: 'Region',
  facet: {
    dimension: 'region',
    optionsEndpoint: '/lens/facet-options?_facet=region',
    searchParam: '_facet_search',
    selections: [
      { label: 'Tashkent city', removeUrl: '/reports/sales?_f=product%3Aosago' },
      { label: 'Samarkand region', removeUrl: '/reports/sales?_f=product%3Aosago' },
    ],
    clearUrl: '/reports/sales',
  },
}

function statPanel(id: string, title: string): Panel {
  return {
    id,
    kind: 'stat',
    semantics: 'series',
    title,
    frame: `${id}:frame`,
    encoding: { label: 'label', value: 'value' },
    format: { value: { kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' } },
    terminal: true,
    actions: [],
  }
}

interface SceneOptions {
  theme?: LensThemeMode
  filters?: Array<Filter>
  subtitle?: string
  /** Omit to drop the recompute and export affordances, as a read-only board. */
  endpoints?: DashboardDocument['endpoints']
  width?: number
}

function headerDocument(options: SceneOptions): DashboardDocument {
  const panels = [
    statPanel('loss-ratio', 'Loss ratio'),
    statPanel('expense-ratio', 'Expense ratio'),
    statPanel('combined-ratio', 'Combined ratio'),
  ]
  return {
    version: '1.0.0',
    snapshotId: 'story-header',
    meta: {
      dashboardId: 'header',
      title: 'Profitability and combined ratio',
      generatedAt: '2026-07-22T00:00:00Z',
      locale: 'en',
    },
    header: { title: 'Profitability and combined ratio', subtitle: options.subtitle },
    layout: { rows: [{ heading: 'Key ratios', panels: panels.map((panel) => ({ panelId: panel.id, span: 4 })) }] },
    panels,
    frames: Object.fromEntries(panels.map((panel, index) => [panel.frame, {
      columns: [{ name: 'label', type: 'string' as const }, { name: 'value', type: 'number' as const }],
      rows: [[panel.title, 38.4 + index * 2.7]],
    }])),
    drill: { inlineDepth: 0, edges: {} },
    perspectives: [],
    filters: options.filters ?? [periodFilter, compareFilter],
    endpoints: options.endpoints ?? { panel: '/lens/panel', export: '/lens/export' },
    i18n: {},
    theme: { palette: {}, series: {} },
  }
}

function HeaderScene(options: SceneOptions) {
  const scene = (
    <LensDashboard
      filterToday={storyToday}
      initialDocument={headerDocument(options)}
      theme={options.theme ?? 'light'}
    />
  )
  return options.width ? <div style={{ width: options.width }}>{scene}</div> : scene
}

/**
 * The whole vocabulary at once: the scope sentence carries the period with its
 * two step arrows, the comparison and the facet trigger, and the action row
 * carries the freshness control, copy link and export. Three bordered controls
 * where there were eight.
 */
export const HeaderLight: Story = () => (
  <HeaderScene filters={[periodFilter, compareFilter, regionFacet]} theme="light" />
)
HeaderLight.storyName = 'Header light'

export const HeaderDark: Story = () => (
  <HeaderScene filters={[periodFilter, compareFilter, regionFacet]} theme="dark" />
)
HeaderDark.storyName = 'Header dark'

/**
 * A dashboard with no panel endpoint cannot recompute and no export endpoint
 * cannot export, so the freshness reading renders as a bare stamp on the same
 * baseline and the row keeps its geometry rather than collapsing.
 */
export const HeaderReadOnly: Story = () => (
  <HeaderScene endpoints={{}} filters={[periodFilter]} theme="light" />
)
HeaderReadOnly.storyName = 'Header read only'

/**
 * The producer's own line, which states what no control can — here a data
 * cut-off. It is the tail of the scope sentence, not a second heading.
 */
export const HeaderWithSubtitle: Story = () => (
  <HeaderScene subtitle="Data as of 22 July 2026" theme="light" />
)
HeaderWithSubtitle.storyName = 'Header with subtitle'

/**
 * Under the 1100px threshold the action row drops below the title instead of
 * squeezing it, and the scope sentence wraps clause by clause.
 */
export const HeaderNarrow: Story = () => (
  <HeaderScene filters={[periodFilter, compareFilter, regionFacet]} theme="light" width={720} />
)
HeaderNarrow.storyName = 'Header narrow'
