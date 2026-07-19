import type { Story } from '@ladle/react'
import { useEffect, useMemo, useRef, useState } from 'react'
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

function ChartPreview({ chartInput, onSelect }: { chartInput: ChartInput, onSelect?: (key: string) => void }) {
  const element = useRef<HTMLDivElement>(null)
  const instance = useRef<ChartInstance>()
  const currentInput = useRef(chartInput)
  currentInput.current = chartInput

  useEffect(() => {
    let active = true
    if (!element.current) return
    const target = element.current
    void getChartAdapter().then((adapter) => {
      if (!active) return
      instance.current = adapter.mount(target, currentInput.current, {
        onSelect: (key) => onSelect?.(key),
        onHover: () => undefined,
      })
    })
    return () => {
      active = false
      instance.current?.dispose()
      instance.current = undefined
    }
  }, [onSelect])

  useEffect(() => {
    instance.current?.update(chartInput)
  }, [chartInput])

  return <div ref={element} style={{ width: '100%', height: 320 }} />
}

function Family({ kinds, mode }: { kinds: [ChartKind, ChartKind], mode: 'light' | 'dark' }) {
  return (
    <div className="lens-root" data-theme={mode} style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 16 }}>
      {kinds.map((kind) => (
        <section key={kind} className="lens-stat-card">
          <h2 className="lens-m-0 lens-text-sm lens-font-semibold lens-text-strong">{kind}</h2>
          <ChartPreview chartInput={input(kind)} />
        </section>
      ))}
    </div>
  )
}

export const PieAndDonutLight: Story = () => <Family kinds={['pie', 'donut']} mode="light" />
export const PieAndDonutDark: Story = () => <Family kinds={['pie', 'donut']} mode="dark" />
export const BarAndHorizontalBarLight: Story = () => <Family kinds={['bar', 'hbar']} mode="light" />
export const BarAndHorizontalBarDark: Story = () => <Family kinds={['bar', 'hbar']} mode="dark" />
export const LineAndAreaLight: Story = () => <Family kinds={['line', 'area']} mode="light" />
export const LineAndAreaDark: Story = () => <Family kinds={['line', 'area']} mode="dark" />

export const ControlledSelection: Story = () => {
  const [selectedKey, setSelectedKey] = useState<string>()
  const chartInput = useMemo(() => input('donut', selectedKey), [selectedKey])
  return (
    <div className="lens-root" data-theme="light">
      <section className="lens-stat-card" style={{ maxWidth: 640 }}>
        <p className="lens-m-0 lens-text-sm lens-text-muted">Selected NodeKey: {selectedKey ?? 'none'}</p>
        <ChartPreview chartInput={chartInput} onSelect={setSelectedKey} />
      </section>
    </div>
  )
}
