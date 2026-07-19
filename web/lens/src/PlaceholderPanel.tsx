import type { DashboardDocument, Panel } from './contract'

export interface PlaceholderPanelProps {
  document: DashboardDocument
  locale: string
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

function readStat(document: DashboardDocument, panel: Panel) {
  const frame = document.frames[panel.frame]
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

function formatValue(value: unknown, locale: string): string {
  if (typeof value === 'number') {
    return new Intl.NumberFormat(locale).format(value)
  }
  return displayString(value, '—')
}

export function PlaceholderPanel({ document, locale, src }: PlaceholderPanelProps) {
  const panel = document.panels.find((candidate) => candidate.kind === 'stat') ?? document.panels[0]

  if (!panel) {
    return <div className="lens-placeholder-state">The fixture contains no panels.</div>
  }

  const stat = readStat(document, panel)

  return (
    <section className="lens-stat-card" aria-label={panel.title}>
      <div className="lens-flex lens-items-start lens-justify-between lens-gap-4">
        <div>
          <p className="lens-m-0 lens-text-xs lens-font-semibold lens-uppercase lens-tracking-wider lens-text-muted">
            {stat.label}
          </p>
          <p className="lens-m-0 lens-mt-3 lens-text-3xl lens-font-semibold lens-tabular-nums lens-text-strong">
            {formatValue(stat.value, locale)}
          </p>
        </div>
        <span className="lens-runtime-badge">React runtime</span>
      </div>
      <p className="lens-m-0 lens-mt-5 lens-border-0 lens-border-t lens-border-solid lens-border-border lens-pt-3 lens-text-xs lens-text-muted">
        {src ? `Loaded from ${src}` : 'Bundled fixture'} · contract {document.version}
      </p>
    </section>
  )
}
