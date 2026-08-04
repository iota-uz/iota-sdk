import type { Story } from '@ladle/react'
import { useCallback, useEffect, useRef, useState } from 'react'
import type { DashboardDocument, Filter, Panel } from './contract'
import { Calendar } from './controls'
import { LensDashboard } from './LensDashboard'
import type { LensThemeMode } from './runtime'
import './styles.css'

const storyToday = { year: 2026, month: 7, day: 22 }

const identityTranslate = (_key: string, fallback: string, vars?: Readonly<Record<string, string | number>>) => {
  if (!vars) return fallback
  return fallback.replace(/\{(\w+)\}/g, (match, name: string) => (name in vars ? String(vars[name]) : match))
}

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

function statPanel(id: string, title: string): Panel {
  return {
    id, kind: 'stat', semantics: 'series', title, frame: `${id}:frame`,
    encoding: { label: 'label', value: 'value' },
    format: { value: { kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' } },
    terminal: true,
    actions: [],
  }
}

function filteredDocument(locale = 'en'): DashboardDocument {
  const panels = [
    statPanel('loss-ratio', 'Loss ratio'),
    statPanel('expense-ratio', 'Expense ratio'),
    statPanel('combined-ratio', 'Combined ratio'),
  ]
  return {
    version: '1.0.0',
    snapshotId: 'story-filters',
    meta: { dashboardId: 'filters', title: 'Profitability', generatedAt: '2026-07-22T00:00:00Z', locale },
    layout: { rows: [{ heading: 'Key ratios', panels: panels.map((panel) => ({ panelId: panel.id, span: 4 })) }] },
    panels,
    frames: Object.fromEntries(panels.map((panel, index) => [panel.frame, {
      columns: [{ name: 'label', type: 'string' as const }, { name: 'value', type: 'number' as const }],
      rows: [[panel.title, 38.4 + index * 2.7]],
    }])),
    drill: { inlineDepth: 0, edges: {} },
    perspectives: [],
    filters: [periodFilter],
    endpoints: {},
    i18n: {},
    theme: { palette: {}, series: {} },
  }
}

function DashboardScene({ theme, period = periodFilter }: { theme: LensThemeMode; period?: Filter }) {
  const base = filteredDocument()
  return (
    <LensDashboard
      filterToday={storyToday}
      initialDocument={{ ...base, filters: [period] }}
      theme={theme}
    />
  )
}

/**
 * A period that came from a declared preset. It reads exactly like one a reader
 * drew by hand: the trigger prints the resolved range either way, and both step
 * arrows are live because a whole calendar year has a year on each side of it.
 */
const presetMatchedFilter: Filter = {
  ...periodFilter,
  period: { ...periodFilter.period!, value: { start: '2025-01-01', end: '2025-12-31' } },
}

export const DashboardFilterPresetApplied: Story = () => (
  <DashboardScene period={presetMatchedFilter} theme="light" />
)
DashboardFilterPresetApplied.storyName = 'Dashboard filter preset applied'

export const DashboardFilterLight: Story = () => <DashboardScene theme="light" />
DashboardFilterLight.storyName = 'Dashboard filter light'

export const DashboardFilterDark: Story = () => <DashboardScene theme="dark" />
DashboardFilterDark.storyName = 'Dashboard filter dark'

const regionFacet: Filter = {
  id: 'facet-region',
  kind: 'facet',
  label: 'Region',
  facet: {
    dimension: 'region',
    optionsEndpoint: '/lens/facet-options?_facet=region',
    searchParam: '_facet_search',
    selections: [
      { label: 'Tashkent city', removeUrl: '/reports/sales?_f=region%3Asamarkand&_f=product%3Aosago' },
      { label: 'Samarkand region', removeUrl: '/reports/sales?_f=region%3Atashkent&_f=product%3Aosago' },
    ],
    clearUrl: '/reports/sales',
  },
}

export const DashboardFacetActive: Story = () => {
  const base = filteredDocument()
  return (
    <div style={{ width: 960 }}>
      <LensDashboard
        filterToday={storyToday}
        initialDocument={{ ...base, filters: [periodFilter, regionFacet] }}
        theme="light"
      />
    </div>
  )
}
DashboardFacetActive.storyName = 'Dashboard facet active'

const productFacet: Filter = {
  id: 'facet-product',
  kind: 'facet',
  label: 'Product',
  facet: {
    dimension: 'product',
    optionsEndpoint: '/lens/facet-options?_facet=product',
    searchParam: '_facet_search',
    selections: [
      { label: '18-11. Insurance of persons travelling abroad', removeUrl: '/reports/sales' },
    ],
    clearUrl: '/reports/sales',
  },
}

const channelFacet: Filter = {
  id: 'facet-channel',
  kind: 'facet',
  label: 'Contract source',
  facet: {
    dimension: 'channel',
    optionsEndpoint: '/lens/facet-options?_facet=channel',
    searchParam: '_facet_search',
    selections: [],
    clearUrl: '/reports/sales',
  },
}

const genderFacet: Filter = {
  id: 'facet-gender',
  kind: 'facet',
  label: 'Gender',
  facet: {
    dimension: 'gender',
    optionsEndpoint: '/lens/facet-options?_facet=gender',
    searchParam: '_facet_search',
    selections: [],
    clearUrl: '/reports/sales',
  },
}

function FacetOptionsScene() {
  const [ready, setReady] = useState(false)
  useEffect(() => {
    const original = globalThis.fetch
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const target = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      if (target.startsWith('/lens/facet-options')) {
        return Promise.resolve(new Response(JSON.stringify({
          applyUrl: '/reports/sales?_f=product%3Aosago',
          options: [
            { label: 'Tashkent city', value: 'tashkent', count: 275, selected: true, toggleUrl: '/reports/sales' },
            { label: 'Samarkand region', value: 'samarkand', count: 256, toggleUrl: '/reports/sales' },
            { label: 'Fergana region', value: 'fergana', count: 229, toggleUrl: '/reports/sales' },
            { label: 'Andijan region', value: 'andijan', count: 205, toggleUrl: '/reports/sales' },
            { label: 'Bukhara region', value: 'bukhara', count: 182, toggleUrl: '/reports/sales' },
            { label: 'Khorezm region', value: 'khorezm', count: 151, toggleUrl: '/reports/sales' },
          ],
        }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      }
      return original(input, init)
    }) as typeof fetch
    setReady(true)
    return () => { globalThis.fetch = original }
  }, [])
  if (!ready) return null
  const base = filteredDocument()
  return (
    <AutoClick selector=".lens-facet-trigger">
      <div style={{ width: 960 }}>
        <LensDashboard
          filterToday={storyToday}
          initialDocument={{ ...base, filters: [regionFacet] }}
          theme="light"
        />
      </div>
    </AutoClick>
  )
}

/** Historical staged multi-select: checkboxes, count bars, and one Apply. */
export const FacetOptionsOpen: Story = () => <FacetOptionsScene />
FacetOptionsOpen.storyName = 'Facet options open'

function FiltersMenuScene() {
  const [ready, setReady] = useState(false)
  useEffect(() => {
    const original = globalThis.fetch
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const target = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      if (target.startsWith('/lens/facet-options')) {
        return Promise.resolve(new Response(JSON.stringify({
          applyUrl: '/reports/sales',
          options: [
            { label: '18-11. Insurance of persons travelling abroad the Republic of Uzbekistan', value: 'travel', count: 1644, selected: true, toggleUrl: '/reports/sales' },
            { label: '11-01. Compulsory motor third-party liability', value: 'osago', count: 1204, toggleUrl: '/reports/sales' },
            { label: '13-02. Property of legal entities', value: 'property', count: 812, toggleUrl: '/reports/sales' },
            { label: '16-04. Cargo in transit', value: 'cargo', count: 455, toggleUrl: '/reports/sales' },
            { label: '12-03. Accident and illness', value: 'accident', count: 301, toggleUrl: '/reports/sales' },
            { label: '15-08. Contractor all risks', value: 'car', count: 96, toggleUrl: '/reports/sales' },
          ],
        }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      }
      return original(input, init)
    }) as typeof fetch
    setReady(true)
    return () => { globalThis.fetch = original }
  }, [])
  if (!ready) return null
  const base = filteredDocument()
  return (
    <AutoClick selector=".lens-filter-menu .lens-facet-trigger">
      <div style={{ width: 1100 }}>
        <LensDashboard
          filterToday={storyToday}
          initialDocument={{
            ...base,
            activeFilters: [{
              dimension: 'product',
              value: 'travel',
              label: '18-11. Insurance of persons travelling abroad',
              removeUrl: '/reports/sales',
            }],
            filters: [periodFilter, productFacet, regionFacet, channelFacet, genderFacet],
            resetFiltersUrl: '/reports/sales',
          }}
          theme="light"
        />
      </div>
    </AutoClick>
  )
}

/**
 * The headline composition: every facet behind one «Filters (n)» trigger, the
 * dimensions in a rail beside their options, applied selections as chips on a
 * row of their own. This replaced nine sibling dropdowns wrapping into three
 * ragged rows above the first number.
 */
export const FiltersMenuOpen: Story = () => <FiltersMenuScene />
FiltersMenuOpen.storyName = 'Filters menu open'

function RefetchErrorScene() {
  const requests = useRef(0)
  const fetcher = useCallback<typeof fetch>(() => {
    requests.current += 1
    if (requests.current > 1) {
      return Promise.resolve(new Response(JSON.stringify({ message: 'document refetch failed' }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' },
      }))
    }
    return Promise.resolve(new Response(JSON.stringify(filteredDocument()), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))
  }, [])
  return (
    <LensDashboard
      fetcher={fetcher}
      filterToday={storyToday}
      src="/lens/document"
      theme="light"
    />
  )
}

export const RefetchError: Story = () => <RefetchErrorScene />
RefetchError.storyName = 'Refetch error'

function clickWhenReady(find: () => HTMLElement | null | undefined): () => void {
  let cancelled = false
  let attempts = 0
  const click = () => {
    if (cancelled) return
    const element = find()
    if (element) {
      element.click()
      return
    }
    if (attempts++ < 60) window.requestAnimationFrame(click)
  }
  void window.document.fonts.ready.then(() => {
    window.requestAnimationFrame(() => window.requestAnimationFrame(click))
  })
  return () => { cancelled = true }
}

/** Clicks the period trigger once mounted so the popover is the subject. */
function AutoOpen({ children }: { children: React.ReactNode }) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => clickWhenReady(
    () => ref.current?.querySelector<HTMLElement>('.lens-filter-trigger'),
  ), [])
  return <div ref={ref}>{children}</div>
}

export const PopoverOpenLight: Story = () => (
  <AutoOpen><DashboardScene theme="light" /></AutoOpen>
)
PopoverOpenLight.storyName = 'Popover open light'

export const PopoverOpenDark: Story = () => (
  <AutoOpen><DashboardScene theme="dark" /></AutoOpen>
)
PopoverOpenDark.storyName = 'Popover open dark'

function CalendarCard({ children, theme = 'light' }: { children: React.ReactNode; theme?: LensThemeMode }) {
  return (
    <div className="lens-root" data-theme={theme}>
      <div className="lens-filter-popover" style={{ position: 'static', width: 496 }}>
        <div className="lens-filter-popover-main">
          {children}
        </div>
      </div>
    </div>
  )
}

const committedRange = {
  start: { year: 2026, month: 7, day: 3 },
  end: { year: 2026, month: 7, day: 18 },
}

export const CalendarLight: Story = () => (
  <CalendarCard>
    <Calendar
      draft={committedRange}
      locale="en"
      onPick={() => undefined}
      today={storyToday}
      translate={identityTranslate}
    />
  </CalendarCard>
)
CalendarLight.storyName = 'Calendar light'

export const CalendarDark: Story = () => (
  <CalendarCard theme="dark">
    <Calendar
      draft={committedRange}
      locale="en"
      onPick={() => undefined}
      today={storyToday}
      translate={identityTranslate}
    />
  </CalendarCard>
)
CalendarDark.storyName = 'Calendar dark'

/**
 * A pending range anchor: the VR keyframe test hovers a later day to capture
 * the live preview wash between anchor and pointer.
 */
export const CalendarRangePending: Story = () => (
  <CalendarCard>
    <Calendar
      draft={{ start: { year: 2026, month: 7, day: 3 } }}
      locale="en"
      onPick={() => undefined}
      today={storyToday}
      translate={identityTranslate}
    />
  </CalendarCard>
)
CalendarRangePending.storyName = 'Calendar range pending'

/** Clicks a selector once mounted, so a click-only state can be a story. */
function AutoClick({ children, selector }: { children: React.ReactNode; selector: string }) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => clickWhenReady(
    () => ref.current?.querySelector<HTMLElement>(selector),
  ), [selector])
  return <div ref={ref}>{children}</div>
}

/** The month panel behind the heading: the only way to travel by year. */
export const CalendarMonthPanel: Story = () => (
  <AutoClick selector=".lens-calendar-month">
    <CalendarCard>
      <Calendar
        draft={committedRange}
        locale="en"
        onPick={() => undefined}
        today={storyToday}
        translate={identityTranslate}
      />
    </CalendarCard>
  </AutoClick>
)
CalendarMonthPanel.storyName = 'Calendar month panel'

/** All four product locales: month names, weekday rows, first day of week. */
export const CalendarLocales: Story = () => (
  <div className="lens-root" style={{ display: 'flex', flexWrap: 'wrap', gap: 16 }}>
    {(['en-US', 'ru', 'uz', 'uz-Cyrl'] as const).map((locale) => (
      <div className="lens-filter-popover" key={locale} style={{ position: 'static', width: 496 }}>
        <div className="lens-filter-popover-main">
          <Calendar
            draft={committedRange}
            locale={locale}
            onPick={() => undefined}
            today={storyToday}
            translate={identityTranslate}
          />
        </div>
      </div>
    ))}
  </div>
)
CalendarLocales.storyName = 'Calendar locales'

const compareFilter: Filter = {
  id: 'compare',
  kind: 'compare',
  label: 'Compare with',
  compare: {
    modeParam: 'compare',
    startParam: 'compare_start',
    endParam: 'compare_end',
    compareTo: 'period',
    value: { mode: 'previous_period' },
  },
}

function ComparisonScene({ compare }: { compare: Filter }) {
  const base = filteredDocument()
  return (
    <AutoClick selector=".lens-compare-trigger">
      <div style={{ width: 960 }}>
        <LensDashboard
          filterToday={storyToday}
          initialDocument={{ ...base, filters: [periodFilter, compare] }}
          theme="light"
        />
      </div>
    </AutoClick>
  )
}

/**
 * The comparison control open. It is the facet control's popover primitive, not
 * a native `<select>`: same trigger box, same option rows, same ring — which is
 * the whole point of the story, since the header row is where a stray control
 * treatment is most visible.
 */
export const ComparisonMenuOpen: Story = () => <ComparisonScene compare={compareFilter} />
ComparisonMenuOpen.storyName = 'Comparison menu open'

/**
 * The custom interval: its two date fields live inside the popover, under the
 * mode that needs them, with one Apply. They used to sit in the header row,
 * appearing and disappearing beside the filters as the mode changed.
 */
export const ComparisonCustomInterval: Story = () => (
  <ComparisonScene
    compare={{
      ...compareFilter,
      compare: { ...compareFilter.compare!, value: { mode: 'custom', start: '2025-01-01', end: '2025-06-30' } },
    }}
  />
)
ComparisonCustomInterval.storyName = 'Comparison custom interval'

const granularityFilter: Filter = {
  id: 'grain',
  kind: 'segmented',
  label: 'Periodicity',
  segmented: {
    param: 'PeriodGrain',
    value: 'quarter',
    options: [
      { value: 'year', label: 'By year' },
      { value: 'quarter', label: 'By quarter' },
    ],
  },
}

/* Fluid, not a fixed-width box: the point of the story is that the three
   controls keep one order and one baseline as the header row wraps. */
function GranularityScene({ theme }: { theme: LensThemeMode }) {
  const base = filteredDocument()
  return (
    <LensDashboard
      filterToday={storyToday}
      initialDocument={{ ...base, filters: [periodFilter, granularityFilter, compareFilter] }}
      theme={theme}
    />
  )
}

/**
 * The segmented filter in the row it was built for: a closed choice standing
 * between the period it grains and the comparison it applies to. It reuses the
 * period control's tray and chips, so the header carries one recessed-track
 * idiom rather than a second control that merely resembles it — and the choice
 * it holds is URL state, not the renderer-local selection a Tabs group keeps.
 */
export const GranularitySegmented: Story = () => <GranularityScene theme="light" />
GranularitySegmented.storyName = 'Granularity segmented'

export const GranularitySegmentedDark: Story = () => <GranularityScene theme="dark" />
GranularitySegmentedDark.storyName = 'Granularity segmented dark'
