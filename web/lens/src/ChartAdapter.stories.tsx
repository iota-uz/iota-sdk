import { createEffect, createSignal, on, onCleanup, onMount, type JSX } from 'solid-js'
import type { Encoding, Frame, Theme } from './contract'
import { getChartAdapter, type ChartInput, type ChartInstance, type ChartKind } from './charts'
import { formatFieldValue } from './runtime'
import './styles.css'

const chartTheme: Theme = {
  palette: {
    blue: '#2563eb',
    green: '#059669',
    amber: '#d97706',
    violet: '#7c3aed',
  },
  series: { Revenue: 'blue', Cost: 'amber', North: 'blue', South: 'green', East: 'violet' },
}

const partitionFrame: Frame = {
  columns: [
    { name: 'id', type: 'string' },
    { name: 'label', type: 'string' },
    { name: 'value', type: 'number' },
  ],
  rows: [
    ['north', 'North', 42],
    ['south', 'South', 31],
    ['east', 'East', 19],
    ['west', 'West', 8],
  ],
}

const seriesFrame: Frame = {
  columns: [
    { name: 'id', type: 'string' },
    { name: 'month', type: 'string' },
    { name: 'series', type: 'string' },
    { name: 'value', type: 'number' },
  ],
  rows: [
    ['jan-revenue', 'Jan', 'Revenue', 72000], ['jan-cost', 'Jan', 'Cost', 41000],
    ['feb-revenue', 'Feb', 'Revenue', 84000], ['feb-cost', 'Feb', 'Cost', 46000],
    ['mar-revenue', 'Mar', 'Revenue', 91000], ['mar-cost', 'Mar', 'Cost', 49000],
    ['apr-revenue', 'Apr', 'Revenue', 105000], ['apr-cost', 'Apr', 'Cost', 54000],
  ],
}

const partitionEncoding: Encoding = { id: 'id', label: 'label', value: 'value' }
const seriesEncoding: Encoding = { id: 'id', category: 'month', series: 'series', value: 'value' }
const radialPartitionFrame: Frame = {
  columns: [
    { name: 'id', type: 'string' },
    { name: 'label', type: 'string' },
    { name: 'ring', type: 'string' },
    { name: 'value', type: 'number' },
  ],
  rows: [
    ['north', 'North', 'actual', 42], ['south', 'South', 'actual', 31],
    ['east', 'East', 'actual', 27], ['north', 'North', 'plan', 36],
    ['south', 'South', 'plan', 34], ['east', 'East', 'plan', 30],
  ],
  total: 100,
}
const radialProgressFrame: Frame = {
  columns: partitionFrame.columns,
  rows: [['quality', 'Quality', 86], ['delivery', 'Delivery', 73], ['coverage', 'Coverage', 61]],
}
const money = { kind: 'money', currency: 'USD', minorUnits: false, precision: 0 } as const

function input(kind: ChartKind, selectedKey?: string): ChartInput {
  const partition = kind === 'pie' || kind === 'donut'
  return {
    kind,
    frame: partition ? partitionFrame : seriesFrame,
    encoding: partition ? partitionEncoding : seriesEncoding,
    format: (_field, value) => formatFieldValue(value, partition ? { kind: 'percent', minorUnits: false } : money, 'en-US'),
    theme: chartTheme,
    selectedKey,
  }
}

function radialInput(mode: 'partition' | 'progress'): ChartInput {
  if (mode === 'progress') {
    return {
      kind: 'radial',
      frame: radialProgressFrame,
      encoding: partitionEncoding,
      format: (_field, value) => formatFieldValue(value, { kind: 'number', minorUnits: false, precision: 0 }, 'en-US'),
      theme: chartTheme,
      radial: { mode: 'progress', max: 120 },
    }
  }
  return {
    kind: 'radial',
    frame: radialPartitionFrame,
    encoding: { id: 'id', label: 'label', series: 'ring', value: 'value' },
    format: (_field, value) => formatFieldValue(value, { kind: 'number', minorUnits: false, precision: 0 }, 'en-US'),
    theme: chartTheme,
    presentation: { sliceLabels: 'percent' },
    tooltipTotalLabel: 'Total',
    radial: {
      mode: 'partition',
      rings: [
        { key: 'actual', label: 'Actual', order: 1, total: 100 },
        { key: 'plan', label: 'Plan', order: 2, total: 100 },
      ],
    },
  }
}

function ChartPreview(props: { chartInput: ChartInput, onSelect?: (key: string) => void }): JSX.Element {
  let element: HTMLDivElement | undefined
  let instance: ChartInstance | undefined

  onMount(() => {
    let active = true
    void getChartAdapter(props.chartInput.kind).then((adapter) => {
      if (!active) return
      instance = adapter.mount(element!, props.chartInput, {
        onSelect: (key) => props.onSelect?.(key),
        onHover: () => undefined,
      })
    })
    onCleanup(() => {
      active = false
      instance?.dispose()
      instance = undefined
    })
  })

  createEffect(on(() => props.chartInput, (current) => {
    instance?.update(current)
  }))

  return <div ref={element} style={{ width: '100%', height: '320px' }} />
}

function Family(props: { kinds: [ChartKind, ChartKind], mode: 'light' | 'dark' }): JSX.Element {
  return (
    <div class="lens-root" data-theme={props.mode} style={{ display: 'grid', 'grid-template-columns': 'repeat(2, minmax(0, 1fr))', gap: '16px' }}>
      {props.kinds.map((kind) => (
        <section class="lens-stat-card">
          <h2 class="lens-m-0 lens-text-md lens-font-semibold lens-text-strong">{kind}</h2>
          <ChartPreview chartInput={input(kind)} />
        </section>
      ))}
    </div>
  )
}

export const PieAndDonutLight = () => <Family kinds={['pie', 'donut']} mode="light" />
export const PieAndDonutDark = () => <Family kinds={['pie', 'donut']} mode="dark" />
export const BarAndHorizontalBarLight = () => <Family kinds={['bar', 'hbar']} mode="light" />
export const BarAndHorizontalBarDark = () => <Family kinds={['bar', 'hbar']} mode="dark" />
export const LineAndAreaLight = () => <Family kinds={['line', 'area']} mode="light" />
export const LineAndAreaDark = () => <Family kinds={['line', 'area']} mode="dark" />

function RadialFamily(props: { mode: 'light' | 'dark' }): JSX.Element {
  return (
    <div class="lens-root" data-theme={props.mode} style={{ display: 'grid', 'grid-template-columns': 'repeat(2, minmax(0, 1fr))', gap: '16px' }}>
      <section class="lens-stat-card">
        <h2 class="lens-m-0 lens-text-md lens-font-semibold lens-text-strong">Multi-ring partition</h2>
        <ChartPreview chartInput={radialInput('partition')} />
      </section>
      <section class="lens-stat-card">
        <h2 class="lens-m-0 lens-text-md lens-font-semibold lens-text-strong">Radial progress</h2>
        <ChartPreview chartInput={radialInput('progress')} />
      </section>
    </div>
  )
}

/**
 * The share that only matters because it is small: a collection ring where
 * 99.1% has been received and the 0.9% still owed is the whole reason anyone
 * opens the panel. Inside the arc that number has nowhere to print, so it moves
 * outside on a leader line.
 */
const receivableRingFrame: Frame = {
  columns: radialPartitionFrame.columns,
  rows: [
    ['collected', 'Collected', 'payment', 99.1],
    ['receivable', 'Receivable', 'payment', 0.9],
  ],
  total: 100,
}

function receivableRingInput(): ChartInput {
  return {
    kind: 'radial',
    frame: receivableRingFrame,
    encoding: { id: 'id', label: 'label', series: 'ring', value: 'value' },
    format: (_field, value) => formatFieldValue(value, { kind: 'percent', minorUnits: false, precision: 1 }, 'en-US'),
    theme: chartTheme,
    presentation: { sliceLabels: 'percent' },
    tooltipTotalLabel: 'Total',
    radial: { mode: 'partition', rings: [{ key: 'payment', label: 'Collection', order: 1, total: 100 }] },
  }
}

export const RadialMicroSlice = () => (
  <div class="lens-root" data-theme="light" style={{ display: 'grid', 'grid-template-columns': 'repeat(2, minmax(0, 1fr))', gap: '16px' }}>
    <section class="lens-stat-card">
      <h2 class="lens-m-0 lens-text-md lens-font-semibold lens-text-strong">Sub-1% share, called out</h2>
      <ChartPreview chartInput={receivableRingInput()} />
    </section>
    <section class="lens-stat-card lens-root" data-theme="dark">
      <h2 class="lens-m-0 lens-text-md lens-font-semibold lens-text-strong">Dark</h2>
      <ChartPreview chartInput={receivableRingInput()} />
    </section>
  </div>
)

/**
 * Three decompositions of one whole — the shape the accumulated-premium donut
 * ships: how much is recognised, how much is collected, and which risk groups
 * it came from. Three rings is where the band arithmetic is tightest, so this
 * is the case that proves the hub still clears the innermost ring.
 */
const threeRingFrame: Frame = {
  columns: radialPartitionFrame.columns,
  rows: [
    ['earned', 'Earned', 'recognition', 64],
    ['unearned', 'Unearned', 'recognition', 36],
    ['received', 'Received', 'payment', 91],
    ['receivable', 'Receivable', 'payment', 9],
    ['motor', 'Motor', 'groups', 52],
    ['property', 'Property', 'groups', 31],
    ['liability', 'Liability', 'groups', 17],
  ],
  total: 100,
}

function threeRingInput(): ChartInput {
  return {
    kind: 'radial',
    frame: threeRingFrame,
    encoding: { id: 'id', label: 'label', series: 'ring', value: 'value' },
    format: (_field, value) => formatFieldValue(value, { kind: 'number', minorUnits: false, precision: 0 }, 'en-US'),
    theme: chartTheme,
    presentation: { sliceLabels: 'percent' },
    tooltipTotalLabel: 'Total',
    radial: {
      mode: 'partition',
      rings: [
        { key: 'recognition', label: 'Recognition', order: 1, total: 100 },
        { key: 'payment', label: 'Collection', order: 2, total: 100 },
        { key: 'groups', label: 'Risk group', order: 3, total: 100 },
      ],
    },
  }
}

export const RadialLight = () => <RadialFamily mode="light" />
export const RadialDark = () => <RadialFamily mode="dark" />
export const RadialThreeRings = () => (
  <div class="lens-root" data-theme="light" style={{ display: 'grid', 'grid-template-columns': 'repeat(2, minmax(0, 1fr))', gap: '16px' }}>
    <section class="lens-stat-card">
      <h2 class="lens-m-0 lens-text-md lens-font-semibold lens-text-strong">Three rings, one whole</h2>
      <ChartPreview chartInput={threeRingInput()} />
    </section>
    <section class="lens-stat-card lens-root" data-theme="dark">
      <h2 class="lens-m-0 lens-text-md lens-font-semibold lens-text-strong">Dark</h2>
      <ChartPreview chartInput={threeRingInput()} />
    </section>
  </div>
)
export const RadialNarrow = () => (
  <div class="lens-root" data-theme="light" style={{ width: '420px' }}>
    <section class="lens-stat-card">
      <h2 class="lens-m-0 lens-text-md lens-font-semibold lens-text-strong">Multi-ring on compact cards</h2>
      <ChartPreview chartInput={radialInput('partition')} />
    </section>
  </div>
)

export const ControlledSelection = () => {
  const [selectedKey, setSelectedKey] = createSignal<string>()
  const chartInput = () => input('donut', selectedKey())
  return (
    <div class="lens-root" data-theme="light">
      <section class="lens-stat-card" style={{ 'max-width': '640px' }}>
        <p class="lens-m-0 lens-text-md lens-text-muted">Selected NodeKey: {selectedKey() ?? 'none'}</p>
        <ChartPreview chartInput={chartInput()} onSelect={(key) => setSelectedKey(key)} />
      </section>
    </div>
  )
}
