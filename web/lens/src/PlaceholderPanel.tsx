import type { Frame, Panel } from './contract'
import { useDashboard, useFormat, usePanelFrame } from './runtime'

export interface PlaceholderPanelProps {
  src?: string
}

function findColumn(name: string | undefined): string | undefined {
  return name?.trim() || undefined
}

function displayString(value: unknown, fallback: string): string {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') {
    return String(value)
  }
  return fallback
}

function readStat(frame: Frame | undefined, panel: Panel) {
  const row = frame?.rows[0]
  const labelName = findColumn(panel.encoding.label)
  const valueName = findColumn(panel.encoding.value)
  const labelIndex = frame?.columns.findIndex((column) => column.name === labelName) ?? -1
  const valueIndex = frame?.columns.findIndex((column) => column.name === valueName) ?? -1

  return {
    label: labelIndex >= 0 ? displayString(row?.[labelIndex], panel.title) : panel.title,
    value: valueIndex >= 0 ? row?.[valueIndex] : undefined,
  }
}

export function PlaceholderPanel({ src }: PlaceholderPanelProps) {
  const { document } = useDashboard()
  const panel = document.panels.find((candidate) => candidate.kind === 'stat') ?? document.panels[0]

  if (!panel) {
    return <div className="lens-placeholder-state">The fixture contains no panels.</div>
  }

  return <PlaceholderPanelFrame panel={panel} src={src} />
}

function PlaceholderPanelFrame({ panel, src }: { panel: Panel; src?: string }) {
  const { document } = useDashboard()
  const frame = usePanelFrame(panel.id)
  const valueField = findColumn(panel.encoding.value)
  const format = useFormat(valueField ? panel.format[valueField] : undefined)
  const stat = readStat(frame.data, panel)

  if (!frame.data && frame.isLoading) {
    return <div className="lens-placeholder-state lens-skeleton" aria-busy="true">Loading panel…</div>
  }

  return (
    <section
      className={`lens-stat-card${frame.isStale ? ' lens-panel-stale' : ''}`}
      aria-label={panel.title}
      aria-busy={frame.isLoading}
      data-stale={frame.isStale || undefined}
    >
      <div className="lens-flex lens-items-start lens-justify-between lens-gap-4">
        <div>
          <p className="lens-m-0 lens-text-xs lens-font-semibold lens-uppercase lens-tracking-wider lens-text-muted">
            {stat.label}
          </p>
          <p className="lens-m-0 lens-mt-3 lens-text-3xl lens-font-semibold lens-tabular-nums lens-text-strong">
            {format(stat.value)}
          </p>
        </div>
        <span className="lens-runtime-badge">React runtime</span>
      </div>
      <p className="lens-m-0 lens-mt-5 lens-border-0 lens-border-t lens-border-solid lens-border-border lens-pt-3 lens-text-xs lens-text-muted">
        {src ? `Loaded from ${src}` : 'Bundled fixture'} · contract {document.version}
      </p>
      {frame.error && (
        <div className="lens-panel-error" role="alert">
          <span>{frame.error.message}</span>
          <button type="button" onClick={frame.retry}>Retry</button>
        </div>
      )}
    </section>
  )
}
