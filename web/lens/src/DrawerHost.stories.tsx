import type { Story } from '@ladle/react'
import { useEffect, useRef } from 'react'
import type { Action, DashboardDocument, Panel } from './contract'
import { LensDashboard } from './LensDashboard'
import type { LensThemeMode } from './runtime'
import './styles.css'

const drawerURL = '/stories/drill/loss-ratio/lens/document?token=story'
const openDrawer: Action = { kind: 'open_drawer', method: 'GET', urlTemplate: drawerURL, params: [], payload: {} }

function statPanel(id: string, title: string, action?: Action): Panel {
  return {
    id, kind: 'stat', semantics: 'series', title, frame: `${id}:frame`,
    encoding: { label: 'label', value: 'value' },
    format: { value: { kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' } },
    actions: action ? [action] : [],
  }
}

function documentWith(title: string, panels: Panel[], rows: DashboardDocument['layout']['rows']): DashboardDocument {
  return {
    version: '1.0.0', snapshotId: `story-${title}`, meta: {
      dashboardId: title.toLowerCase().replaceAll(' ', '-'), title, generatedAt: '2026-07-22T00:00:00Z', locale: 'en',
    },
    layout: { rows }, panels,
    frames: Object.fromEntries(panels.map((panel, index) => [panel.frame, {
      columns: [{ name: 'label', type: 'string' as const }, { name: 'value', type: 'number' as const }],
      rows: [[panel.title, 38.4 + index * 2.7]],
    }])),
    drill: { inlineDepth: 0, edges: {} }, perspectives: [], endpoints: {},
    i18n: { 'drawer.label': 'Drill details', 'drawer.eyebrow': 'Ratio breakdown', 'drawer.close': 'Close details' },
    theme: { palette: {}, series: {} },
  }
}

const dashboardPanels = [
  statPanel('loss-ratio', 'Loss ratio', openDrawer),
  statPanel('expense-ratio', 'Expense ratio'),
  statPanel('combined-ratio', 'Combined ratio'),
]
const dashboard = documentWith('Profitability overview', dashboardPanels, [{
  heading: 'Key ratios',
  panels: dashboardPanels.map((panel) => ({
    panelId: panel.id, span: 4,
    group: { id: 'ratios', kind: 'metrics', label: 'Earned basis', layout: 'columns', span: 12 },
  })),
}])

const detailPanels = [
  statPanel('claims', 'Claims incurred'),
  statPanel('premium', 'Earned premium'),
  statPanel('result', 'Loss ratio'),
]
const detail = documentWith('Loss ratio detail', detailPanels, [
  { heading: 'Result', panels: [{ panelId: 'result', span: 12 }] },
  { heading: 'Components', panels: detailPanels.slice(0, 2).map((panel) => ({ panelId: panel.id, span: 6 })) },
])

const longPanels = Array.from({ length: 18 }, (_, index) => statPanel(
  `component-${index + 1}`,
  `Portfolio component ${index + 1}`,
))
const longDetail = documentWith('Loss ratio by portfolio', longPanels, Array.from({ length: 9 }, (_, row) => ({
  heading: `Portfolio group ${row + 1}`,
  panels: longPanels.slice(row * 2, row * 2 + 2).map((panel) => ({ panelId: panel.id, span: 6 })),
})))

type DrawerState = 'ready' | 'loading' | 'error' | 'long'

function drawerFetcher(state: DrawerState): typeof fetch {
  return () => {
    if (state === 'loading') return new Promise<Response>(() => undefined)
    if (state === 'error') return Promise.resolve(new Response('failed', { status: 503 }))
    return Promise.resolve(new Response(JSON.stringify(state === 'long' ? longDetail : detail), {
      status: 200, headers: { 'Content-Type': 'application/json' },
    }))
  }
}

function DrawerScene({ open, theme, state = 'ready' }: { open: boolean; theme: LensThemeMode; state?: DrawerState }) {
  const host = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    host.current?.querySelector<HTMLAnchorElement>('.lens-card-link')?.click()
  }, [open])
  return (
    <div ref={host}>
      <LensDashboard initialDocument={dashboard} fetcher={drawerFetcher(state)} theme={theme} />
    </div>
  )
}

export const ClosedLight: Story = () => <DrawerScene open={false} theme="light" />
ClosedLight.storyName = 'Closed light'

export const ClosedDark: Story = () => <DrawerScene open={false} theme="dark" />
ClosedDark.storyName = 'Closed dark'

export const OpenLight: Story = () => <DrawerScene open theme="light" />
OpenLight.storyName = 'Open light'

export const OpenDark: Story = () => <DrawerScene open theme="dark" />
OpenDark.storyName = 'Open dark'

export const Loading: Story = () => <DrawerScene open state="loading" theme="light" />
Loading.storyName = 'Loading'

export const Error: Story = () => <DrawerScene open state="error" theme="light" />
Error.storyName = 'Error'

export const LongDocumentScrolls: Story = () => <DrawerScene open state="long" theme="light" />
LongDocumentScrolls.storyName = 'Long document scrolls'
