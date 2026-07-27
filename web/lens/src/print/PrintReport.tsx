import { useCallback, useMemo } from 'react'
import { createPortal } from 'react-dom'
import type { Encoding, FieldFormat, Frame, Panel, PanelKind, Semantics, Theme } from '../contract'
import { ChartHost } from '../panels/ChartHost'
import { seriesColorResolver } from '../panels/data'
import {
  formatAxis,
  formatFieldValue,
  formatFieldValueExact,
  useDashboard,
  usePrint,
  useTranslate,
} from '../runtime'
import type { PrintSection } from '../runtime/print'

const chartKinds = new Set<PanelKind>(['pie', 'donut', 'radial', 'bar', 'hbar', 'line', 'area'])

function numeric(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

function text(value: unknown): string {
  if (value === null || value === undefined) return '—'
  if (typeof value === 'string') return value || '—'
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return String(value)
  return '—'
}

function indexOf(frame: Frame, field: string | undefined): number {
  return field ? frame.columns.findIndex(({ name }) => name === field) : -1
}

function formatsForEncoding(panel: Panel, encoding: Encoding): Record<string, FieldFormat> {
  const formats = { ...panel.format }
  for (const [role, target] of Object.entries(encoding) as Array<[keyof Encoding, string | undefined]>) {
    const source = panel.encoding[role]
    if (target && source && panel.format[source] && !formats[target]) formats[target] = panel.format[source]!
  }
  return formats
}

function viewFor(semantics: Semantics, preferred: PanelKind): PanelKind {
  if (semantics === 'partition') {
    return ['pie', 'donut', 'bar', 'hbar'].includes(preferred) ? preferred : 'donut'
  }
  if (semantics === 'reconciliation') return 'cascade'
  if (semantics === 'series') return 'line'
  return 'table'
}

function sectionPanel(section: PrintSection): Panel {
  const semantics = section.perspective?.semantics ?? section.panel.semantics
  const encoding = section.level.encoding ?? section.panel.encoding
  return {
    ...section.panel,
    kind: section.root ? section.panel.kind : (section.level.view ?? viewFor(semantics, section.panel.kind)),
    title: section.level.label.trim() || section.panel.title,
    semantics,
    encoding,
    format: formatsForEncoding(section.panel, encoding),
    presentation: section.level.presentation ?? section.panel.presentation,
  }
}

interface AuditRow {
  key: string
  series?: string
  label: string
  value: string
  exact?: string
  share?: string
  color?: string
}

function auditRows(section: PrintSection, locale: string, theme: Theme): Array<AuditRow> {
  const frame = section.frame
  if (!frame) return []
  const panel = sectionPanel(section)
  const labelIndex = indexOf(frame, panel.encoding.label ?? panel.encoding.category ?? panel.encoding.id)
  const valueIndex = indexOf(frame, panel.encoding.value)
  const seriesIndex = indexOf(frame, panel.encoding.series)
  const idIndex = indexOf(frame, panel.encoding.id)
  const valueField = panel.encoding.value
  const valueFormat = valueField ? panel.format[valueField] : undefined
  const values = frame.rows.map((row) => numeric(row[valueIndex]))
  const groups = new Map<string, number>()

  frame.rows.forEach((row, rowIndex) => {
    const group = seriesIndex >= 0 ? text(row[seriesIndex]) : ''
    const value = values[rowIndex]
    if (value !== undefined && value >= 0) groups.set(group, (groups.get(group) ?? 0) + value)
  })

  const resolveColor = seriesColorResolver(theme, panel, { positional: section.root })
  return frame.rows.map((row, rowIndex) => {
    const rawValue = valueIndex >= 0 ? row[valueIndex] : undefined
    const number = values[rowIndex]
    const group = seriesIndex >= 0 ? text(row[seriesIndex]) : ''
    const denominator = panel.radial?.mode === 'progress' ? panel.radial.max : groups.get(group)
    const ratio = (panel.semantics === 'partition' || panel.radial?.mode === 'progress') &&
      number !== undefined && number >= 0 && denominator && denominator > 0
      ? number / denominator
      : undefined
    const label = labelIndex >= 0 ? text(row[labelIndex]) : text(row[idIndex])
    return {
      key: `${group}:${idIndex >= 0 ? text(row[idIndex]) : rowIndex}`,
      ...(group ? { series: group } : {}),
      label,
      value: valueIndex >= 0 ? formatFieldValue(rawValue, valueFormat, locale) : '—',
      exact: valueIndex >= 0 ? formatFieldValueExact(rawValue, valueFormat, locale) : undefined,
      share: ratio === undefined ? undefined : new Intl.NumberFormat(locale, {
        style: 'percent',
        minimumFractionDigits: 1,
        maximumFractionDigits: 1,
      }).format(ratio),
      color: resolveColor(label, rowIndex),
    }
  })
}

function PrintChart({ section }: { section: PrintSection }) {
  const panel = useMemo(() => sectionPanel(section), [section])
  const format = useCallback(
    (field: string, value: unknown) => formatFieldValue(value, panel.format[field], section.document.meta.locale),
    [panel.format, section.document.meta.locale],
  )
  const formatChartAxis = useCallback(
    (field: string, value: unknown) => formatAxis(value, panel.format[field], section.document.meta.locale),
    [panel.format, section.document.meta.locale],
  )
  // A one-point chart adds no visual evidence and consumes most of a printed
  // page; the exact value remains in the table immediately below.
  if (!section.frame || section.frame.rows.length <= 1 || !chartKinds.has(panel.kind)) return null
  return (
    <div className="lens-print-chart">
      <ChartHost
        input={{
          kind: panel.kind as Extract<PanelKind, 'pie' | 'donut' | 'radial' | 'bar' | 'hbar' | 'line' | 'area'>,
          frame: section.frame,
          encoding: panel.encoding,
          format,
          formatAxis: formatChartAxis,
          theme: section.document.theme,
          presentation: panel.presentation,
          radial: panel.radial,
        }}
        label={panel.title}
        panelId={`print:${section.id}`}
      />
    </div>
  )
}

function PrintDataTable({ section }: { section: PrintSection }) {
  const translate = useTranslate()
  const rows = useMemo(
    () => auditRows(section, section.document.meta.locale, section.document.theme),
    [section],
  )
  if (!section.frame) {
    return <p className="lens-print-empty">{translate('print.noData', 'No data')}</p>
  }
  const panel = sectionPanel(section)
  if (panel.kind === 'table' && panel.columns?.length) {
    const indexes = panel.columns.map(({ field }) => indexOf(section.frame!, field))
    return (
      <table className="lens-print-data lens-print-data-wide">
        <thead>
          <tr>{panel.columns.map((column) => <th key={column.field}>{column.label}</th>)}</tr>
        </thead>
        <tbody>
          {section.frame.rows.map((row, rowIndex) => (
            <tr key={rowIndex}>
              {panel.columns!.map((column, columnIndex) => {
                const raw = indexes[columnIndex]! >= 0 ? row[indexes[columnIndex]!] : undefined
                const formatted = formatFieldValue(raw, panel.format[column.field], section.document.meta.locale)
                const exact = formatFieldValueExact(raw, panel.format[column.field], section.document.meta.locale)
                return (
                  <td key={column.field}>
                    {formatted}
                    {exact && exact !== formatted && <small>{exact}</small>}
                  </td>
                )
              })}
            </tr>
          ))}
        </tbody>
      </table>
    )
  }
  return (
    <table className="lens-print-data">
      <thead>
        <tr>
          <th>{translate('print.category', 'Category')}</th>
          <th>{translate('print.value', 'Value')}</th>
          <th>{translate('print.share', 'Share')}</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr key={row.key}>
            <td>
              <span className="lens-print-swatch" style={{ backgroundColor: row.color }} />
              {row.series && <span className="lens-print-series">{row.series} · </span>}
              {row.label}
            </td>
            <td>
              {row.value}
              {row.exact && row.exact !== row.value && <small>{row.exact}</small>}
            </td>
            <td>{row.share ?? '—'}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function PrintSectionView({ section, index }: { section: PrintSection; index: number }) {
  const translate = useTranslate()
  const title = section.breadcrumb
    .filter((part, partIndex, all) => part && part !== all[partIndex - 1])
    .join(' › ')
  return (
    <section className={`lens-print-section${section.root ? ' lens-print-section-root' : ''}`}>
      <header className="lens-print-section-header">
        <span>{String(index + 1).padStart(2, '0')}</span>
        <div>
          <h3>{title}</h3>
          {section.perspective && <p>{translate('print.view', 'View')}: {section.perspective.label}</p>}
        </div>
      </header>
      <PrintChart section={section} />
      <PrintDataTable section={section} />
    </section>
  )
}

export function PrintReport() {
  const { document } = useDashboard()
  const print = usePrint()
  const translate = useTranslate()
  if (!print.report || typeof globalThis.document === 'undefined') return null
  return createPortal(
    <article className="lens-print-report" aria-hidden={!print.active}>
      <header className="lens-print-cover">
        <p className="lens-print-kicker">{translate('print.kicker', 'Management audit report')}</p>
        <h1>{translate('print.title', 'Dashboard report')}</h1>
        <h2>{document.header?.title || document.meta.title}</h2>
        {document.header?.subtitle && <p>{document.header.subtitle}</p>}
        <dl>
          <div>
            <dt>{translate('print.generated', 'Generated')}</dt>
            <dd>{new Intl.DateTimeFormat(document.meta.locale, {
              dateStyle: 'long',
              timeStyle: 'short',
            }).format(new Date())}</dd>
          </div>
          <div>
            <dt>{translate('print.sections', 'Detailed views')}</dt>
            <dd>{print.report.sections.length}</dd>
          </div>
        </dl>
      </header>
      <section className="lens-print-index">
        <h2>{translate('print.appendix', 'Figures and detailed breakdowns')}</h2>
        <p>{translate(
          'print.note',
          'Every interactive view is expanded below. Values and shares accompany each chart so the report remains self-contained on paper.',
        )}</p>
      </section>
      {(print.report.truncated || print.report.warnings.length > 0) && (
        <section className="lens-print-warnings">
          <h2>{translate('print.limitations', 'Data limitations')}</h2>
          <p>{translate(
            'print.limitationsNote',
            'The report remains usable, but the following detail views could not be calculated and are explicitly disclosed:',
          )}</p>
          <ul>
            {print.report.truncated && (
              <li>{translate(
                'print.timeLimit',
                'Further detail was stopped at the preparation time limit; all sections calculated by then are included.',
              )}</li>
            )}
            {print.report.warnings.map((warning, index) => <li key={`${index}:${warning}`}>{warning}</li>)}
          </ul>
        </section>
      )}
      {print.report.sections.map((section, index) => (
        <PrintSectionView index={index} key={section.id} section={section} />
      ))}
    </article>,
    globalThis.document.body,
  )
}
