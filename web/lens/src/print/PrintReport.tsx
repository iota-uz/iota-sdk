import { useCallback, useMemo, type CSSProperties } from 'react'
import { createPortal } from 'react-dom'
import type { DashboardDocument, Panel, Theme } from '../contract'
import { buildCascadeStages, buildWaterfallModel } from '../panels/CascadePanel'
import { ChartHost } from '../panels/ChartHost'
import { seriesColorResolver } from '../panels/data'
import { WaterfallPlot } from '../panels/WaterfallPlot'
import {
  formatAxis,
  formatFieldValue,
  formatFieldValueExact,
  useDashboard,
  usePrint,
  useTranslate,
} from '../runtime'
import type { PrintReport as PrintReportModel, PrintSection } from '../runtime/print'
import { narrativeFact } from './narrative'
import { buildOutline, type PrintChapter, type PrintDetail, type PrintFigure, type PrintOutline } from './outline'
import { chartKinds, indexOf, numeric, sectionPanel, text, type ChartKind } from './values'

interface AuditRow {
  key: string
  series?: string
  label: string
  value: string
  exact?: string
  share?: string
  ratio?: number
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
      ...(ratio === undefined ? {} : {
        ratio,
        share: new Intl.NumberFormat(locale, {
          style: 'percent',
          minimumFractionDigits: 1,
          maximumFractionDigits: 1,
        }).format(ratio),
      }),
      color: resolveColor(label, rowIndex),
    }
  })
}

function PrintChart({ section, height }: { section: PrintSection; height: number }) {
  const panel = useMemo(() => sectionPanel(section), [section])
  const format = useCallback(
    (field: string, value: unknown) => formatFieldValue(value, panel.format[field], section.document.meta.locale),
    [panel.format, section.document.meta.locale],
  )
  const formatChartAxis = useCallback(
    (field: string, value: unknown) => formatAxis(value, panel.format[field], section.document.meta.locale),
    [panel.format, section.document.meta.locale],
  )
  if (!section.frame || section.frame.rows.length <= 1 || !chartKinds.has(panel.kind)) return null
  // On paper a slice is named by the evidence table directly beneath it, so the
  // chart keeps its share inside the slice instead of spending a third of a
  // half-page box on leader lines that clip the labels anyway.
  const presentation = panel.kind === 'pie' || panel.kind === 'donut' || panel.kind === 'radial'
    ? {
        ...panel.presentation,
        sliceLabels: panel.presentation?.sliceLabels ?? ('percent' as const),
        // A wider band on paper: the share is written inside the ring, and a
        // thin ring cannot hold five characters.
        fill: true,
      }
    : panel.presentation
  return (
    <div className="lens-print-chart" style={{ height }}>
      <ChartHost
        input={{
          kind: panel.kind as ChartKind,
          frame: section.frame,
          encoding: panel.encoding,
          format,
          formatAxis: formatChartAxis,
          theme: section.document.theme,
          presentation,
          radial: panel.radial,
        }}
        label={panel.title}
        panelId={`print:${section.id}`}
      />
    </div>
  )
}

/**
 * A bridge prints as the bridge. Its stage model is pure arithmetic over the
 * frame, so the printed page can carry the same columns the dashboard draws
 * instead of demoting the argument of the page to a list of numbers.
 */
function PrintWaterfall({ section }: { section: PrintSection }) {
  const panel = sectionPanel(section)
  const locale = section.document.meta.locale
  const model = useMemo(() => {
    if (!section.frame) return undefined
    const valueField = panel.encoding.value ?? 'value'
    const cutField = panel.encoding.cut ?? 'cut'
    const formatValue = (value: unknown) => formatFieldValue(value, panel.format[valueField], locale)
    const formatCut = (value: unknown) => formatFieldValue(
      value,
      panel.format[cutField] ?? panel.format[valueField],
      locale,
    )
    return buildWaterfallModel(buildCascadeStages(panel, section.frame, formatValue, formatCut), formatValue)
  }, [locale, panel, section.frame])
  if (!model || model.items.length === 0) return null
  return (
    <div className="lens-print-chart lens-print-chart-waterfall">
      <WaterfallPlot label={panel.title} model={model} />
    </div>
  )
}

function isWaterfall(panel: Panel): boolean {
  return panel.kind === 'cascade' && panel.presentation?.bridgeLayout === 'waterfall'
}

function PrintDataTable({ section, dense }: { section: PrintSection; dense?: boolean }) {
  const translate = useTranslate()
  const rows = useMemo(
    () => auditRows(section, section.document.meta.locale, section.document.theme),
    [section],
  )
  if (!section.frame) return <p className="lens-print-empty">{translate('print.noData', 'No data')}</p>
  const panel = sectionPanel(section)
  if (panel.kind === 'table' && panel.columns?.length) {
    const indexes = panel.columns.map(({ field }) => indexOf(section.frame!, field))
    return (
      <table className="lens-print-data lens-print-data-wide">
        <thead>
          <tr>
            {panel.columns.map((column) => (
              <th data-align={column.align ?? 'left'} key={column.field}>{column.label}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {section.frame.rows.map((row, rowIndex) => (
            <tr key={rowIndex}>
              {panel.columns!.map((column, columnIndex) => {
                const raw = indexes[columnIndex]! >= 0 ? row[indexes[columnIndex]!] : undefined
                const formatted = formatFieldValue(raw, panel.format[column.field], section.document.meta.locale)
                const exact = formatFieldValueExact(raw, panel.format[column.field], section.document.meta.locale)
                const value = numeric(raw)
                return (
                  <td
                    data-align={column.align ?? 'left'}
                    data-negative={value !== undefined && value < 0 ? '' : undefined}
                    key={column.field}
                  >
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
  const shares = rows.some(({ share }) => share !== undefined)
  return (
    <table className={`lens-print-data${dense ? ' lens-print-data-dense' : ''}`}>
      <thead>
        <tr>
          <th>{translate('print.category', 'Category')}</th>
          <th data-align="right">{translate('print.value', 'Value')}</th>
          {shares && <th data-align="right">{translate('print.share', 'Share')}</th>}
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
            <td data-align="right">
              {row.value}
              {row.exact && row.exact !== row.value && <small>{row.exact}</small>}
            </td>
            {shares && (
              <td className="lens-print-share" data-align="right">
                {/* The bar carries the proportion the eye needs; the number
                    keeps the row auditable. Neither costs an extra line. */}
                {row.ratio !== undefined && (
                  <span
                    aria-hidden="true"
                    className="lens-print-share-bar"
                    style={{ width: `${Math.min(100, Math.round(row.ratio * 100))}%` }}
                  />
                )}
                <span>{row.share ?? '—'}</span>
              </td>
            )}
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function FigureNote({ section }: { section: PrintSection }) {
  const translate = useTranslate()
  const panel = sectionPanel(section)
  const authored = panel.caption?.trim()
  const fact = useMemo(
    () => narrativeFact(panel, section.frame, section.document.meta.locale),
    [panel, section.document.meta.locale, section.frame],
  )
  if (!authored && !fact) return null
  return (
    <p className="lens-print-figure-note">
      {authored && <span className="lens-print-figure-authored">{authored}</span>}
      {fact && <span>{translate(fact.labelKey, fact.fallback, fact.vars)}</span>}
    </p>
  )
}

function FigureView({ figure }: { figure: PrintFigure }) {
  const translate = useTranslate()
  const { section } = figure
  const panel = sectionPanel(section)
  const waterfall = isWaterfall(panel)
  return (
    <figure className={`lens-print-figure lens-print-figure-${waterfall ? 'full' : figure.width}`}>
      <figcaption className="lens-print-figure-head">
        <span className="lens-print-figure-number">
          {translate('print.figure', 'Fig. {number}', { number: figure.number })}
        </span>
        <h3>{panel.title}</h3>
        {panel.status?.label && (
          <span className="lens-print-chip" data-tone={panel.status.tone ?? 'neutral'}>{panel.status.label}</span>
        )}
        {section.perspective && (
          <span className="lens-print-chip" data-tone="neutral">
            {translate('print.view', 'View')}: {section.perspective.label}
          </span>
        )}
      </figcaption>
      {waterfall
        ? <PrintWaterfall section={section} />
        : figure.chart && <PrintChart height={figure.width === 'half' ? 225 : 235} section={section} />}
      <FigureNote section={section} />
      <PrintDataTable section={section} />
    </figure>
  )
}

function KpiView({ figure }: { figure: PrintFigure }) {
  const { section } = figure
  const panel = sectionPanel(section)
  const locale = section.document.meta.locale
  const frame = section.frame
  const valueIndex = frame ? indexOf(frame, panel.encoding.value) : -1
  const raw = frame && valueIndex >= 0 ? frame.rows[0]?.[valueIndex] : undefined
  const format = panel.encoding.value ? panel.format[panel.encoding.value] : undefined
  return (
    <div className="lens-print-kpi">
      <p className="lens-print-kpi-label">{panel.title}</p>
      <p className="lens-print-kpi-value">{formatFieldValue(raw, format, locale)}</p>
      {panel.caption && <p className="lens-print-kpi-caption">{panel.caption}</p>}
      {panel.status?.label && (
        <span className="lens-print-chip" data-tone={panel.status.tone ?? 'neutral'}>{panel.status.label}</span>
      )}
    </div>
  )
}

function periodLabel(document: DashboardDocument, allTime: string): string | undefined {
  const period = document.filters?.find((filter) => filter.kind === 'period')?.period
  if (!period) return undefined
  const { start, end } = period.value
  if (!start && !end) return allTime
  const format = (value: string) => {
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return value
    return new Intl.DateTimeFormat(document.meta.locale, { dateStyle: 'medium' }).format(date)
  }
  if (start && end) return `${format(start)} — ${format(end)}`
  return format(start || end)
}

function Cover({
  document,
  outline,
  report,
}: {
  document: DashboardDocument
  outline: PrintOutline
  report: PrintReportModel
}) {
  const translate = useTranslate()
  const kpis = outline.kpis
  const period = periodLabel(document, translate('filter.period.allTime', 'All time'))
  return (
    <header className="lens-print-cover">
      <div className="lens-print-cover-head">
        <p className="lens-print-kicker">{translate('print.kicker', 'Management audit report')}</p>
        <h1>{document.header?.title || document.meta.title}</h1>
        {document.header?.subtitle && <p className="lens-print-cover-subtitle">{document.header.subtitle}</p>}
      </div>
      <dl className="lens-print-cover-meta">
        {period && (
          <div>
            <dt>{translate('print.period', 'Period')}</dt>
            <dd>{period}</dd>
          </div>
        )}
        <div>
          <dt>{translate('print.generated', 'Generated')}</dt>
          <dd>{new Intl.DateTimeFormat(document.meta.locale, {
            dateStyle: 'long',
            timeStyle: 'short',
          }).format(new Date(document.meta.generatedAt))}</dd>
        </div>
        <div>
          <dt>{translate('print.sections', 'Detailed views')}</dt>
          <dd>{outline.figureCount + outline.detailCount}</dd>
        </div>
      </dl>
      {kpis.length > 0 && (
        <div className="lens-print-kpis">
          {kpis.map((figure) => <KpiView figure={figure} key={figure.section.id} />)}
        </div>
      )}
      {(report.truncated || report.warnings.length > 0 || outline.missing.length > 0) && (
        <p className="lens-print-cover-flag">{translate(
          'print.limitationsFlag',
          'This report discloses gaps in its data; see the closing appendix.',
        )}</p>
      )}
    </header>
  )
}

function Contents({ outline }: { outline: PrintOutline }) {
  const translate = useTranslate()
  return (
    <section className="lens-print-contents">
      <h2>{translate('print.contents', 'Contents')}</h2>
      <ol>
        {outline.chapters.map((chapter) => (
          <li key={chapter.id}>
            <span className="lens-print-contents-number">{chapter.number}</span>
            <span className="lens-print-contents-title">{chapter.title}</span>
            {chapter.caption && <span className="lens-print-contents-caption">{chapter.caption}</span>}
          </li>
        ))}
        {outline.appendix.length > 0 && (
          <li>
            <span className="lens-print-contents-number">A</span>
            <span className="lens-print-contents-title">
              {translate('print.appendix', 'Appendix A. Detailed breakdowns')}
            </span>
          </li>
        )}
        <li>
          <span className="lens-print-contents-number">B</span>
          <span className="lens-print-contents-title">
            {translate('print.method', 'Appendix B. Sources and definitions')}
          </span>
        </li>
      </ol>
      <p className="lens-print-contents-note">{translate(
        'print.note',
        'Every interactive view is expanded below. Values and shares accompany each chart so the report remains self-contained on paper.',
      )}</p>
    </section>
  )
}

function ChapterView({ chapter, title }: { chapter: PrintChapter; title: string }) {
  const figures = chapter.figures
  return (
    <section className="lens-print-chapter">
      <header className="lens-print-chapter-head">
        <p className="lens-print-runninghead">{title} · {chapter.number}. {chapter.title}</p>
        <h2><span className="lens-print-chapter-number">{chapter.number}</span>{chapter.title}</h2>
        {chapter.caption && <p className="lens-print-lead">{chapter.caption}</p>}
      </header>
      <div className="lens-print-grid">
        {figures.map((figure) => <FigureView figure={figure} key={figure.section.id} />)}
      </div>
    </section>
  )
}

function DetailView({ detail }: { detail: PrintDetail }) {
  const panel = sectionPanel(detail.section)
  return (
    <article className="lens-print-detail">
      <header>
        <span className="lens-print-detail-number">{detail.number}</span>
        <h4>{panel.title}</h4>
        <p className="lens-print-detail-trail">{detail.trail}</p>
      </header>
      <PrintDataTable dense section={detail.section} />
    </article>
  )
}

function DetailAppendix({ outline, title }: { outline: PrintOutline; title: string }) {
  const translate = useTranslate()
  if (outline.appendix.length === 0) return null
  return (
    <section className="lens-print-appendix">
      <header className="lens-print-chapter-head">
        <p className="lens-print-runninghead">{title} · {translate('print.appendix', 'Appendix A. Detailed breakdowns')}</p>
        <h2><span className="lens-print-chapter-number">A</span>{translate('print.appendix', 'Appendix A. Detailed breakdowns')}</h2>
        <p className="lens-print-lead">{translate(
          'print.appendixNote',
          'Each reading below is one step of the drill path printed above it, kept for verification rather than for reading in order.',
        )}</p>
      </header>
      {outline.appendix.map((chapter) => (
        <div className="lens-print-appendix-chapter" key={chapter.id}>
          <h3>A{chapter.number}. {chapter.title}</h3>
          <div className="lens-print-grid lens-print-grid-dense">
            {chapter.details.map((detail) => <DetailView detail={detail} key={detail.section.id} />)}
          </div>
        </div>
      ))}
    </section>
  )
}

function Methodology({
  document,
  outline,
  report,
  title,
}: {
  document: DashboardDocument
  outline: PrintOutline
  report: PrintReportModel
  title: string
}) {
  const translate = useTranslate()
  return (
    <section className="lens-print-method">
      <header className="lens-print-chapter-head">
        <p className="lens-print-runninghead">{title} · {translate('print.method', 'Appendix B. Sources and definitions')}</p>
        <h2>
          <span className="lens-print-chapter-number">B</span>
          {translate('print.method', 'Appendix B. Sources and definitions')}
        </h2>
      </header>
      {outline.notes.length > 0 && (
        <dl className="lens-print-notes">
          {outline.notes.map((note) => (
            <div key={note.id}>
              <dt>{note.label}</dt>
              <dd>{note.detail}</dd>
            </div>
          ))}
        </dl>
      )}
      {(outline.missing.length > 0 || report.truncated || report.warnings.length > 0) && (
        <div className="lens-print-limits">
          <h3>{translate('print.limitations', 'Data limitations')}</h3>
          <p>{translate(
            'print.limitationsNote',
            'The report remains usable, but the following detail views could not be calculated and are explicitly disclosed:',
          )}</p>
          <ul>
            {outline.missing.length > 0 && (
              <li>{translate('print.missing', 'Not calculated: {names}', { names: outline.missing.join(', ') })}</li>
            )}
            {report.truncated && (
              <li>{translate(
                'print.timeLimit',
                'Further detail was stopped at the preparation time limit; all sections calculated by then are included.',
              )}</li>
            )}
            {report.warnings.map((warning, index) => <li key={`${index}:${warning}`}>{warning}</li>)}
          </ul>
        </div>
      )}
      <p className="lens-print-provenance">
        {translate('print.snapshot', 'Snapshot')}: {document.snapshotId} · {document.meta.dashboardId} · {document.meta.locale}
      </p>
    </section>
  )
}

/**
 * The printed document itself, driven by a report rather than by runtime state,
 * so a story and a visual-regression run can render exactly what a printer
 * would receive. `PrintReport` is the same document wired to the live print
 * state.
 */
export function PrintReportView({ report }: { report: PrintReportModel }) {
  const translate = useTranslate()
  const document = report.document
  const title = document.header?.title || document.meta.title
  const outline = useMemo(
    () => buildOutline(
      report,
      translate('print.summary', 'Summary'),
      translate('print.other', 'Other readings'),
    ),
    [report, translate],
  )
  return (
    <>
      <Cover document={document} outline={outline} report={report} />
      <Contents outline={outline} />
      {outline.chapters.map((chapter) => <ChapterView chapter={chapter} key={chapter.id} title={title} />)}
      <DetailAppendix outline={outline} title={title} />
      <Methodology document={document} outline={outline} report={report} title={title} />
    </>
  )
}

export function PrintReport() {
  const { document } = useDashboard()
  const print = usePrint()
  if (!print.report || typeof globalThis.document === 'undefined') return null
  // The report is printed in the dashboard's own accent, so a company's board
  // and its report do not disagree about what colour "this matters" is.
  const accent = document.theme.palette.accent ?? document.theme.palette.primary
  return createPortal(
    <article
      aria-hidden={!print.active}
      className="lens-print-report"
      data-preview={print.preview ? 'true' : undefined}
      lang={document.meta.locale}
      style={accent ? ({ '--lens-accent-500': accent } as CSSProperties) : undefined}
    >
      <PrintReportView report={print.report} />
    </article>,
    globalThis.document.body,
  )
}
