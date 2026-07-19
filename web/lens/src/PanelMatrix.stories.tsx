import type { Story } from '@ladle/react'
import { useEffect } from 'react'
import type { DashboardDocument, Frame, Panel, PanelKind } from './contract'
import type { ChartAdapter, ChartInput } from './charts/adapter'
import { BarPanel, LinePanel, PiePanel, StatPanel } from './panels'
import { DashboardRuntimeProvider, DocumentProvider, useDrill } from './runtime'
import './styles.css'

type PanelState = 'loading' | 'empty' | 'error' | 'stale' | 'data'
type V1Kind = Extract<PanelKind, 'stat' | 'pie' | 'donut' | 'bar' | 'hbar' | 'line' | 'area'>

const kinds: V1Kind[] = ['stat', 'pie', 'donut', 'bar', 'hbar', 'line', 'area']
const states: PanelState[] = ['loading', 'empty', 'error', 'stale', 'data']

const chartFrame: Frame = {
  columns: [
    { name: 'id', type: 'string' },
    { name: 'label', type: 'string' },
    { name: 'period', type: 'time' },
    { name: 'series', type: 'string' },
    { name: 'value', type: 'number' },
  ],
  rows: [
    ['root/north', 'North', '2026-04-01T00:00:00Z', 'Actual', 64],
    ['root/south', 'South', '2026-05-01T00:00:00Z', 'Actual', 41],
    ['root/east', 'East', '2026-06-01T00:00:00Z', 'Plan', 27],
  ],
}

const statFrame: Frame = {
  columns: [
    { name: 'label', type: 'string' },
    { name: 'value', type: 'number' },
    { name: 'delta', type: 'number' },
  ],
  rows: [['Net revenue', 12486000, 7.4]],
}

function storyPanel(kind: V1Kind): Panel {
  const chart = kind !== 'stat'
  return {
    id: `${kind}-panel`,
    kind,
    title: chart ? `${kind} performance` : 'Revenue this quarter',
    semantics: kind === 'pie' || kind === 'donut' ? 'partition' : 'series',
    frame: `${kind}-frame`,
    encoding: chart
      ? { id: 'id', label: 'label', category: 'period', series: 'series', value: 'value' }
      : { label: 'label', value: 'value', final: 'delta' },
    format: chart
      ? { value: { kind: 'number', minorUnits: false, precision: 0 } }
      : {
          value: { kind: 'money', currency: 'USD', minorUnits: true, precision: 0 },
          delta: { kind: 'percent', minorUnits: false, precision: 1 },
        },
    drillRoot: 'root',
    actions: [],
  }
}

function storyDocument(kind: V1Kind, state: PanelState): DashboardDocument {
  const panel = storyPanel(kind)
  const sourceFrame = kind === 'stat' ? statFrame : chartFrame
  const includeFrame = state === 'data' || state === 'stale' || state === 'empty'
  return {
    version: '1.0.0',
    snapshotId: `${kind}-${state}`,
    meta: { dashboardId: 'panel-matrix', title: 'Panel matrix', generatedAt: '2026-07-19T00:00:00Z', locale: 'en' },
    layout: { rows: [{ panels: [{ panelId: panel.id, span: 12 }] }] },
    panels: [panel],
    frames: includeFrame ? { [panel.frame]: state === 'empty' ? { ...sourceFrame, rows: [] } : sourceFrame } : {},
    drill: {
      inlineDepth: 0,
      edges: {
        root: {
          path: ['root'],
          label: 'All regions',
          children: [
            { key: 'root/north', path: ['root', 'root/north'], label: 'North', target: 'north' },
            { key: 'root/south', path: ['root', 'root/south'], label: 'South', target: 'south' },
            { key: 'root/east', path: ['root', 'root/east'], label: 'East', target: 'east' },
          ],
          perspectives: [],
        },
        north: { path: ['root', 'root/north'], label: 'North', children: [], perspectives: [] },
        south: { path: ['root', 'root/south'], label: 'South', children: [], perspectives: [] },
        east: { path: ['root', 'root/east'], label: 'East', children: [], perspectives: [] },
      },
    },
    perspectives: [],
    endpoints: state === 'loading' || state === 'stale' || state === 'error' ? { query: '/story/query' } : {},
    i18n: {},
    theme: {
      palette: { accent: '#2563eb', muted: '#94a3b8' },
      series: { Actual: '#2563eb', Plan: '#8b5cf6' },
    },
  }
}

function numberValue(input: ChartInput, row: Array<unknown>): number {
  const field = input.encoding.value
  const index = field ? input.frame.columns.findIndex((column) => column.name === field) : -1
  const value = index >= 0 ? row[index] : undefined
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

const storyChartAdapter: ChartAdapter = {
  mount(el, initialInput, events) {
    const hidden = new Set<string>()
    const render = (input: ChartInput) => {
      const idIndex = input.encoding.id ? input.frame.columns.findIndex((column) => column.name === input.encoding.id) : -1
      const labelIndex = input.encoding.label ? input.frame.columns.findIndex((column) => column.name === input.encoding.label) : -1
      const visible = input.frame.rows.filter((row) => {
        const key = idIndex >= 0 ? row[idIndex] : undefined
        return typeof key !== 'string' || !hidden.has(key)
      })
      const total = visible.reduce((sum, row) => sum + numberValue(input, row), 0)
      const visual = document.createElement('div')
      visual.className = `lens-fake-chart lens-fake-chart-${input.kind}`
      visual.setAttribute('role', 'img')
      visual.setAttribute('aria-label', `${input.kind} chart with ${visible.length} visible values`)

      const plot = document.createElement('div')
      plot.className = 'lens-fake-chart-plot'
      for (const row of visible) {
        const key = idIndex >= 0 ? row[idIndex] : undefined
        const label = labelIndex >= 0 ? row[labelIndex] : key
        const mark = document.createElement('button')
        mark.type = 'button'
        mark.className = 'lens-fake-chart-mark'
        mark.style.setProperty('--lens-mark-size', `${Math.max(18, numberValue(input, row))}%`)
        mark.textContent = typeof label === 'string' ? label : 'Value'
        if (typeof key === 'string') {
          mark.onclick = () => events.onSelect(key)
          mark.onpointerenter = () => events.onHover(key)
          mark.onpointerleave = () => events.onHover(null)
        }
        plot.append(mark)
      }

      const footer = document.createElement('div')
      footer.className = 'lens-fake-chart-footer'
      const totalLabel = document.createElement('span')
      totalLabel.textContent = `Visible total ${input.format(input.encoding.value ?? '', total)}`
      footer.append(totalLabel)
      for (const row of input.frame.rows) {
        const key = idIndex >= 0 ? row[idIndex] : undefined
        const label = labelIndex >= 0 ? row[labelIndex] : key
        if (typeof key !== 'string') continue
        const legend = document.createElement('button')
        legend.type = 'button'
        legend.textContent = typeof label === 'string' ? label : key
        legend.setAttribute('aria-pressed', String(!hidden.has(key)))
        legend.onclick = () => {
          if (hidden.has(key)) hidden.delete(key)
          else hidden.add(key)
          render(input)
        }
        footer.append(legend)
      }
      visual.append(plot, footer)
      el.replaceChildren(visual)
    }
    render(initialInput)
    return { update: render, dispose: () => el.replaceChildren() }
  },
}

function TriggerQuery({ enabled }: { enabled: boolean }) {
  const { drillInto } = useDrill()
  useEffect(() => {
    if (enabled) drillInto('root')
  }, [drillInto, enabled])
  return null
}

function StoryPanel({ panel }: { panel: Panel }) {
  if (panel.kind === 'stat') return <StatPanel panel={panel} />
  if (panel.kind === 'pie' || panel.kind === 'donut') return <PiePanel panel={panel} adapter={storyChartAdapter} />
  if (panel.kind === 'bar' || panel.kind === 'hbar') return <BarPanel panel={panel} adapter={storyChartAdapter} />
  return <LinePanel panel={panel} adapter={storyChartAdapter} />
}

function MatrixCell({ kind, state }: { kind: V1Kind; state: PanelState }) {
  const document = storyDocument(kind, state)
  const fetcher: typeof fetch = () => state === 'error'
    ? Promise.resolve(new Response(JSON.stringify({ error: 'internal', message: 'Data source unavailable' }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' },
      }))
    : new Promise<Response>(() => undefined)

  return (
    <div className="lens-story-cell">
      <span className="lens-story-cell-label">{kind} · {state}</span>
      <DocumentProvider initialDocument={document} fetcher={fetcher}>
        <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
          <TriggerQuery enabled={state === 'loading' || state === 'stale' || state === 'error'} />
          <StoryPanel panel={document.panels[0]!} />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

function PanelMatrix({ theme }: { theme: 'light' | 'dark' }) {
  return (
    <div className="lens-root lens-story-matrix" data-theme={theme}>
      <div className="lens-story-matrix-grid">
        <span />
        {states.map((state) => <strong key={state}>{state}</strong>)}
        {kinds.flatMap((kind) => [
          <strong className="lens-story-row-label" key={`${kind}-label`}>{kind}</strong>,
          ...states.map((state) => <MatrixCell kind={kind} state={state} key={`${kind}-${state}`} />),
        ])}
      </div>
    </div>
  )
}

export const Light: Story = () => <PanelMatrix theme="light" />
Light.storyName = 'All kinds and states · light'

export const Dark: Story = () => <PanelMatrix theme="dark" />
Dark.storyName = 'All kinds and states · dark'
