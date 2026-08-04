import type { Story } from '@ladle/react'
import { useEffect, useRef } from 'react'
import focusFixture from '../fixtures/focus.json'
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
    terminal: action ? undefined : true,
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
    groups: [{ id: 'ratios', kind: 'metrics', label: 'Earned basis', layout: 'columns', span: 12 }],
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

// The loaded document opts into the wide drawer (~1080px) through its own
// `drawer` header — the variant cross-metric detail views use to host a full
// focus canvas. The slide-in chrome is otherwise unchanged.
const wideDetail: DashboardDocument = {
  ...detail,
  snapshotId: 'story-loss-ratio-detail-wide',
  drawer: { eyebrow: 'Ratio breakdown', title: 'Loss ratio', caption: 'FY 2026 · earned basis', size: 'wide' },
}

// A wide drawer whose subject is a focus-canvas host: the family's donut with
// its root-level lenses surfaced immediately. Derived from the focus fixture
// so the drill wiring stays valid; the root level gains a two-lens perspective
// set (its own ref, siblings derived from the branch as production emits) and
// the document opts into the wide slab. This exercises the drawer-scoped canvas
// treatment (full width, roomy body) and the root lens selector.
const focusDrawer = (() => {
  const raw = JSON.parse(JSON.stringify(focusFixture)) as {
    snapshotId: string
    drawer?: unknown
    perspectives: Array<Record<string, unknown>>
    drill: { edges: Record<string, { perspectives: Array<{ id: string }> }> }
  }
  raw.snapshotId = 'story-focus-canvas-drawer'
  raw.drawer = { eyebrow: 'Family breakdown', title: 'Earned premium', caption: 'FY 2026 · earned basis', size: 'wide' }
  raw.perspectives.push(
    { id: 'premium/by-market', explorerId: 'premium', branchKey: 'premium', key: 'by-market', label: 'By market', semantics: 'partition', root: 'premium' },
    { id: 'premium/by-channel', explorerId: 'premium', branchKey: 'premium', key: 'by-channel', label: 'By channel', semantics: 'series', root: 'premium/motor/trend' },
  )
  raw.drill.edges['premium']!.perspectives = [{ id: 'premium/by-market' }]
  return raw as unknown as DashboardDocument
})()

type DrawerState = 'ready' | 'loading' | 'error' | 'long' | 'wide' | 'focusCanvas'

function drawerFetcher(state: DrawerState): typeof fetch {
  return () => {
    if (state === 'loading') return new Promise<Response>(() => undefined)
    if (state === 'error') return Promise.resolve(new Response('failed', { status: 503 }))
    const body = state === 'long'
      ? longDetail
      : state === 'wide'
        ? wideDetail
        : state === 'focusCanvas' ? focusDrawer : detail
    return Promise.resolve(new Response(JSON.stringify(body), {
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

/**
 * The drill drawer stacks on top of an expanded panel. The panel expands into a
 * body-level portal at a huge z-index; the drawer opened from inside it portals
 * to a second body-level host one rung higher, so it paints over the fullscreen
 * view instead of hiding behind its backdrop. This is the regression the portal
 * move fixes: expand first, then open the drill from the link inside the overlay.
 */
function StackedScene({ theme }: { theme: LensThemeMode }) {
  const host = useRef<HTMLDivElement>(null)
  useEffect(() => {
    let cancelled = false
    let attempts = 0
    const openDrill = () => {
      if (cancelled) return
      // The overlay is a body-level portal, so the drill link is reached from
      // the document, not the story host.
      const link = globalThis.document.querySelector<HTMLAnchorElement>('.lens-panel-overlay .lens-card-link')
      if (link) { link.click(); return }
      if (attempts++ < 30) globalThis.requestAnimationFrame(openDrill)
    }
    const expand = () => {
      if (cancelled) return
      const button = host.current?.querySelector<HTMLButtonElement>('button[aria-label="Expand panel"]')
      if (button) { button.click(); globalThis.requestAnimationFrame(openDrill); return }
      if (attempts++ < 30) globalThis.requestAnimationFrame(expand)
    }
    globalThis.requestAnimationFrame(expand)
    return () => { cancelled = true }
  }, [])
  return (
    <div ref={host}>
      <LensDashboard initialDocument={dashboard} fetcher={drawerFetcher('ready')} theme={theme} />
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

export const OpenWide: Story = () => <DrawerScene open state="wide" theme="light" />
OpenWide.storyName = 'Open wide'

export const OpenWideFocusCanvas: Story = () => <DrawerScene open state="focusCanvas" theme="light" />
OpenWideFocusCanvas.storyName = 'Open wide focus canvas'

export const OpenOverExpandedPanel: Story = () => <StackedScene theme="light" />
OpenOverExpandedPanel.storyName = 'Open over expanded panel'

export const Loading: Story = () => <DrawerScene open state="loading" theme="light" />
Loading.storyName = 'Loading'

export const Error: Story = () => <DrawerScene open state="error" theme="light" />
Error.storyName = 'Error'

export const LongDocumentScrolls: Story = () => <DrawerScene open state="long" theme="light" />
LongDocumentScrolls.storyName = 'Long document scrolls'
