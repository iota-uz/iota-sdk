import { createContext, Fragment, useCallback, useContext, useMemo, type CSSProperties } from 'react'
import { createPortal } from 'react-dom'
import type { DashboardDocument, Panel, Theme } from '../contract'
import { buildCascadeStages, buildWaterfallModel } from '../panels/CascadePanel'
import { ChartHost } from '../panels/ChartHost'
import { seriesColorResolver } from '../panels/data'
import { WaterfallPlot } from '../panels/WaterfallPlot'
import {
  clampedDeltaPercent,
  formatAxis,
  formatFieldValue,
  formatFieldValueExact,
  useDashboard,
  usePrint,
  useTranslate,
} from '../runtime'
import type { PrintReport as PrintReportModel, PrintSection } from '../runtime/print'
import { buildChapterFootnotes, type FigureFootnotes } from './footnotes'
import { PrintFormula } from './formula'
import { headlineReadings, narrativeFact } from './narrative'
import { buildOutline, type PrintChapter, type PrintDetail, type PrintFigure, type PrintOutline } from './outline'
import { buildLabelPalette, printTheme } from './palette'
import { PrintQualityChip, qualityDefinitions } from './quality'
import { columnUnit } from './units'
import { chartKinds, formulaKinds, indexOf, numeric, sectionPanel, text, type ChartKind } from './values'

/**
 * The colour every figure in this report gives a given name. Prop-drilling it
 * would thread it through every printed component; a reading's colour is a
 * property of the document, not of the component tree.
 */
const PaletteContext = createContext<Map<string, string>>(new Map())

/** The section's own theme with the report-wide label palette folded in. */
function useSectionTheme(section: PrintSection): Theme {
  const labels = useContext(PaletteContext)
  return useMemo(() => printTheme(section.document.theme, labels), [labels, section.document.theme])
}

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

interface AuditTable {
  rows: Array<AuditRow>
  /** The unit every value in the column shares, said once in its head. */
  unit?: string
}

function auditRows(section: PrintSection, locale: string, theme: Theme): AuditTable {
  const frame = section.frame
  if (!frame) return { rows: [] }
  const panel = sectionPanel(section)
  const labelIndex = indexOf(frame, panel.encoding.label ?? panel.encoding.category ?? panel.encoding.id)
  const valueIndex = indexOf(frame, panel.encoding.value)
  const seriesIndex = indexOf(frame, panel.encoding.series)
  const idIndex = indexOf(frame, panel.encoding.id)
  const valueField = panel.encoding.value
  const valueFormat = valueField ? panel.format[valueField] : undefined
  const values = frame.rows.map((row) => numeric(row[valueIndex]))
  const groups = new Map<string, number>()
  // A series column is how a chart separates its rings; its values are only
  // reader-facing when the dashboard declared a format for them. Printing the
  // undeclared ones put the internal keys «recognition ·» and «payment ·» in
  // front of every label — the table below the rings names them already.
  const seriesFormat = panel.encoding.series ? panel.format[panel.encoding.series] : undefined

  frame.rows.forEach((row, rowIndex) => {
    const group = seriesIndex >= 0 ? text(row[seriesIndex]) : ''
    const value = values[rowIndex]
    if (value !== undefined && value >= 0) groups.set(group, (groups.get(group) ?? 0) + value)
  })

  // A bridge frame carries the running total in its value column. The chart
  // above the table draws the movement between stages, so a table of running
  // totals contradicts it row for row — an outward cession drawn as −14.68 bn
  // printed as 185.21 bn beneath it. The stages are what a bridge is about, so
  // the table states the same movement the bars do, computed the same way; the
  // opening and closing rows keep their totals.
  const bridge = panel.semantics === 'reconciliation' && indexOf(frame, panel.encoding.cut) >= 0
  const finalIndex = indexOf(frame, panel.encoding.final)
  const stepValue = (row: Array<unknown>, rowIndex: number): unknown => {
    const raw = valueIndex >= 0 ? row[valueIndex] : undefined
    if (!bridge || rowIndex === 0) return raw
    if (finalIndex >= 0 && row[finalIndex] === true) return raw
    const current = values[rowIndex]
    const previous = values[rowIndex - 1]
    if (current === undefined || previous === undefined) return raw
    return current - previous
  }

  const resolveColor = seriesColorResolver(theme, panel, { positional: section.root })
  // The value column is stated in one unit, so a bridge step and a portfolio
  // total are read against each other rather than digit by digit.
  const unit = columnUnit(frame.rows.map((row, rowIndex) => stepValue(row, rowIndex)), valueFormat, locale)
  return { unit: unit.note, rows: frame.rows.map((row, rowIndex) => {
    const rawValue = stepValue(row, rowIndex)
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
      ...(group && seriesFormat
        ? { series: formatFieldValue(row[seriesIndex], seriesFormat, locale) }
        : {}),
      label,
      value: valueIndex >= 0 ? unit.format(rawValue) : '—',
      exact: valueIndex >= 0 ? formatFieldValueExact(rawValue, valueFormat, locale) : undefined,
      ...(ratio === undefined ? {} : {
        ratio,
        // A row worth 1.5 million printed as «0,0 %» reads as nothing at all.
        // Below the resolution of the column, the share says it is below it.
        share: ratio > 0 && ratio < 0.001
          ? `< ${new Intl.NumberFormat(locale, { style: 'percent', minimumFractionDigits: 1 }).format(0.001)}`
          : new Intl.NumberFormat(locale, {
            style: 'percent',
            minimumFractionDigits: 1,
            maximumFractionDigits: 1,
          }).format(ratio),
      }),
      color: resolveColor(label, rowIndex),
    }
  }) }
}

function PrintChart({ section, height }: { section: PrintSection; height: number }) {
  const panel = useMemo(() => sectionPanel(section), [section])
  const theme = useSectionTheme(section)
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
          locale: section.document.meta.locale,
          theme,
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
function useWaterfallModel(section: PrintSection) {
  const panel = sectionPanel(section)
  const locale = section.document.meta.locale
  return useMemo(() => {
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
}

function PrintWaterfall({ section }: { section: PrintSection }) {
  const translate = useTranslate()
  const panel = sectionPanel(section)
  const model = useWaterfallModel(section)
  const locale = section.document.meta.locale
  // Paper reads an axis differently from a screen: eight gridlines each
  // spelling «175.00 млрд UZS» is the unit said eight times and a precision
  // nothing needs. The scale keeps its top tick in full and states the rest
  // as compact numbers.
  const printed = useMemo(() => {
    if (!model) return undefined
    const keep = model.ticks.length > 5
      ? model.ticks.filter((_, index) => index % 2 === 0)
      : model.ticks
    const compact = new Intl.NumberFormat(locale, { notation: 'compact', maximumFractionDigits: 1 })
    return {
      ...model,
      ticks: keep.map((tick, index) => (
        index === 0 ? tick : { ...tick, label: compact.format(tick.value) }
      )),
    }
  }, [locale, model])
  // A colour that means something has to say what it means. The bridge tints a
  // stage by what the movement is worth to the reader, not by its direction, so
  // ink alone leaves «green among the orange» unexplained.
  const tones = useMemo(
    () => Array.from(new Set((printed?.items ?? []).map((item) => item.tone).filter(Boolean))) as Array<string>,
    [printed],
  )
  if (!printed || printed.items.length === 0) return null
  return (
    <div className="lens-print-chart lens-print-chart-waterfall">
      <WaterfallPlot label={panel.title} model={printed} />
      {tones.length > 1 && (
        <p className="lens-print-tone-key">
          {tones.map((tone) => (
            <span key={tone}>
              <i aria-hidden="true" data-tone={tone} />
              {tone === 'positive'
                ? translate('print.toneFavourable', 'favourable')
                : tone === 'negative'
                  ? translate('print.toneAdverse', 'adverse')
                  : translate('print.toneNeutral', 'neutral')}
            </span>
          ))}
        </p>
      )}
    </div>
  )
}

/**
 * A bridge's evidence is the bridge: the same stages, in the same order, with
 * the same signs the plot draws. Printing the backing frame instead lets the
 * table disagree with the picture above it — a stage the plot splits out goes
 * missing, and an intermediate keeps a name the axis never shows.
 */
function PrintWaterfallTable({ section }: { section: PrintSection }) {
  const translate = useTranslate()
  const panel = sectionPanel(section)
  const model = useWaterfallModel(section)
  // The unit is said once in the column head, as every other printed table
  // says it: six repetitions of «млрд UZS» down a column are six repetitions.
  const unit = useMemo(
    () => columnUnit(
      (model?.items ?? []).map((item) => item.value),
      panel.encoding.value ? panel.format[panel.encoding.value] : undefined,
      section.document.meta.locale,
    ),
    [model, panel, section.document.meta.locale],
  )
  if (!model || model.items.length === 0) return null
  return (
    <table className="lens-print-data">
      <thead>
        <tr>
          <th>{translate('print.stage', 'Stage')}</th>
          <th data-align="right">{translate('print.value', 'Value')}{unit.note && <small>{unit.note}</small>}</th>
        </tr>
      </thead>
      <tbody>
        {model.items.map((item, index) => (
          <Fragment key={`${item.label}:${index}`}>
            <tr data-role={item.kind}>
              <td>{item.label}</td>
              <td data-align="right">{unit.format(item.value)}</td>
            </tr>
            {item.formattedSplit && (
              <tr data-role="split">
                <td>{item.splitLabel || translate('print.splitPart', 'of which')}</td>
                <td data-align="right">{item.formattedSplit}</td>
              </tr>
            )}
          </Fragment>
        ))}
      </tbody>
    </table>
  )
}

/**
 * Every bridge prints as a bridge. On screen a cascade without the waterfall
 * hint renders as stacked stage rows, which paper has no room for; the same
 * stages drawn as columns say the same thing in a third of the space, and the
 * evidence table underneath keeps the exact figures.
 */
function isWaterfall(panel: Panel, section: PrintSection): boolean {
  return panel.kind === 'cascade' && Boolean(section.frame) && indexOf(section.frame!, panel.encoding.cut) >= 0
}

function PrintDataTable({ section, dense, exact: withExact }: {
  section: PrintSection
  dense?: boolean
  /**
   * Print the full-precision figure under every abbreviated one. It doubles the
   * height of a table, so a chapter states the reading and the appendix — the
   * part kept for checking — states it to the som.
   */
  exact?: boolean
}) {
  const translate = useTranslate()
  const theme = useSectionTheme(section)
  const rows = useMemo(
    () => auditRows(section, section.document.meta.locale, theme),
    [section, theme],
  )
  if (!section.frame) return <p className="lens-print-empty">{translate('print.noData', 'No data')}</p>
  const panel = sectionPanel(section)
  if (panel.kind === 'table' && panel.columns?.length) {
    const frame = section.frame
    const columns = panel.columns
    const indexes = columns.map(({ field }) => indexOf(frame, field))
    const locale = section.document.meta.locale
    // Each column says its unit once, in its head.
    const units = columns.map((column, columnIndex) => {
      const index = indexes[columnIndex]!
      return columnUnit(
        index >= 0 ? frame.rows.map((row) => row[index]) : [],
        panel.format[column.field],
        locale,
      )
    })
    // A column of numbers is read down its digits: the printed table aligns a
    // money, count or percentage column to the right unless the panel says
    // otherwise, so the magnitudes stack instead of ragging.
    const alignments = columns.map((column) => {
      if (column.align) return column.align
      const kind = panel.format[column.field]?.kind
      return kind === 'money' || kind === 'number' || kind === 'percent' ? 'right' : 'left'
    })
    // A bar cell is drawn against the largest magnitude in its own column, the
    // same scale the dashboard uses, so a printed row keeps the proportion the
    // reader saw on screen.
    const maxima = new Map<string, number>()
    columns.forEach((column, columnIndex) => {
      if (column.cell.kind !== 'bar') return
      const index = indexes[columnIndex]!
      if (index < 0) return
      maxima.set(column.field, frame.rows.reduce((largest, row) => {
        const value = numeric(row[index])
        return value === undefined ? largest : Math.max(largest, Math.abs(value))
      }, 0))
    })
    return (
      <table className="lens-print-data lens-print-data-wide">
        <thead>
          <tr>
            {columns.map((column, columnIndex) => (
              <th data-align={alignments[columnIndex]} key={column.field}>
                {column.label}
                {units[columnIndex]?.note && <small>{units[columnIndex]?.note}</small>}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {frame.rows.map((row, rowIndex) => (
            <tr key={rowIndex}>
              {columns.map((column, columnIndex) => {
                const raw = indexes[columnIndex]! >= 0 ? row[indexes[columnIndex]!] : undefined
                const formatted = units[columnIndex]!.format(raw)
                const exact = withExact
                  ? formatFieldValueExact(raw, panel.format[column.field], locale)
                  : undefined
                const value = numeric(raw)
                // The producer's own row verdict — a loss ratio past 100%, say.
                // On screen it tints the value; ink can carry the same tint.
                const tone = text(row[indexOf(frame, column.cell.toneField)])
                const max = maxima.get(column.field)
                // A `delta` column carries two readings in one cell: the amount
                // and the percentage it moved. Printing only the amount lost the
                // half that says whether the move was large.
                const secondaryIndex = indexOf(frame, column.cell.secondaryField)
                const secondary = secondaryIndex >= 0 ? numeric(row[secondaryIndex]) : undefined
                return (
                  <td
                    data-align={alignments[columnIndex]}
                    data-negative={value !== undefined && value < 0 ? '' : undefined}
                    data-tone={tone === 'pos' || tone === 'warn' || tone === 'neg' ? tone : undefined}
                    key={column.field}
                  >
                    {formatted}
                    {secondary !== undefined && (
                      <em className="lens-print-cell-secondary" data-negative={secondary < 0 ? '' : undefined}>
                        {clampedDeltaPercent(secondary) ?? `${secondary > 0 ? '+' : ''}${formatFieldValue(
                          row[secondaryIndex],
                          column.cell.secondaryField ? panel.format[column.cell.secondaryField] : undefined,
                          locale,
                        )}`}
                      </em>
                    )}
                    {max !== undefined && max > 0 && value !== undefined && (
                      <span
                        aria-hidden="true"
                        className="lens-print-cell-bar"
                        data-negative={value < 0 ? '' : undefined}
                        style={{ width: `${Math.min(100, Math.round((Math.abs(value) / max) * 100))}%` }}
                      />
                    )}
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
  const shares = rows.rows.some(({ share }) => share !== undefined)
  return (
    <table className={`lens-print-data${dense ? ' lens-print-data-dense' : ''}`}>
      <thead>
        <tr>
          <th>{translate('print.category', 'Category')}</th>
          <th data-align="right">
            {translate('print.value', 'Value')}
            {rows.unit && <small>{rows.unit}</small>}
          </th>
          {shares && <th data-align="right">{translate('print.share', 'Share')}</th>}
        </tr>
      </thead>
      <tbody>
        {rows.rows.map((row) => (
          <tr key={row.key}>
            <td>
              <span className="lens-print-swatch" style={{ backgroundColor: row.color }} />
              {row.series && <span className="lens-print-series">{row.series} · </span>}
              {row.label}
            </td>
            <td data-align="right">
              {row.value}
              {withExact && row.exact && row.exact !== row.value && <small>{row.exact}</small>}
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

/** What a printed table stops short of: one page of a level that has more. */
function TruncationNote({ section }: { section: PrintSection }) {
  const translate = useTranslate()
  if (!section.hasMore || !section.frame) return null
  return (
    <p className="lens-print-truncated">
      {translate('print.truncated', 'First {rows} rows shown; the level continues in the dashboard.', {
        rows: section.frame.rows.length,
      })}
    </p>
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

/** The markers a figure carries into the page's footnotes. */
function FootnoteMarkers({ footnotes }: { footnotes?: FigureFootnotes }) {
  if (!footnotes || footnotes.markers.length === 0) return null
  return <sup className="lens-print-footnote-marker">{footnotes.markers.join(', ')}</sup>
}

/** The notes this figure introduced, printed with it so they share its page. */
function FootnoteTexts({ footnotes }: { footnotes?: FigureFootnotes }) {
  if (!footnotes || footnotes.notes.length === 0) return null
  return (
    <ol className="lens-print-footnotes">
      {footnotes.notes.map((note) => (
        // The number is drawn rather than left to the list marker: a marker is
        // the one glyph a print engine feels free to drop, and a note nobody
        // can tie back to its figure is a note nobody reads.
        <li key={note.number} value={note.number}>
          <span className="lens-print-footnote-index">{note.number}</span>
          {note.text}
        </li>
      ))}
    </ol>
  )
}

/**
 * What the dashboard shows when the reading above is clicked. A one-row level
 * is a term of the calculation and prints as one line; anything wider keeps its
 * table. Printed here, the ratio and its parts are read together.
 */
const breakdownRowCap = 8

function BreakdownView({ sections }: { sections: Array<PrintSection> }) {
  const translate = useTranslate()
  if (sections.length === 0) return null
  return (
    <div className="lens-print-breakdown">
      <p className="lens-print-breakdown-head">{translate('print.breakdown', 'How it is calculated')}</p>
      {sections.map((section) => {
        const panel = sectionPanel(section)
        const frame = section.frame
        // A level of one column carries no reading — on screen it is a row of
        // links into the claim register, on paper it is the word «Открыть
        // претензии» under a heading.
        const printedColumns = panel.kind === 'table' && panel.columns?.length
          ? panel.columns.length
          : frame?.columns.length ?? 0
        if (frame && printedColumns <= 1) return null
        if (formulaKinds.has(panel.kind)) {
          return <PrintFormula key={section.id} section={section} />
        }
        const valueIndex = frame ? indexOf(frame, panel.encoding.value) : -1
        if (frame && frame.rows.length === 1 && valueIndex >= 0) {
          return (
            <p className="lens-print-breakdown-term" key={section.id}>
              <span>{panel.title}</span>
              <span>{formatFieldValue(
                frame.rows[0]?.[valueIndex],
                panel.encoding.value ? panel.format[panel.encoding.value] : undefined,
                section.document.meta.locale,
              )}</span>
            </p>
          )
        }
        // A term of a calculation is a handful of rows. Anything longer is a
        // dataset that belongs in the appendix, so the tile keeps the head of
        // it and says what it kept.
        const capped = frame && frame.rows.length > breakdownRowCap
        const shown = capped && frame
          ? { ...section, frame: { ...frame, rows: frame.rows.slice(0, breakdownRowCap) } }
          : section
        // A level whose first column is already named after it — «Общий резерв
        // по группам риска» over a column of the same name — needs the heading
        // said once.
        // Two cuts of one number — by product and by claim size — carry the
        // panel's name twice and their own name nowhere. The cut is what tells
        // the two tables apart, so it is what the part is called.
        const title = section.perspective?.label.trim() || panel.title
        const named = panel.columns?.[0]?.label?.trim().toLowerCase() === title.trim().toLowerCase()
        return (
          <div className="lens-print-breakdown-part" key={section.id}>
            {!named && <p className="lens-print-breakdown-title">{title}</p>}
            <PrintDataTable dense section={shown} />
            {capped && frame && (
              // Here the whole term is in hand, so the reader is told what
              // share of it the page keeps rather than merely that it was cut.
              <p className="lens-print-truncated">
                {translate('print.truncatedOf', '{rows} of {total} rows shown.', {
                  rows: breakdownRowCap,
                  total: frame.rows.length,
                })}
              </p>
            )}
          </div>
        )
      })}
    </div>
  )
}

function FigureView({ figure, footnotes }: { figure: PrintFigure; footnotes?: FigureFootnotes }) {
  const translate = useTranslate()
  const { section } = figure
  const panel = sectionPanel(section)
  const waterfall = isWaterfall(panel, section)
  const formula = formulaKinds.has(panel.kind)
  // A single value needs neither a chart of one point nor a table of one row.
  if (figure.metric) {
    return (
      <figure className={`lens-print-figure lens-print-figure-${figure.width} lens-print-figure-metric`}>
        <MetricTile figure={figure} footnotes={footnotes} numbered />
        <BreakdownView sections={figure.breakdown} />
        <FootnoteTexts footnotes={footnotes} />
      </figure>
    )
  }
  const total = panel.total !== undefined && panel.encoding.value
    ? formatFieldValue(panel.total, panel.format[panel.encoding.value], section.document.meta.locale)
    : undefined
  return (
    <figure className={`lens-print-figure lens-print-figure-${waterfall || formula ? 'full' : figure.width}`}>
      {/* Two rows, always both: a title that runs long must not push the
          chips down and take the figure beside it out of line. */}
      <figcaption className="lens-print-figure-head">
        <p className="lens-print-figure-title">
          <span className="lens-print-figure-number">
            {translate('print.figure', 'Fig. {number}', { number: figure.number })}
          </span>
          <h3>{panel.title}<FootnoteMarkers footnotes={footnotes} /></h3>
        </p>
        <p className="lens-print-figure-meta">
          {panel.status?.label && (
            <span className="lens-print-chip" data-tone={panel.status.tone ?? 'neutral'}>{panel.status.label}</span>
          )}
          <PrintQualityChip availability={panel.availability} confidence={panel.confidence} />
          {section.perspective && (
            <span className="lens-print-chip" data-tone="neutral">
              {translate('print.view', 'View')}: {section.perspective.label}
            </span>
          )}
          {/* The header badge the dashboard shows: the authoritative total the
              shares below are taken against. */}
          {total && <span className="lens-print-figure-total">{translate('print.total', 'Total')}: {total}</span>}
        </p>
      </figcaption>
      {formula
        ? <PrintFormula section={section} />
        : waterfall
          ? <PrintWaterfall section={section} />
          : figure.chart && <PrintChart height={figure.width === 'half' ? 225 : 235} section={section} />}
      <FigureNote section={section} />
      {!formula && (waterfall ? <PrintWaterfallTable section={section} /> : <PrintDataTable section={section} />)}
      <TruncationNote section={section} />
      <BreakdownView sections={figure.breakdown} />
      <FootnoteTexts footnotes={footnotes} />
    </figure>
  )
}

/**
 * A trend line the way the dashboard draws it beside a stat: no axis, no
 * gridlines, just the shape of the last few periods. Eight quarters of history
 * cost one line of ink and answer the first question a reader has of any single
 * number — whether it is going anywhere.
 */
function PrintSparkline({ values }: { values: Array<number> }) {
  const points = values.filter((value) => Number.isFinite(value))
  if (points.length < 2) return null
  const min = Math.min(...points)
  const max = Math.max(...points)
  const span = max - min || 1
  const step = 100 / (points.length - 1)
  const path = points
    .map((value, index) => `${(index * step).toFixed(2)},${(24 - ((value - min) / span) * 22).toFixed(2)}`)
    .join(' ')
  return (
    <svg aria-hidden="true" className="lens-print-sparkline" preserveAspectRatio="none" viewBox="0 0 100 26">
      <polyline fill="none" points={path} strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

/** One reading: its name, its number, where it came from and where it is going. */
function MetricTile({ figure, footnotes, numbered }: {
  figure: PrintFigure
  footnotes?: FigureFootnotes
  /** Chapter tiles are numbered so the text can point at them; cover tiles are not. */
  numbered?: boolean
}) {
  const translate = useTranslate()
  const { section } = figure
  const panel = sectionPanel(section)
  const locale = section.document.meta.locale
  const frame = section.frame
  const valueIndex = frame ? indexOf(frame, panel.encoding.value) : -1
  const raw = frame && valueIndex >= 0 ? frame.rows[0]?.[valueIndex] : undefined
  const format = panel.encoding.value ? panel.format[panel.encoding.value] : undefined
  // A stat carries its change against the previous period in the `final` slot —
  // the same cell the dashboard prints as a delta chip beside the value.
  const deltaIndex = frame ? indexOf(frame, panel.encoding.final) : -1
  const deltaRaw = frame && deltaIndex >= 0 ? frame.rows[0]?.[deltaIndex] : undefined
  const delta = numeric(deltaRaw)
  const deltaFormat = panel.encoding.final ? panel.format[panel.encoding.final] : format
  const target = panel.target
  return (
    <div className="lens-print-kpi">
      <p className="lens-print-kpi-label">
        {numbered && (
          <span className="lens-print-figure-number">
            {translate('print.figure', 'Fig. {number}', { number: figure.number })}
          </span>
        )}
        {panel.title}
        <FootnoteMarkers footnotes={footnotes} />
      </p>
      <p className="lens-print-kpi-value">
        {formatFieldValue(raw, format, locale)}
        {delta !== undefined && (
          <span className="lens-print-kpi-delta" data-negative={delta < 0 ? '' : undefined}>
            {delta > 0 ? '+' : ''}{formatFieldValue(deltaRaw, deltaFormat, locale)}
          </span>
        )}
      </p>
      {panel.sparkline && <PrintSparkline values={panel.sparkline.values} />}
      {panel.trend && (
        <p className="lens-print-kpi-trend" data-negative={panel.trend.percent < 0 ? '' : undefined}>
          {panel.trend.percent > 0 ? '+' : ''}{clampedDeltaPercent(panel.trend.percent)
            ?? `${new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(panel.trend.percent)}%`}
          {panel.trend.label ? ` ${panel.trend.label}` : ''}
        </p>
      )}
      {target && (
        <p className="lens-print-kpi-target">
          {translate('print.target', 'Target')}: {formatFieldValue(target.value, format, locale)}
          {target.label ? ` · ${target.label}` : ''}
        </p>
      )}
      {panel.caption && <p className="lens-print-kpi-caption">{panel.caption}</p>}
      <p className="lens-print-kpi-chips">
        {panel.status?.label && (
          <span className="lens-print-chip" data-tone={panel.status.tone ?? 'neutral'}>{panel.status.label}</span>
        )}
        <PrintQualityChip availability={panel.availability} confidence={panel.confidence} />
      </p>
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
        {(outline.estimated > 0 || outline.missing.length > 0) && (
          <div>
            <dt>{translate('print.quality', 'Data quality')}</dt>
            <dd>{translate('print.qualityCount', '{estimated} estimated · {missing} not calculated', {
              estimated: outline.estimated,
              missing: outline.missing.length,
            })}</dd>
          </div>
        )}
      </dl>
      {kpis.length > 0 && (
        <div className="lens-print-kpis">
          {kpis.map((figure) => <MetricTile figure={figure} key={figure.section.id} />)}
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
        {outline.chapters.filter(({ figures }) => figures.length > 0).map((chapter) => (
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
  const translate = useTranslate()
  // The chapter's own numbers, said once before the figures that carry them.
  const headline = useMemo(
    () => headlineReadings(
      chapter.figures.map((figure) => ({
        id: figure.section.id,
        panel: sectionPanel(figure.section),
        frame: figure.section.frame,
      })),
      chapter.figures[0]?.section.document.meta.locale ?? 'en',
    ),
    [chapter.figures],
  )
  // Footnotes are numbered per chapter and printed where they first apply, so a
  // reader never has to hold a number across a page break.
  const footnotes = buildChapterFootnotes(chapter.figures, translate)
  // A chapter can hold more than one authored strip of metrics — two bases for
  // the same ratios, say. Each strip announces itself where it begins, so two
  // readings called the same thing are never left to be told apart by order.
  // A chapter whose readings are all drill detail has nothing to show here;
  // its numbers are printed, once, in the detail appendix.
  if (chapter.figures.length === 0) return null
  const printed = new Set<string>([chapter.caption ?? ''])
  const body: Array<JSX.Element> = []
  // Half-width readings are paired explicitly rather than left to wrap. In a
  // printed chapter the figures flow as blocks — a wrapping run of inline
  // halves inherits the gutter of the pair it broke away from and walks down
  // the page in a staircase.
  let pending: PrintFigure | undefined
  // A strip's heading is bound to the first reading under it: `break-after:
  // avoid` alone still left «По премии за период» as the last line of a page
  // with its readings overleaf.
  let lead: JSX.Element | undefined
  const render = (figure: PrintFigure) => (
    <FigureView figure={figure} footnotes={footnotes.get(figure.section.id)} key={figure.section.id} />
  )
  const push = (element: JSX.Element) => {
    if (!lead) {
      body.push(element)
      return
    }
    const heading = lead
    lead = undefined
    body.push(
      <div className="lens-print-lead-group" key={`lead-${element.key}`}>
        {heading}
        {element}
      </div>,
    )
  }
  const flush = () => {
    if (!pending) return
    push(render(pending))
    pending = undefined
  }
  const place = (figure: PrintFigure) => {
    if (figure.width !== 'half') {
      flush()
      push(render(figure))
      return
    }
    if (!pending) {
      pending = figure
      return
    }
    const left = pending
    pending = undefined
    push(
      <div className="lens-print-pair" key={`pair-${left.section.id}`}>
        {render(left)}
        {render(figure)}
      </div>,
    )
  }
  for (const figure of chapter.figures) {
    const group = figure.group
    const key = `${group?.label ?? ''}::${group?.caption ?? ''}`
    if (group && !printed.has(key)) {
      printed.add(key)
      // A strip starts its own run of readings; it never shares a row with the
      // pair that preceded it.
      flush()
      lead = (
        <div className="lens-print-strip" key={`strip-${figure.section.id}`}>
          {group.label && <h3>{group.label}</h3>}
          {group.caption && group.caption !== chapter.caption && <p>{group.caption}</p>}
        </div>
      )
    }
    place(figure)
  }
  flush()
  // A strip that named nothing — every reading under it was lifted to the
  // cover — still announces itself rather than vanishing.
  if (lead) body.push(lead)
  return (
    <section className="lens-print-chapter">
      <header className="lens-print-chapter-head">
        <p className="lens-print-runninghead">{title} · {chapter.number}. {chapter.title}</p>
        <h2><span className="lens-print-chapter-number">{chapter.number}</span>{chapter.title}</h2>
        {chapter.caption && <p className="lens-print-lead">{chapter.caption}</p>}
        {headline.length > 0 && (
          <p className="lens-print-chapter-headline">
            {headline.map((reading) => (
              <span key={reading.id}>
                <span className="lens-print-chapter-headline-label">{reading.label}</span>
                {reading.value}
              </span>
            ))}
          </p>
        )}
      </header>
      <div className="lens-print-grid">{body}</div>
    </section>
  )
}

/** Beyond this many points a printed series is a shape, not a list. */
const seriesTableLimit = 12
/** How many periods stay in the table when the shape carries the rest. */
const seriesTableTail = 8

function DetailView({ detail }: { detail: PrintDetail }) {
  const translate = useTranslate()
  const panel = sectionPanel(detail.section)
  const rows = detail.section.frame?.rows.length ?? 0
  // A quarterly series back to 2011 is sixty rows of numbers nobody reads and a
  // trend nobody can see. The whole run prints as a line; the table keeps the
  // recent periods, and says so.
  const compact = panel.semantics === 'series' && rows > seriesTableLimit
  const section = compact && detail.section.frame
    ? {
        ...detail.section,
        frame: { ...detail.section.frame, rows: detail.section.frame.rows.slice(-seriesTableTail) },
      }
    : detail.section
  return (
    <article className="lens-print-detail">
      <header>
        <span className="lens-print-detail-number">{detail.number}</span>
        <h4>{panel.title}</h4>
        <p className="lens-print-detail-trail">{detail.trail}</p>
      </header>
      {compact && <PrintChart height={110} section={detail.section} />}
      <PrintDataTable dense exact section={section} />
      <TruncationNote section={detail.section} />
      {compact && (
        <p className="lens-print-detail-note">
          {translate('print.seriesTail', 'The chart carries all {rows} periods; the table keeps the most recent {kept}.', {
            rows,
            kept: seriesTableTail,
          })}
        </p>
      )}
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
  const definitions = useMemo(
    () => qualityDefinitions(outline.qualities, translate),
    [outline.qualities, translate],
  )
  return (
    <section className="lens-print-method">
      <header className="lens-print-chapter-head">
        <p className="lens-print-runninghead">{title} · {translate('print.method', 'Appendix B. Sources and definitions')}</p>
        <h2>
          <span className="lens-print-chapter-number">B</span>
          {translate('print.method', 'Appendix B. Sources and definitions')}
        </h2>
      </header>
      {definitions.length > 0 && (
        <div className="lens-print-glossary">
          <h3>{translate('print.qualityTerms', 'What the quality marks mean')}</h3>
          <dl>
            {definitions.map((entry) => (
              <div key={entry.value}>
                <dt><span className="lens-print-chip" data-quality={entry.value}>{entry.label}</span></dt>
                <dd>{entry.definition}</dd>
              </div>
            ))}
          </dl>
        </div>
      )}
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
  const labels = useMemo(() => buildLabelPalette(report), [report])
  const outline = useMemo(
    () => buildOutline(
      report,
      translate('print.summary', 'Summary'),
      translate('print.other', 'Other readings'),
    ),
    [report, translate],
  )
  return (
    <PaletteContext.Provider value={labels}>
      <Cover document={document} outline={outline} report={report} />
      <Contents outline={outline} />
      {outline.chapters.map((chapter) => <ChapterView chapter={chapter} key={chapter.id} title={title} />)}
      <DetailAppendix outline={outline} title={title} />
      <Methodology document={document} outline={outline} report={report} title={title} />
    </PaletteContext.Provider>
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
