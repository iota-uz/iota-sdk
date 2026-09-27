import { createContext, createMemo, useContext, For, Show, type JSX } from 'solid-js'
import { Portal } from 'solid-js/web'
import type { DashboardDocument, Panel, Theme } from '../contract'
import { buildCascadeStages, buildWaterfallModel } from '../panels/CascadePanel'
import { ChartHost } from '../panels/ChartHost'
import { fallbackMarkKey, markCellText } from '../charts/keys'
import { colorLabels, frameSeriesColorResolver, rowColorResolver } from '../panels/data'
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
  return createMemo(() => printTheme(section.document.theme, labels))()
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
  // A partition ring declares the whole its parts are measured against, and a
  // tolerance says the rows may miss it slightly. The chart divides by the
  // declared total; a table dividing by the row sum prints a different share
  // for the same arc.
  for (const ring of panel.radial?.mode === 'partition' ? panel.radial.rings ?? [] : []) {
    if (groups.has(ring.key)) groups.set(ring.key, ring.total)
  }

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

  const categoryIndex = indexOf(frame, panel.encoding.category)
  const rowMarks = panel.kind === 'pie' || panel.kind === 'donut' || panel.kind === 'radial'
  const categoryKeys = frame.rows.map(row => {
    const category = markCellText(row[categoryIndex >= 0 ? categoryIndex : labelIndex])
    const id = idIndex >= 0 ? markCellText(row[idIndex]) : ''
    const series = seriesIndex >= 0 ? markCellText(row[seriesIndex]) : ''
    return { category, nodeKey: id || fallbackMarkKey(category, panel.radial?.mode === 'partition' ? '' : series) }
  })
  const categoryOrder = new Map<string, number>()
  categoryKeys.forEach(({ nodeKey, category }) => {
    const key = nodeKey ?? category
    if (!categoryOrder.has(key)) categoryOrder.set(key, categoryOrder.size)
  })
  const resolveSeriesColor = frameSeriesColorResolver(theme, panel, frame, section.root)
  const resolveRowColor = rowColorResolver(theme, panel, { colors: frame.colors, positional: section.root, labels: colorLabels(frame, panel) })
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
    const { category, nodeKey } = categoryKeys[rowIndex]!
    const colorIndex = panel.radial?.mode === 'partition' ? categoryOrder.get(nodeKey ?? category)! : rowIndex
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
      color: rowMarks
        ? resolveRowColor(category, colorIndex, nodeKey)
        : seriesIndex >= 0 ? resolveSeriesColor(group, rowIndex) : resolveRowColor(label, rowIndex),
    }
  }) }
}

function PrintChart(props: { section: PrintSection; height: number }): JSX.Element | null {
  const panel = createMemo(() => sectionPanel(props.section))()
  const theme = useSectionTheme(props.section)
  const format = (field: string, value: unknown) => formatFieldValue(value, panel.format[field], props.section.document.meta.locale)
  const formatChartAxis = (field: string, value: unknown) => formatAxis(value, panel.format[field], props.section.document.meta.locale)
  const section = props.section
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
    <div class="lens-print-chart" style={{ height: `${props.height}px` }}>
      <ChartHost
        input={{
          kind: panel.kind as ChartKind,
          frame: section.frame,
          encoding: panel.encoding,
          seriesColor: frameSeriesColorResolver(theme, panel, section.frame, section.root),
          rowColor: rowColorResolver(theme, panel, { colors: section.frame.colors, positional: section.root, labels: colorLabels(section.frame, panel) }),
          labels: {
            noData: section.document.i18n['panel.empty'] ?? 'No data',
            current: section.document.i18n['chart.series.current'] ?? 'Current period',
            previous: section.document.i18n['chart.series.previous'] ?? 'Previous',
          },
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
  return createMemo(() => {
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
  })()
}

function PrintWaterfall(props: { section: PrintSection }): JSX.Element | null {
  const translate = useTranslate()
  const panel = sectionPanel(props.section)
  const model = useWaterfallModel(props.section)
  const locale = props.section.document.meta.locale
  // Paper reads an axis differently from a screen: eight gridlines each
  // spelling «175.00 млрд UZS» is the unit said eight times and a precision
  // nothing needs. The scale keeps its top tick in full and states the rest
  // as compact numbers.
  const printed = createMemo(() => {
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
  })
  // A colour that means something has to say what it means. The bridge tints a
  // stage by what the movement is worth to the reader, not by its direction, so
  // ink alone leaves «green among the orange» unexplained.
  const tones = createMemo(
    () => Array.from(new Set((printed()?.items ?? []).map((item) => item.tone).filter(Boolean))) as Array<string>,
  )
  return (
    <Show when={printed() && printed()!.items.length > 0}>
      <div class="lens-print-chart lens-print-chart-waterfall">
        {/* Paper cannot be hovered, so every split names itself here. */}
        <WaterfallPlot label={panel.title} model={printed()!} splitCallout="always" />
        <Show when={tones().length > 1}>
          <p class="lens-print-tone-key">
            <For each={tones()}>
              {(tone) => (
                <span>
                  <i aria-hidden="true" data-tone={tone} />
                  {tone === 'positive'
                    ? translate('print.toneFavourable', 'favourable')
                    : tone === 'negative'
                      ? translate('print.toneAdverse', 'adverse')
                      : translate('print.toneNeutral', 'neutral')}
                </span>
              )}
            </For>
          </p>
        </Show>
      </div>
    </Show>
  )
}

/**
 * A bridge's evidence is the bridge: the same stages, in the same order, with
 * the same signs the plot draws. Printing the backing frame instead lets the
 * table disagree with the picture above it — a stage the plot splits out goes
 * missing, and an intermediate keeps a name the axis never shows.
 */
function PrintWaterfallTable(props: { section: PrintSection }): JSX.Element | null {
  const translate = useTranslate()
  const panel = sectionPanel(props.section)
  const model = useWaterfallModel(props.section)
  // The unit is said once in the column head, as every other printed table
  // says it: six repetitions of «млрд UZS» down a column are six repetitions.
  const unit = createMemo(
    () => columnUnit(
      (model?.items ?? []).map((item) => item.value),
      panel.encoding.value ? panel.format[panel.encoding.value] : undefined,
      props.section.document.meta.locale,
    ),
  )
  return (
    <Show when={model && model.items.length > 0}>
      <table class="lens-print-data">
        <thead>
          <tr>
            <th>{translate('print.stage', 'Stage')}</th>
            <th data-align="right">{translate('print.value', 'Value')}{unit().note && <small>{unit().note}</small>}</th>
          </tr>
        </thead>
        <tbody>
          <For each={model!.items}>
            {(item) => (
              <>
                <tr data-role={item.kind}>
                  <td>{item.label}</td>
                  <td data-align="right">{unit().format(item.value)}</td>
                </tr>
                <Show when={item.formattedSplit}>
                  <tr data-role="split">
                    <td>{item.splitLabel || translate('print.splitPart', 'of which')}</td>
                    <td data-align="right">{item.formattedSplit}</td>
                  </tr>
                </Show>
              </>
            )}
          </For>
        </tbody>
      </table>
    </Show>
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

function PrintDataTable(props: {
  section: PrintSection
  dense?: boolean
  /**
   * Print the full-precision figure under every abbreviated one. It doubles the
   * height of a table, so a chapter states the reading and the appendix — the
   * part kept for checking — states it to the som.
   */
  exact?: boolean
}): JSX.Element | null {
  const translate = useTranslate()
  const theme = useSectionTheme(props.section)
  const section = props.section
  const withExact = props.exact
  const rows = createMemo(
    () => auditRows(section, section.document.meta.locale, theme),
  )
  if (!section.frame) return <p class="lens-print-empty">{translate('print.noData', 'No data')}</p>
  const panel = sectionPanel(section)
  const frame = section.frame
  if (panel.kind === 'table' && panel.columns?.length) {
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
      <table class="lens-print-data lens-print-data-wide">
        <thead>
          <tr>
            <For each={columns}>
              {(column, columnIndex) => (
                <th data-align={alignments[columnIndex()]}>
                  {column.label}
                  <Show when={units[columnIndex()]?.note}>
                    <small>{units[columnIndex()]?.note}</small>
                  </Show>
                </th>
              )}
            </For>
          </tr>
        </thead>
        <tbody>
          <For each={frame.rows}>
            {(row) => (
              <tr>
                <For each={columns}>
                  {(column, columnIndex) => {
                    const raw = indexes[columnIndex()]! >= 0 ? row[indexes[columnIndex()]!] : undefined
                    const formatted = units[columnIndex()]!.format(raw)
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
                        data-align={alignments[columnIndex()]}
                        data-negative={value !== undefined && value < 0 ? '' : undefined}
                        data-tone={tone === 'pos' || tone === 'warn' || tone === 'neg' ? tone : undefined}
                      >
                        {formatted}
                        <Show when={secondary !== undefined}>
                          <em class="lens-print-cell-secondary" data-negative={secondary! < 0 ? '' : undefined}>
                            {clampedDeltaPercent(secondary!) ?? `${secondary! > 0 ? '+' : ''}${formatFieldValue(
                              row[secondaryIndex],
                              column.cell.secondaryField ? panel.format[column.cell.secondaryField] : undefined,
                              locale,
                            )}`}
                          </em>
                        </Show>
                        <Show when={max !== undefined && max > 0 && value !== undefined}>
                          <span
                            aria-hidden="true"
                            class="lens-print-cell-bar"
                            data-negative={value! < 0 ? '' : undefined}
                            style={{ width: `${Math.min(100, Math.round((Math.abs(value!) / max!) * 100))}%` }}
                          />
                        </Show>
                        <Show when={exact && exact !== formatted}>
                          <small>{exact}</small>
                        </Show>
                      </td>
                    )
                  }}
                </For>
              </tr>
            )}
          </For>
        </tbody>
      </table>
    )
  }
  const shares = rows().rows.some(({ share }) => share !== undefined)
  return (
    <table class={`lens-print-data${props.dense ? ' lens-print-data-dense' : ''}`}>
      <thead>
        <tr>
          <th>{translate('print.category', 'Category')}</th>
          <th data-align="right">
            {translate('print.value', 'Value')}
            <Show when={rows().unit}>
              <small>{rows().unit}</small>
            </Show>
          </th>
          <Show when={shares}>
            <th data-align="right">{translate('print.share', 'Share')}</th>
          </Show>
        </tr>
      </thead>
      <tbody>
        <For each={rows().rows}>
          {(row) => (
            <tr>
              <td>
                <span class="lens-print-swatch" style={{ 'background-color': row.color }} />
                <Show when={row.series}>
                  <span class="lens-print-series">{row.series} · </span>
                </Show>
                {row.label}
              </td>
              <td data-align="right">
                {row.value}
                <Show when={withExact && row.exact && row.exact !== row.value}>
                  <small>{row.exact}</small>
                </Show>
              </td>
              <Show when={shares}>
                <td class="lens-print-share" data-align="right">
                  {/* The bar carries the proportion the eye needs; the number
                      keeps the row auditable. Neither costs an extra line. */}
                  <Show when={row.ratio !== undefined}>
                    <span
                      aria-hidden="true"
                      class="lens-print-share-bar"
                      style={{ width: `${Math.min(100, Math.round(row.ratio! * 100))}%` }}
                    />
                  </Show>
                  <span>{row.share ?? '—'}</span>
                </td>
              </Show>
            </tr>
          )}
        </For>
      </tbody>
    </table>
  )
}

/** What a printed table stops short of: one page of a level that has more. */
function TruncationNote(props: { section: PrintSection }): JSX.Element | null {
  const translate = useTranslate()
  return (
    <Show when={props.section.hasMore && props.section.frame}>
      <p class="lens-print-truncated">
        {translate('print.truncated', 'First {rows} rows shown; the level continues in the dashboard.', {
          rows: props.section.frame!.rows.length,
        })}
      </p>
    </Show>
  )
}

function FigureNote(props: { section: PrintSection }): JSX.Element | null {
  const translate = useTranslate()
  const panel = sectionPanel(props.section)
  const authored = panel.caption?.trim()
  const fact = createMemo(
    () => narrativeFact(panel, props.section.frame, props.section.document.meta.locale),
  )()
  return (
    <Show when={Boolean(authored) || Boolean(fact)}>
      <p class="lens-print-figure-note">
        <Show when={authored}>
          <span class="lens-print-figure-authored">{authored}</span>
        </Show>
        <Show when={fact}>
          <span>{translate(fact!.labelKey, fact!.fallback, fact!.vars)}</span>
        </Show>
      </p>
    </Show>
  )
}

/** The markers a figure carries into the page's footnotes. */
function FootnoteMarkers(props: { footnotes?: FigureFootnotes }): JSX.Element | null {
  return (
    <Show when={props.footnotes && props.footnotes.markers.length > 0}>
      <sup class="lens-print-footnote-marker">{props.footnotes!.markers.join(', ')}</sup>
    </Show>
  )
}

/** The notes this figure introduced, printed with it so they share its page. */
function FootnoteTexts(props: { footnotes?: FigureFootnotes }): JSX.Element | null {
  return (
    <Show when={props.footnotes && props.footnotes.notes.length > 0}>
      <ol class="lens-print-footnotes">
        <For each={props.footnotes!.notes}>
          {(note) => (
            // The number is drawn rather than left to the list marker: a marker is
            // the one glyph a print engine feels free to drop, and a note nobody
            // can tie back to its figure is a note nobody reads.
            <li value={note.number}>
              <span class="lens-print-footnote-index">{note.number}</span>
              {note.text}
            </li>
          )}
        </For>
      </ol>
    </Show>
  )
}

/**
 * What the dashboard shows when the reading above is clicked. A one-row level
 * is a term of the calculation and prints as one line; anything wider keeps its
 * table. Printed here, the ratio and its parts are read together.
 */
const breakdownRowCap = 8

function BreakdownView(props: { sections: Array<PrintSection> }): JSX.Element | null {
  const translate = useTranslate()
  return (
    <Show when={props.sections.length > 0}>
      <div class="lens-print-breakdown">
        <p class="lens-print-breakdown-head">{translate('print.breakdown', 'How it is calculated')}</p>
        <For each={props.sections}>
          {(section) => {
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
              return <PrintFormula section={section} />
            }
            const valueIndex = frame ? indexOf(frame, panel.encoding.value) : -1
            if (frame && frame.rows.length === 1 && valueIndex >= 0) {
              return (
                <p class="lens-print-breakdown-term">
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
            const capped = Boolean(frame && frame.rows.length > breakdownRowCap)
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
              <div class="lens-print-breakdown-part">
                <Show when={!named}>
                  <p class="lens-print-breakdown-title">{title}</p>
                </Show>
                <PrintDataTable dense section={shown} />
                <Show when={capped && frame}>
                  {/* Here the whole term is in hand, so the reader is told what
                      share of it the page keeps rather than merely that it was cut. */}
                  <p class="lens-print-truncated">
                    {translate('print.truncatedOf', '{rows} of {total} rows shown.', {
                      rows: breakdownRowCap,
                      total: frame!.rows.length,
                    })}
                  </p>
                </Show>
              </div>
            )
          }}
        </For>
      </div>
    </Show>
  )
}

function FigureView(props: { figure: PrintFigure; footnotes?: FigureFootnotes }): JSX.Element | null {
  const translate = useTranslate()
  const figure = props.figure
  const section = figure.section
  const panel = sectionPanel(section)
  const waterfall = isWaterfall(panel, section)
  const formula = formulaKinds.has(panel.kind)
  // A single value needs neither a chart of one point nor a table of one row.
  if (figure.metric) {
    return (
      <figure class={`lens-print-figure lens-print-figure-${figure.width} lens-print-figure-metric`}>
        <MetricTile figure={figure} footnotes={props.footnotes} numbered />
        <BreakdownView sections={figure.breakdown} />
        <FootnoteTexts footnotes={props.footnotes} />
      </figure>
    )
  }
  const total = panel.total !== undefined && panel.encoding.value
    ? formatFieldValue(panel.total, panel.format[panel.encoding.value], section.document.meta.locale)
    : undefined
  return (
    <figure class={`lens-print-figure lens-print-figure-${waterfall || formula ? 'full' : figure.width}`}>
      {/* Two rows, always both: a title that runs long must not push the
          chips down and take the figure beside it out of line. */}
      <figcaption class="lens-print-figure-head">
        <div class="lens-print-figure-title">
          <span class="lens-print-figure-number">
            {translate('print.figure', 'Fig. {number}', { number: figure.number })}
          </span>
          <h3>{panel.title}<FootnoteMarkers footnotes={props.footnotes} /></h3>
        </div>
        <p class="lens-print-figure-meta">
          <Show when={panel.status?.label}>
            <span class="lens-print-chip" data-tone={panel.status!.tone ?? 'neutral'}>{panel.status!.label}</span>
          </Show>
          <PrintQualityChip availability={panel.availability} confidence={panel.confidence} />
          <Show when={section.perspective}>
            <span class="lens-print-chip" data-tone="neutral">
              {translate('print.view', 'View')}: {section.perspective!.label}
            </span>
          </Show>
          {/* The header badge the dashboard shows: the authoritative total the
              shares below are taken against. */}
          <Show when={total}>
            <span class="lens-print-figure-total">{translate('print.total', 'Total')}: {total}</span>
          </Show>
        </p>
      </figcaption>
      {formula
        ? <PrintFormula section={section} />
        : waterfall
          ? <PrintWaterfall section={section} />
          : (figure.chart && <PrintChart height={figure.width === 'half' ? 225 : 235} section={section} />)}
      <FigureNote section={section} />
      {!formula && (waterfall ? <PrintWaterfallTable section={section} /> : <PrintDataTable section={section} />)}
      <TruncationNote section={section} />
      <BreakdownView sections={figure.breakdown} />
      <FootnoteTexts footnotes={props.footnotes} />
    </figure>
  )
}

/**
 * A trend line the way the dashboard draws it beside a stat: no axis, no
 * gridlines, just the shape of the last few periods. Eight quarters of history
 * cost one line of ink and answer the first question a reader has of any single
 * number — whether it is going anywhere.
 */
function PrintSparkline(props: { values: Array<number> }): JSX.Element | null {
  const points = props.values.filter((value) => Number.isFinite(value))
  if (points.length < 2) return null
  const min = Math.min(...points)
  const max = Math.max(...points)
  const span = max - min || 1
  const step = 100 / (points.length - 1)
  const path = points
    .map((value, index) => `${(index * step).toFixed(2)},${(24 - ((value - min) / span) * 22).toFixed(2)}`)
    .join(' ')
  return (
    <svg aria-hidden="true" class="lens-print-sparkline" preserveAspectRatio="none" viewBox="0 0 100 26">
      <polyline fill="none" points={path} stroke-linecap="round" stroke-linejoin="round" />
    </svg>
  )
}

/** One reading: its name, its number, where it came from and where it is going. */
function MetricTile(props: {
  figure: PrintFigure
  footnotes?: FigureFootnotes
  /** Chapter tiles are numbered so the text can point at them; cover tiles are not. */
  numbered?: boolean
}): JSX.Element {
  const translate = useTranslate()
  const figure = props.figure
  const section = figure.section
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
    <div class="lens-print-kpi">
      <p class="lens-print-kpi-label">
        <Show when={props.numbered}>
          <span class="lens-print-figure-number">
            {translate('print.figure', 'Fig. {number}', { number: figure.number })}
          </span>
        </Show>
        {panel.title}
        <FootnoteMarkers footnotes={props.footnotes} />
      </p>
      <p class="lens-print-kpi-value">
        {formatFieldValue(raw, format, locale)}
        <Show when={delta !== undefined}>
          <span class="lens-print-kpi-delta" data-negative={delta! < 0 ? '' : undefined}>
            {delta! > 0 ? '+' : ''}{formatFieldValue(deltaRaw, deltaFormat, locale)}
          </span>
        </Show>
      </p>
      <Show when={panel.sparkline}>
        <PrintSparkline values={panel.sparkline!.values} />
      </Show>
      <Show when={panel.trend}>
        <p class="lens-print-kpi-trend" data-negative={panel.trend!.percent < 0 ? '' : undefined}>
          {panel.trend!.percent > 0 ? '+' : ''}{clampedDeltaPercent(panel.trend!.percent)
            ?? `${new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(panel.trend!.percent)}%`}
          {panel.trend!.label ? ` ${panel.trend!.label}` : ''}
        </p>
      </Show>
      <Show when={target}>
        <p class="lens-print-kpi-target">
          {translate('print.target', 'Target')}: {formatFieldValue(target!.value, format, locale)}
          {target!.label ? ` · ${target!.label}` : ''}
        </p>
      </Show>
      <Show when={panel.caption}>
        <p class="lens-print-kpi-caption">{panel.caption}</p>
      </Show>
      <p class="lens-print-kpi-chips">
        <Show when={panel.status?.label}>
          <span class="lens-print-chip" data-tone={panel.status!.tone ?? 'neutral'}>{panel.status!.label}</span>
        </Show>
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

function Cover(props: {
  document: DashboardDocument
  outline: PrintOutline
  report: PrintReportModel
}): JSX.Element {
  const translate = useTranslate()
  const kpis = props.outline.kpis
  const document = props.document
  const period = periodLabel(document, translate('filter.period.allTime', 'All time'))
  return (
    <header class="lens-print-cover">
      <div class="lens-print-cover-head">
        <p class="lens-print-kicker">{translate('print.kicker', 'Management audit report')}</p>
        <h1>{document.header?.title || document.meta.title}</h1>
        <Show when={document.header?.subtitle}>
          <p class="lens-print-cover-subtitle">{document.header?.subtitle}</p>
        </Show>
      </div>
      <dl class="lens-print-cover-meta">
        <Show when={period}>
          <div>
            <dt>{translate('print.period', 'Period')}</dt>
            <dd>{period}</dd>
          </div>
        </Show>
        <div>
          <dt>{translate('print.generated', 'Generated')}</dt>
          <dd>{new Intl.DateTimeFormat(document.meta.locale, {
            dateStyle: 'long',
            timeStyle: 'short',
          }).format(new Date(document.meta.generatedAt))}</dd>
        </div>
        <div>
          <dt>{translate('print.sections', 'Detailed views')}</dt>
          <dd>{props.outline.figureCount + props.outline.detailCount}</dd>
        </div>
        <Show when={props.outline.estimated > 0 || props.outline.missing.length > 0}>
          <div>
            <dt>{translate('print.quality', 'Data quality')}</dt>
            <dd>{translate('print.qualityCount', '{estimated} estimated · {missing} not calculated', {
              estimated: props.outline.estimated,
              missing: props.outline.missing.length,
            })}</dd>
          </div>
        </Show>
      </dl>
      <Show when={kpis.length > 0}>
        <div class="lens-print-kpis">
          <For each={kpis}>
            {(figure) => <MetricTile figure={figure} />}
          </For>
        </div>
      </Show>
      <Show when={props.report.truncated || props.report.warnings.length > 0 || props.outline.missing.length > 0}>
        <p class="lens-print-cover-flag">{translate(
          'print.limitationsFlag',
          'This report discloses gaps in its data; see the closing appendix.',
        )}</p>
      </Show>
    </header>
  )
}

function Contents(props: { outline: PrintOutline }): JSX.Element {
  const translate = useTranslate()
  return (
    <section class="lens-print-contents">
      <h2>{translate('print.contents', 'Contents')}</h2>
      <ol>
        <For each={props.outline.chapters.filter(({ figures }) => figures.length > 0)}>
          {(chapter) => (
            <li>
              <span class="lens-print-contents-number">{chapter.number}</span>
              <span class="lens-print-contents-title">{chapter.title}</span>
              <Show when={chapter.caption}>
                <span class="lens-print-contents-caption">{chapter.caption}</span>
              </Show>
            </li>
          )}
        </For>
        <Show when={props.outline.appendix.length > 0}>
          <li>
            <span class="lens-print-contents-number">A</span>
            <span class="lens-print-contents-title">
              {translate('print.appendix', 'Appendix A. Detailed breakdowns')}
            </span>
          </li>
        </Show>
        <li>
          <span class="lens-print-contents-number">B</span>
          <span class="lens-print-contents-title">
            {translate('print.method', 'Appendix B. Sources and definitions')}
          </span>
        </li>
      </ol>
      <p class="lens-print-contents-note">{translate(
        'print.note',
        'Every interactive view is expanded below. Values and shares accompany each chart so the report remains self-contained on paper.',
      )}</p>
    </section>
  )
}

function ChapterView(props: { chapter: PrintChapter; title: string }): JSX.Element | null {
  const translate = useTranslate()
  const chapter = props.chapter
  // The chapter's own numbers, said once before the figures that carry them.
  const headline = createMemo(
    () => headlineReadings(
      chapter.figures.map((figure) => ({
        id: figure.section.id,
        panel: sectionPanel(figure.section),
        frame: figure.section.frame,
      })),
      chapter.figures[0]?.section.document.meta.locale ?? 'en',
    ),
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
    <FigureView figure={figure} footnotes={footnotes.get(figure.section.id)} />
  )
  const push = (element: JSX.Element) => {
    if (!lead) {
      body.push(element)
      return
    }
    const heading = lead
    lead = undefined
    body.push(
      <div class="lens-print-lead-group">
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
      <div class="lens-print-pair">
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
        <div class="lens-print-strip">
          <Show when={group.label}>
            <h3>{group.label}</h3>
          </Show>
          <Show when={group.caption && group.caption !== chapter.caption}>
            <p>{group.caption}</p>
          </Show>
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
    <section class="lens-print-chapter">
      <header class="lens-print-chapter-head">
        <p class="lens-print-runninghead">{props.title} · {chapter.number}. {chapter.title}</p>
        <h2><span class="lens-print-chapter-number">{chapter.number}</span>{chapter.title}</h2>
        <Show when={chapter.caption}>
          <p class="lens-print-lead">{chapter.caption}</p>
        </Show>
        <Show when={headline().length > 0}>
          <p class="lens-print-chapter-headline">
            <For each={headline()}>
              {(reading) => (
                <span>
                  <span class="lens-print-chapter-headline-label">{reading.label}</span>
                  {reading.value}
                </span>
              )}
            </For>
          </p>
        </Show>
      </header>
      <div class="lens-print-grid">{body}</div>
    </section>
  )
}

/** Beyond this many points a printed series is a shape, not a list. */
const seriesTableLimit = 12
/** How many periods stay in the table when the shape carries the rest. */
const seriesTableTail = 8

function DetailView(props: { detail: PrintDetail }): JSX.Element {
  const translate = useTranslate()
  const detail = props.detail
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
    <article class="lens-print-detail">
      <header>
        <span class="lens-print-detail-number">{detail.number}</span>
        <h4>{panel.title}</h4>
        <p class="lens-print-detail-trail">{detail.trail}</p>
      </header>
      <Show when={compact}>
        <PrintChart height={110} section={detail.section} />
      </Show>
      <PrintDataTable dense exact section={section} />
      <TruncationNote section={detail.section} />
      <Show when={compact}>
        <p class="lens-print-detail-note">
          {translate('print.seriesTail', 'The chart carries all {rows} periods; the table keeps the most recent {kept}.', {
            rows,
            kept: seriesTableTail,
          })}
        </p>
      </Show>
    </article>
  )
}

function DetailAppendix(props: { outline: PrintOutline; title: string }): JSX.Element | null {
  const translate = useTranslate()
  return (
    <Show when={props.outline.appendix.length > 0}>
      <section class="lens-print-appendix">
        <header class="lens-print-chapter-head">
          <p class="lens-print-runninghead">{props.title} · {translate('print.appendix', 'Appendix A. Detailed breakdowns')}</p>
          <h2><span class="lens-print-chapter-number">A</span>{translate('print.appendix', 'Appendix A. Detailed breakdowns')}</h2>
          <p class="lens-print-lead">{translate(
            'print.appendixNote',
            'Each reading below is one step of the drill path printed above it, kept for verification rather than for reading in order.',
          )}</p>
        </header>
        <For each={props.outline.appendix}>
          {(chapter) => (
            <div class="lens-print-appendix-chapter">
              <h3>A{chapter.number}. {chapter.title}</h3>
              <div class="lens-print-grid lens-print-grid-dense">
                <For each={chapter.details}>
                  {(detail) => <DetailView detail={detail} />}
                </For>
              </div>
            </div>
          )}
        </For>
      </section>
    </Show>
  )
}

function Methodology(props: {
  document: DashboardDocument
  outline: PrintOutline
  report: PrintReportModel
  title: string
}): JSX.Element {
  const translate = useTranslate()
  const definitions = createMemo(
    () => qualityDefinitions(props.outline.qualities, translate),
  )
  return (
    <section class="lens-print-method">
      <header class="lens-print-chapter-head">
        <p class="lens-print-runninghead">{props.title} · {translate('print.method', 'Appendix B. Sources and definitions')}</p>
        <h2>
          <span class="lens-print-chapter-number">B</span>
          {translate('print.method', 'Appendix B. Sources and definitions')}
        </h2>
      </header>
      <Show when={definitions().length > 0}>
        <div class="lens-print-glossary">
          <h3>{translate('print.qualityTerms', 'What the quality marks mean')}</h3>
          <dl>
            <For each={definitions()}>
              {(entry) => (
                <div>
                  <dt><span class="lens-print-chip" data-quality={entry.value}>{entry.label}</span></dt>
                  <dd>{entry.definition}</dd>
                </div>
              )}
            </For>
          </dl>
        </div>
      </Show>
      <Show when={props.outline.notes.length > 0}>
        <dl class="lens-print-notes">
          <For each={props.outline.notes}>
            {(note) => (
              <div>
                <dt>{note.label}</dt>
                <dd>{note.detail}</dd>
              </div>
            )}
          </For>
        </dl>
      </Show>
      <Show when={props.outline.missing.length > 0 || props.report.truncated || props.report.warnings.length > 0}>
        <div class="lens-print-limits">
          <h3>{translate('print.limitations', 'Data limitations')}</h3>
          <p>{translate(
            'print.limitationsNote',
            'The report remains usable, but the following detail views could not be calculated and are explicitly disclosed:',
          )}</p>
          <ul>
            <Show when={props.outline.missing.length > 0}>
              <li>{translate('print.missing', 'Not calculated: {names}', { names: props.outline.missing.join(', ') })}</li>
            </Show>
            <Show when={props.report.truncated}>
              <li>{translate(
                'print.timeLimit',
                'Further detail was stopped at the preparation time limit; all sections calculated by then are included.',
              )}</li>
            </Show>
            <For each={props.report.warnings}>
              {(warning) => <li>{warning}</li>}
            </For>
          </ul>
        </div>
      </Show>
      <p class="lens-print-provenance">
        {translate('print.snapshot', 'Snapshot')}: {props.document.snapshotId} · {props.document.meta.dashboardId} · {props.document.meta.locale}
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
export function PrintReportView(props: { report: PrintReportModel }): JSX.Element {
  const translate = useTranslate()
  const document = props.report.document
  const title = document.header?.title || document.meta.title
  const labels = createMemo(() => buildLabelPalette(props.report))()
  const outline = createMemo(
    () => buildOutline(
      props.report,
      translate('print.summary', 'Summary'),
      translate('print.other', 'Other readings'),
    ),
  )()
  return (
    <PaletteContext.Provider value={labels}>
      <Cover document={document} outline={outline} report={props.report} />
      <Contents outline={outline} />
      <For each={outline.chapters}>
        {(chapter) => <ChapterView chapter={chapter} title={title} />}
      </For>
      <DetailAppendix outline={outline} title={title} />
      <Methodology document={document} outline={outline} report={props.report} title={title} />
    </PaletteContext.Provider>
  )
}

export function PrintReport(): JSX.Element | null {
  const { document } = useDashboard()
  const print = usePrint()
  if (typeof globalThis.document === 'undefined') return null
  return (
    <Show when={print.report}>
      {(report) => {
        // The report is printed in the dashboard's own accent, so a company's board
        // and its report do not disagree about what colour "this matters" is.
        const accent = document.theme.palette.accent ?? document.theme.palette.primary
        return (
          <Portal mount={globalThis.document.body}>
            <article
              aria-hidden={!print.active}
              class="lens-print-report"
              data-preview={print.preview ? 'true' : undefined}
              lang={document.meta.locale}
              style={accent ? ({ '--lens-accent-500': accent } as JSX.CSSProperties) : undefined}
            >
              <PrintReportView report={report()} />
            </article>
          </Portal>
        )
      }}
    </Show>
  )
}
