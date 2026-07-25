import type { FocusParent } from './focusModel'

/**
 * Static SVG thumbnail of the parent level with the focused element
 * highlighted — a donut for part-to-whole parents, a bar strip for cascade
 * parents. It is a button that returns to the parent; purely decorative
 * otherwise (`aria-hidden` on the drawing, the label carries the meaning).
 */
export interface ContextMiniChartProps {
  parent: FocusParent
  colorFor: (label: string, index: number) => string | undefined
  /** Share of the focused slice, pre-formatted for the donut center. */
  centerLabel?: string
  caption?: string
  label: string
  onClick?: () => void
}

function arcPath(cx: number, cy: number, r: number, from: number, to: number): string {
  const x0 = cx + r * Math.cos(from)
  const y0 = cy + r * Math.sin(from)
  const x1 = cx + r * Math.cos(to)
  const y1 = cy + r * Math.sin(to)
  const large = to - from > Math.PI ? 1 : 0
  return `M ${x0.toFixed(2)} ${y0.toFixed(2)} A ${r} ${r} 0 ${large} 1 ${x1.toFixed(2)} ${y1.toFixed(2)}`
}

function MiniDonut({ parent, colorFor, centerLabel }: {
  parent: FocusParent
  colorFor: ContextMiniChartProps['colorFor']
  centerLabel?: string
}) {
  const size = 64
  const center = size / 2
  const radius = 24
  const positive = parent.slices.map((slice) => Math.max(0, slice.value))
  const total = positive.reduce((sum, value) => sum + value, 0)
  if (total <= 0) return null
  const gap = 0.035
  let angle = -Math.PI / 2
  const arcs = parent.slices.map((slice, index) => {
    const sweep = ((positive[index] ?? 0) / total) * Math.PI * 2
    const path = sweep > gap * 2 ? arcPath(center, center, radius, angle + gap, angle + sweep - gap) : undefined
    angle += sweep
    if (!path) return null
    return (
      <path
        d={path}
        fill="none"
        key={slice.key}
        opacity={slice.selected ? 1 : 0.42}
        style={{ stroke: colorFor(slice.label, index) }}
        strokeLinecap="butt"
        strokeWidth={slice.selected ? 10 : 7}
      />
    )
  })
  return (
    <svg aria-hidden="true" height={size} viewBox={`0 0 ${size} ${size}`} width={size}>
      {arcs}
      {centerLabel && (
        <text
          className="lens-focus-mini-center"
          fontSize="10"
          fontWeight="600"
          textAnchor="middle"
          x={center}
          y={center + 3.5}
        >
          {centerLabel}
        </text>
      )}
    </svg>
  )
}

function MiniStages({ parent, colorFor }: {
  parent: FocusParent
  colorFor: ContextMiniChartProps['colorFor']
}) {
  const width = 84
  const height = 52
  const count = parent.slices.length
  if (count === 0) return null
  const gap = 3
  const barWidth = (width - (count - 1) * gap) / count
  const max = parent.slices.reduce((peak, slice) => Math.max(peak, Math.abs(slice.value)), 0)
  if (max <= 0) return null
  const baseline = height - 4
  const bars = parent.slices.map((slice, index) => {
    const magnitude = Math.max(2, ((height - 10) * Math.abs(slice.value)) / max)
    return (
      <rect
        height={magnitude}
        key={slice.key}
        opacity={slice.selected ? 1 : 0.3}
        rx={1}
        style={{ fill: colorFor(slice.label, index) }}
        width={barWidth}
        x={index * (barWidth + gap)}
        y={baseline - magnitude}
      />
    )
  })
  return (
    <svg aria-hidden="true" height={height} viewBox={`0 0 ${width} ${height}`} width={width}>
      <line className="lens-focus-mini-baseline" x1={0} x2={width} y1={baseline} y2={baseline} />
      {bars}
    </svg>
  )
}

export function ContextMiniChart({ parent, colorFor, centerLabel, caption, label, onClick }: ContextMiniChartProps) {
  // Emptiness is decided here, not by the inner drawing, so a valueless parent
  // never leaves a hollow button in the header.
  const positiveTotal = parent.slices.reduce((sum, slice) => sum + Math.max(0, slice.value), 0)
  const magnitude = parent.slices.reduce((peak, slice) => Math.max(peak, Math.abs(slice.value)), 0)
  const empty = parent.slices.length === 0 || (parent.view === 'cascade' ? magnitude <= 0 : positiveTotal <= 0)
  if (empty) return null
  const drawing = parent.view === 'cascade'
    ? <MiniStages colorFor={colorFor} parent={parent} />
    : <MiniDonut centerLabel={centerLabel} colorFor={colorFor} parent={parent} />
  return (
    <button
      aria-label={label}
      className="lens-focus-mini"
      disabled={!onClick}
      onClick={onClick}
      title={onClick ? label : undefined}
      type="button"
    >
      {drawing}
      {caption && <span className="lens-focus-mini-caption">{caption}</span>}
    </button>
  )
}
