import type { Story } from '@ladle/react'
import { useCallback, useEffect, useRef } from 'react'
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

function DashboardScene({ theme }: { theme: LensThemeMode }) {
  return <LensDashboard filterToday={storyToday} initialDocument={filteredDocument()} theme={theme} />
}

export const DashboardFilterLight: Story = () => <DashboardScene theme="light" />
DashboardFilterLight.storyName = 'Dashboard filter light'

export const DashboardFilterDark: Story = () => <DashboardScene theme="dark" />
DashboardFilterDark.storyName = 'Dashboard filter dark'

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

/** Clicks the period trigger once mounted so the popover is the subject. */
function AutoOpen({ children }: { children: React.ReactNode }) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    ref.current?.querySelector<HTMLElement>('.lens-filter-trigger')?.click()
  }, [])
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
      <div className="lens-filter-popover" style={{ position: 'static', width: 316 }}>
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

/** All four product locales: month names, weekday rows, first day of week. */
export const CalendarLocales: Story = () => (
  <div className="lens-root" style={{ display: 'flex', flexWrap: 'wrap', gap: 16 }}>
    {(['en-US', 'ru', 'uz', 'uz-Cyrl'] as const).map((locale) => (
      <div className="lens-filter-popover" key={locale} style={{ position: 'static', width: 316 }}>
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
