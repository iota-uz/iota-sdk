import { createContext, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties } from 'react'
import type { Column, FieldFormat, Frame, Level, Panel, TableColumn } from '../contract'
import { actionForRow, resolveColumnActionURL, resolveRowLeafActionURL } from '../explore/actions'
import { ArrowUpRight, ArrowsLeftRight, CaretDown, CaretRight } from '../icons'
import { clampedDeltaPercent, compactReference, levelForPath, useDashboard, useFormat, useFormatAtReference, usePanelFrame, usePanelPagination, useTranslate } from '../runtime'
import { PanelFrame } from './PanelFrame'
import { useActionActivation } from './actions'
import { InfoTip } from './InfoTip'

type SortDirection = 'ascending' | 'descending'

const defaultMinimumSampleSize = 5
const heatMinimumMixPercent = 7
const heatMixRangePercent = 21
const rowCountDisclosureThreshold = 10

interface SortState {
  column: string
  direction: SortDirection
}

/**
 * One magnitude per numeric column.
 *
 * A column of numbers is read downwards, so it carries one notation; a compact
 * field on its own does not — it abbreviates each value independently and stops
 * abbreviating below the compact floor, which is how «Выплачено» came to print
 * «111,13 млн», «45 000» and a bare «0» in one stack of digits. The reference
 * is the column's largest magnitude, and every cell in it is written against
 * that. Columns with no shared magnitude are simply absent from the map, and
 * their cells format exactly as they did.
 */
const ColumnReferences = createContext<ReadonlyMap<string, number> | null>(null)

function useCellFormat(field: string | undefined, format?: FieldFormat): (value: unknown) => string {
  const references = useContext(ColumnReferences)
  return useFormatAtReference(format, field ? references?.get(field) : undefined)
}

function inferredFormat(column: Column): FieldFormat | undefined {
  if (column.type === 'number') return { kind: 'number', minorUnits: false }
  if (column.type === 'time') return { kind: 'date', minorUnits: false }
  if (column.type === 'string') return { kind: 'string', minorUnits: false }
  return undefined
}

function numericValue(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

function comparable(value: unknown, type: Column['type']): number | string {
  if (value === null || value === undefined) return ''
  if (type === 'number') {
    const parsed = typeof value === 'number' ? value : Number(value)
    return Number.isFinite(parsed) ? parsed : Number.NEGATIVE_INFINITY
  }
  if (type === 'time') {
    const time = new Date(typeof value === 'string' || typeof value === 'number' ? value : '').getTime()
    return Number.isNaN(time) ? Number.NEGATIVE_INFINITY : time
  }
  if (type === 'bool') return value === true || value === 1 || value === 'true' ? 1 : 0
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') {
    return String(value).toLocaleLowerCase()
  }
  return JSON.stringify(value)?.toLocaleLowerCase() ?? ''
}

function compare(left: number | string, right: number | string): number {
  if (typeof left === 'number' && typeof right === 'number') return left - right
  return String(left).localeCompare(String(right), undefined, { numeric: true, sensitivity: 'base' })
}

/**
 * The sort state of one column, drawn rather than typed.
 *
 * `↑ ↓ ↕` were Unicode text: three glyphs whose shape, weight and baseline came
 * from whatever font the host page happened to load, in a runtime that inlined a
 * Phosphor set precisely so its marks would not do that (`icons.tsx`). The
 * caret is `CaretDown`, rotated for the ascending state — a caret is
 * symmetric, so a rotation is the same drawing, and the runtime already turns
 * this glyph for the group-row chevron.
 *
 * The unsorted state is a different mark, not a fainter caret — the double
 * arrow, which is what `↕` was drawing — and it is held back until the header is
 * hovered or focused: a dense table otherwise repeats "you may sort this" on
 * every column at once, which is noise beside the one column that says which way
 * it is sorted. It still occupies its box in every state, because revealing it
 * must not move the label beside it, least of all in a right-aligned header
 * whose whole point is that the label ends where the digits do.
 *
 * The element is decorative; `aria-sort` on the `th` carries the state.
 */
function SortIndicator({ direction }: { direction?: SortDirection }) {
  return (
    <span
      aria-hidden="true"
      className={`lens-table-sort${direction ? ` lens-table-sort-${direction}` : ''}`}
    >
      {direction === 'ascending' && <CaretDown className="lens-table-sort-up" />}
      {direction === 'descending' && <CaretDown />}
      {!direction && <ArrowsLeftRight className="lens-table-sort-neutral" />}
    </span>
  )
}

/**
 * Whether a column's contents sit against its right edge.
 *
 * Alignment is a property of what the column holds, not of whether a producer
 * remembered to declare it: digits are read by their place values, so they line
 * up on the right, and the header that names them has to sit over the same edge
 * or it heads the column beside it. Reading the frame's own column type is what
 * makes that true for every table at once — the previous rule consulted
 * `align` alone, so a numeric column whose spec omitted it printed a
 * left-aligned label and sort control 160px away from its own values.
 *
 * A declared `align` still wins: a producer that says "left" over a numeric
 * column has a reason (an identifier that happens to be numeric, say).
 */
function isRightAligned(column: TableColumn, frame: Frame): boolean {
  if (column.align) return column.align === 'right'
  // Bars, underlines and deltas are magnitude cells: their own layout already
  // packs to the right, so the column reads as numeric whatever the frame says.
  if (column.cell.kind === 'bar' || column.cell.kind === 'underline' || column.cell.kind === 'delta') return true
  if (!column.field.trim()) return false
  return frame.columns.find((candidate) => candidate.name === column.field)?.type === 'number'
}

function TableCell({ column, format, value }: { column: Column; format?: FieldFormat; value: unknown }) {
  const display = useCellFormat(column.name, format ?? inferredFormat(column))
  const translate = useTranslate()
  if (column.type === 'bool') {
    if (value === null || value === undefined || value === '') return <span className="lens-table-null">—</span>
    const checked = value === true || value === 1 || value === 'true'
    return <span className="lens-table-bool" data-value={checked}>
      {checked ? translate('table.boolean.yes', 'Yes') : translate('table.boolean.no', 'No')}
    </span>
  }
  const text = display(value)
  if (column.type === 'time' && text !== '—') {
    return <time dateTime={typeof value === 'string' ? value : undefined}>{text}</time>
  }
  return <>{text}</>
}

function BarCell({ field, format, value, max }: { field?: string; format?: FieldFormat; value: unknown; max: number }) {
  const display = useCellFormat(field, format)
  const number = numericValue(value)
  const ratio = max > 0 && number !== undefined ? Math.max(0, Math.min(1, Math.abs(number) / max)) : 0
  const negative = number !== undefined && number < 0
  // An empty track is a bar that is loading, not a bar that is zero: five rows
  // of «0 / 0.0%» drew five full-width grey rails with nothing in them, which is
  // the runtime's own skeleton shape. A zero has no magnitude to draw, so it
  // draws none and the numeral carries the row.
  const drawn = number !== undefined && number !== 0
  return (
    <div className="lens-table-bar">
      {drawn && (
        <span className="lens-table-bar-track" aria-hidden="true">
          {/* The magnitude is drawn from the track's midpoint outwards so a
              negative value cannot look identical to its positive twin. */}
          <span
            className={`lens-table-bar-fill${negative ? ' lens-table-bar-fill-negative' : ''}`}
            style={{ width: `${ratio * 50}%`, [negative ? 'right' : 'left']: '50%' }}
          />
        </span>
      )}
      <span className="lens-table-bar-value">{display(value)}</span>
    </div>
  )
}

/**
 * A thin rule under the value, colored by sign. The rule spans the value it
 * underlines rather than encoding magnitude: a proportional rule degenerates
 * into something indistinguishable from a stray hyphen as soon as one row
 * dominates the column, which is exactly what the legacy treatment avoids.
 */
function UnderlineCell({ field, format, value }: { field?: string; format?: FieldFormat; value: unknown }) {
  const display = useCellFormat(field, format)
  const number = numericValue(value)
  const negative = number !== undefined && number < 0
  const blank = number === undefined
  return (
    <span className="lens-table-underline">
      <span className="lens-table-underline-value">{display(value)}</span>
      {!blank && (
        <span
          aria-hidden="true"
          className={`lens-table-underline-rule${negative ? ' lens-table-underline-rule-negative' : ''}`}
        />
      )}
    </span>
  )
}

function DeltaCell({
  valueFormat, value, secondaryFormat, secondaryValue, stacked,
}: {
  valueFormat?: FieldFormat
  value: unknown
  secondaryFormat?: FieldFormat
  secondaryValue: unknown
  stacked?: boolean
}) {
  const displayValue = useFormat(valueFormat)
  const displaySecondary = useFormat(secondaryFormat)
  const translate = useTranslate()
  const { document } = useDashboard()
  const absolute = numericValue(value)
  const secondary = numericValue(secondaryValue)
  const hasSecondary = secondaryValue !== null && secondaryValue !== undefined && secondaryValue !== ''
  const isNew = !hasSecondary && absolute !== undefined && absolute !== 0
  const negative = secondary !== undefined && secondary < 0
  // Percent changes beyond ±999.9% clamp to «>999%» / «<−999%»: a precise
  // «+13 417.3%» is noise, and the absolute value beside it carries the story.
  const clamped = secondary !== undefined ? clampedDeltaPercent(secondary, document.meta.locale) : undefined
  const percent = (hasSecondary || isNew) && (
    <span className={`lens-table-delta-pct${negative ? ' lens-table-delta-pct-negative' : ''}`}>
      {isNew
        ? translate('panel.trend.new', 'New')
        : clamped ?? <>{secondary !== undefined && secondary > 0 ? '+' : ''}{displaySecondary(secondaryValue)}</>}
    </span>
  )
  if (stacked) {
    // Stacked reads top-down: the relative change first, the absolute
    // amount beneath it as supporting detail.
    return (
      <span className="lens-table-delta lens-table-delta-stacked">
        {percent}
        <span className="lens-table-delta-value">{displayValue(value)}</span>
      </span>
    )
  }
  return (
    <span className="lens-table-delta">
      <span className="lens-table-delta-value">{displayValue(value)}</span>
      {percent}
    </span>
  )
}

// rowFieldString reads a companion frame column (tone, badge) as a string. An
// absent field or non-string value reads as "" so callers can treat it as "no
// annotation" without branching on undefined.
function rowFieldString(frame: Frame, row: Array<unknown>, field?: string): string {
  if (!field) return ''
  const index = frame.columns.findIndex((candidate) => candidate.name === field)
  if (index < 0) return ''
  const value = row[index]
  return typeof value === 'string' ? value : ''
}

function ColumnCell({
  column, frame, row, panel, location, max,
}: { column: TableColumn; frame: Frame; row: Array<unknown>; panel: Panel; location: URL; max: number }) {
  const activation = useActionActivation(column.action)
  const translate = useTranslate()
  const index = frame.columns.findIndex((candidate) => candidate.name === column.field)
  const type = frame.columns[index]?.type ?? 'string'
  const value = index >= 0 ? row[index] : undefined
  const format = panel.format[column.field]

  let content
  if (!column.field.trim() && column.text) {
    // An action-only column carries its own literal label; there is no field
    // to read and no value to format.
    content = <span className="lens-table-cell-text">{column.text}</span>
  } else if (column.cell.kind === 'bar') {
    content = <BarCell field={column.field} format={format} value={value} max={max} />
  } else if (column.cell.kind === 'underline') {
    content = <UnderlineCell field={column.field} format={format} value={value} />
  } else if (column.cell.kind === 'delta') {
    const secondaryField = column.cell.secondaryField
    const secondaryIndex = secondaryField ? frame.columns.findIndex((candidate) => candidate.name === secondaryField) : -1
    content = (
      <DeltaCell
        valueFormat={format}
        value={value}
        secondaryFormat={secondaryField ? panel.format[secondaryField] : undefined}
        secondaryValue={secondaryIndex >= 0 ? row[secondaryIndex] : undefined}
        stacked={column.cell.layout === 'stacked'}
      />
    )
  } else {
    content = <TableCell column={{ name: column.field, type }} format={format} value={value} />
  }

  // A per-row status tone tints only the value's color; the producer sets it
  // from its own business thresholds (e.g. a loss ratio over 100%).
  const tone = rowFieldString(frame, row, column.cell.toneField)
  if (tone === 'pos' || tone === 'warn' || tone === 'neg') {
    content = <span className={`lens-table-tone lens-table-tone-${tone}`}>{content}</span>
  }

  const href = column.action && activation.available ? resolveColumnActionURL(column.action, frame, row, location) : undefined

  if (column.clamp) {
    const fullText = typeof value === 'string' ? value : undefined
    // A clamp with no way to see what it cut is the defect, not the clamp. Two
    // lines of a 160px column hold about 21 of the 92 characters of «ОБ-10-1.
    // Обязательное…», and six of the nine products in that table are
    // indistinguishable from their prefixes — so the reader has to be able to
    // read the rest, and a native `title` cannot be reached from a keyboard, is
    // not reached at all by touch, and takes a second of hover to appear.
    // The full name is a real element now: it belongs to the runtime, it takes
    // the runtime's popover chrome, and it appears on focus as readily as on
    // hover.
    const clamped = (
      <>
        <span
          className="lens-table-clamp"
          style={{ WebkitLineClamp: column.clamp } as CSSProperties}
          title={fullText}
        >
          {content}
        </span>
        {fullText && <span className="lens-table-clamp-full">{fullText}</span>}
      </>
    )
    // A cell that drills already has something focusable in it, and revealing
    // on `:focus-within` costs the reader no extra tab stop. A cell that does
    // not is given a real control — a disclosure — rather than a focusable
    // `span`, which is a tab stop that announces nothing.
    content = href || !fullText
      ? <span className="lens-table-clamp-host">{clamped}</span>
      : (
        <button className="lens-table-clamp-host" type="button">
          {clamped}
        </button>
      )
  }
  const pill = column.affordance === 'pill'
  const quiet = column.affordance === 'quiet'
  let rendered
  if (href && quiet) {
    // The whole cell is the drill target; the underline and arrow stay hidden
    // until hover/focus so a dense table of quiet numerals reads as data first.
    rendered = (
      <a className="lens-table-cell-quiet" href={href} onClick={activation.onClick(href)}>
        <span className="lens-table-cell-quiet-value">{content}</span>
        <span aria-hidden="true" className="lens-table-cell-quiet-arrow"><ArrowUpRight /></span>
      </a>
    )
  } else if (href) {
    rendered = (
      <a className={`lens-table-cell-link${pill ? ' lens-table-cell-pill' : ''}`} href={href} onClick={activation.onClick(href)}>
        {content}
        {/* The arrow claims the cell opens something, so it only appears when
            a target actually resolved. */}
        <span className="lens-table-cell-link-arrow">{pill ? <ArrowUpRight /> : <CaretRight />}</span>
      </a>
    )
  } else if (pill) {
    // A pill without a resolvable target still marks the column as a drill
    // surface (the action may be renderer-local), but it does not pretend to
    // be a link.
    rendered = <span className="lens-table-cell-pill">{content}</span>
  } else {
    rendered = content
  }

  // A badge (e.g. an unmatched-source marker) rides beside the value; its
  // tooltip carries the producer's explanation.
  const badge = rowFieldString(frame, row, column.badgeField)
  const sampleIndex = column.sampleSizeField
    ? frame.columns.findIndex((candidate) => candidate.name === column.sampleSizeField)
    : -1
  const sampleSize = sampleIndex >= 0 ? numericValue(row[sampleIndex]) : undefined
  const minimum = (column.minSampleSize ?? 0) > 0 ? column.minSampleSize! : defaultMinimumSampleSize
  if (sampleSize !== undefined && sampleSize < minimum) {
    const smallSampleLabel = translate(
      'table.smallSample',
      'Small sample: n={count}, minimum {minimum}',
      { count: sampleSize, minimum },
    )
    rendered = (
      <span className="lens-table-small-sample">
        {rendered}
        <span className="lens-table-small-sample-marker">
          {translate('table.smallSampleShort', 'n<{minimum}', { minimum })}
          <InfoTip inline text={smallSampleLabel} />
        </span>
      </span>
    )
  }
  if (!badge) return rendered
  return (
    <span className="lens-table-cell-badged">
      {rendered}
      <span className="lens-table-cell-badge" title={badge}><InfoTip inline text={badge} /></span>
    </span>
  )
}

interface NumericRange {
  min: number
  max: number
  /** Every value in the column, ascending — the ranking the shade reads off. */
  sorted: number[]
}

/**
 * Where `value` sits among the column's other values, from 0 to 1.
 *
 * The shade used to be linear in the value, which on a column whose top row is
 * 4,04 млрд and whose next is 312 млн meant one cell at L 0.76 and six of the
 * remaining eight at *exactly* L 0.94 — a 55× spread rendered as one colour.
 * The heat then encoded nothing the numeral had not already said. Ranking is
 * what a reader is actually looking for in a shaded column: which rows are the
 * big ones, in order, whatever the distances between them.
 *
 * Ties share a rank, so two equal amounts cannot take different shades.
 */
export function heatRank(value: number, sorted: readonly number[]): number {
  if (sorted.length <= 1) return 0.5
  const first = sorted.indexOf(value)
  if (first < 0) return 0.5
  const last = sorted.lastIndexOf(value)
  return ((first + last) / 2) / (sorted.length - 1)
}

function heatCellStyle(column: TableColumn, frame: Frame, row: Array<unknown>, ranges: Map<string, NumericRange>): CSSProperties | undefined {
  if (!column.heat) return undefined
  const index = frame.columns.findIndex((candidate) => candidate.name === column.field)
  const value = index >= 0 ? numericValue(row[index]) : undefined
  const range = ranges.get(column.field)
  if (value === undefined || !range) return undefined
  const intensity = heatRank(value, range.sorted)
  const percentage = Math.round(heatMinimumMixPercent + Math.max(0, Math.min(1, intensity)) * heatMixRangePercent)
  return { backgroundColor: `color-mix(in srgb, var(--lens-accent-500) ${percentage}%, var(--lens-bg-card))` }
}

function sortedRows(frame: Frame, sort: SortState | undefined): Array<{ row: Array<unknown>; index: number }> {
  const rows = frame.rows.map((row, index) => ({ row, index }))
  if (!sort) return rows
  const columnIndex = frame.columns.findIndex(({ name }) => name === sort.column)
  const column = frame.columns[columnIndex]
  if (!column) return rows
  const direction = sort.direction === 'ascending' ? 1 : -1
  return rows.sort((left, right) => {
    const result = compare(comparable(left.row[columnIndex], column.type), comparable(right.row[columnIndex], column.type))
    return result === 0 ? left.index - right.index : result * direction
  })
}

export interface TablePanelProps {
  panel: Panel
}

function TableSummaryCell({ panel, field, value }: { panel: Panel; field: string; value: unknown }) {
  const display = useFormat(panel.format[field])
  return <>{value === undefined ? <SummaryVoid /> : display(value)}</>
}

/**
 * What a totals row says about a column it cannot total.
 *
 * «Средняя тяжесть» left its Итого cell empty, which reads as data that failed
 * to arrive rather than as a quantity that does not sum — and the two are
 * indistinguishable from the outside, so the reader has to guess which table
 * they are looking at. The em dash states the second one on purpose.
 *
 * It is a dash and not a computed average deliberately: the runtime is handed a
 * map of producer-computed totals, and the columns without one are ratios and
 * per-unit figures whose set-level value is a *weighted* average of inputs this
 * component never sees. Averaging the visible rows would print a number that is
 * arithmetically wrong (the mean of nine products' severities is not the
 * portfolio's severity), and being wrong is worse than being blank. A column
 * that does have a meaningful set-level value belongs in `summary.values` with
 * `total: true` — that is the producer's call, and the runtime honours it.
 */
function SummaryVoid() {
  const translate = useTranslate()
  return (
    <span className="lens-table-summary-void">
      <span aria-hidden="true">—</span>
      {/* TODO(i18n): register `table.notSummable` (en «Not summable», ru «Не суммируется»). */}
      <span className="lens-sr-only">{translate('table.notSummable', 'Not summable')}</span>
    </span>
  )
}

type RenderRow =
  | { row: Array<unknown>; index: number; kind: 'normal' | 'member' }
  | { row: Array<unknown>; index: number; kind: 'toggle'; group: string; expanded: boolean }

export function TablePanel({ panel }: TablePanelProps) {
  const frame = usePanelFrame(panel.id)
  const { document, navigation } = useDashboard()
  const pagination = usePanelPagination()
  const translate = useTranslate()
  const [sort, setSort] = useState<SortState>()
  const [search, setSearch] = useState('')
  const [locationHref, setLocationHref] = useState(() => globalThis.location.href)
  const scrollRef = useRef<HTMLDivElement>(null)
  const [scrollEdges, setScrollEdges] = useState({ left: false, right: false })
  const [hasHorizontalOverflow, setHasHorizontalOverflow] = useState(false)
  const searchInitialized = useRef(false)
  const [requestedPage, setRequestedPage] = useState(frame.page?.number ?? 1)
  const requestedSnapshotId = useRef(document.snapshotId)
  const level: Level | undefined = navigation.panelId === panel.id && navigation.path.length
    ? levelForPath(document, navigation.path)
    : undefined
  // A static identity table (a fixed decomposition, not a record list) declares
  // sortable:false: its rows carry an inherent order, so offering to reorder
  // them — and printing "sort applies to this page" — would be a lie.
  const serverSort = Boolean(document?.endpoints?.panel && panel.table?.searchable)
  const sortEnabled = panel.presentation?.sortable !== false && (!frame.page || serverSort)
  // A server-searchable table is ordered by the same producer over the same
  // result scope; only an entirely static frame is safe to reorder locally.
  const rows = useMemo(() => frame.data ? sortedRows(frame.data, !serverSort && sortEnabled ? sort : undefined) : [], [frame.data, serverSort, sort, sortEnabled])
  const page = frame.page?.number ?? 1
  const pageSize = frame.page?.size
  const loadingPage = requestedSnapshotId.current === document.snapshotId ? requestedPage : 1
  // The row-count check keeps pagination working with older servers that omit hasNext.
  const hasNext = frame.page?.hasNext ?? Boolean(pageSize && (frame.data?.rows.length ?? 0) >= pageSize)
  useEffect(() => {
    const syncLocation = () => setLocationHref(globalThis.location.href)
    globalThis.addEventListener('popstate', syncLocation)
    return () => globalThis.removeEventListener('popstate', syncLocation)
  }, [])
  const location = useMemo(() => new URL(locationHref), [locationHref])
  const columns = panel.columns?.length ? panel.columns : undefined
  // Column maxima scale bar cells. Computing them per cell is quadratic in
  // row count, so they are derived once per frame.
  const columnStats = useMemo(() => {
    const maxima = new Map<string, number>()
    const ranges = new Map<string, NumericRange>()
    // References are keyed off the frame, not the column specs: a table
    // rendered straight from its frame has no specs at all and its columns
    // read downwards just the same.
    const references = new Map<string, number>()
    if (!frame.data) return { maxima, ranges, references }
    for (const [index, dataColumn] of frame.data.columns.entries()) {
      const values = frame.data.rows
        .map((row) => numericValue(row[index]))
        .filter((value): value is number => value !== undefined)
      const reference = compactReference(values, panel.format[dataColumn.name])
      if (reference !== undefined) references.set(dataColumn.name, reference)
    }
    if (!columns) return { maxima, ranges, references }
    for (const column of columns) {
      if (column.cell.kind !== 'bar' && !column.heat) continue
      const index = frame.data.columns.findIndex((candidate) => candidate.name === column.field)
      if (index < 0) continue
      const values = frame.data.rows.map((row) => numericValue(row[index])).filter((value): value is number => value !== undefined)
      if (column.cell.kind === 'bar') maxima.set(column.field, values.reduce((maximum, value) => Math.max(maximum, Math.abs(value)), 0))
      if (column.heat && values.length > 0) {
        ranges.set(column.field, {
          min: Math.min(...values),
          max: Math.max(...values),
          sorted: [...values].sort((left, right) => left - right),
        })
      }
    }
    return { maxima, ranges, references }
  }, [columns, frame.data, panel.format])
  // A panel-level leaf action applies to whole rows. In columns mode it has
  // no column of its own, so the table appends one; otherwise the action the
  // document declares would never reach the DOM.
  const rowLeafAction = Boolean(columns) && panel.actions.some((action) => (
    action.kind === 'navigate_to_leaf' || action.kind === 'open_drawer'
  ))

  // Row grouping collapses tagged member rows behind a synthetic toggle row.
  // The tag lives in a companion frame column named by presentation.rowGroupField.
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({})
  const rowGroupField = panel.presentation?.rowGroupField
  const groupIndex = columns && frame.data && rowGroupField
    ? frame.data.columns.findIndex((candidate) => candidate.name === rowGroupField)
    : -1
  const toggleGroup = (group: string) => setExpandedGroups((current) => ({ ...current, [group]: !current[group] }))
  const renderRows = useMemo<Array<RenderRow>>(() => {
    if (groupIndex < 0) return rows.map((entry) => ({ ...entry, kind: 'normal' as const }))
    const normal: Array<RenderRow> = []
    const toggles: Array<{ entry: { row: Array<unknown>; index: number }; group: string }> = []
    const members = new Map<string, Array<{ row: Array<unknown>; index: number }>>()
    for (const entry of rows) {
      const raw = entry.row[groupIndex]
      const tag = typeof raw === 'string' ? raw : ''
      if (!tag) { normal.push({ ...entry, kind: 'normal' }); continue }
      if (tag.endsWith(':toggle')) { toggles.push({ entry, group: tag.slice(0, -':toggle'.length) }); continue }
      const bucket = members.get(tag)
      if (bucket) bucket.push(entry)
      else members.set(tag, [entry])
    }
    // Normal rows keep their sorted order; group rows pin to the end so a column
    // sort never scatters the collapsed tail through the live rows.
    const out: Array<RenderRow> = [...normal]
    for (const { entry, group } of toggles) {
      const expanded = Boolean(expandedGroups[group])
      out.push({ ...entry, kind: 'toggle', group, expanded })
      if (expanded) for (const member of members.get(group) ?? []) out.push({ ...member, kind: 'member' })
    }
    return out
  }, [rows, groupIndex, expandedGroups])
  // The footer's "N rows" counts real data rows, never the synthetic toggle.
  const dataRowCount = groupIndex < 0
    ? (frame.data?.rows.length ?? 0)
    : (frame.data?.rows.reduce((count, row) => {
      const raw = row[groupIndex]
      return typeof raw === 'string' && raw.endsWith(':toggle') ? count : count + 1
    }, 0) ?? 0)

  useEffect(() => {
    if (requestedSnapshotId.current !== document.snapshotId) {
      requestedSnapshotId.current = document.snapshotId
      setRequestedPage(1)
      setSearch('')
      searchInitialized.current = false
      return
    }
    if (frame.page?.number) setRequestedPage(frame.page.number)
  }, [document.snapshotId, frame.page?.number])

  useEffect(() => {
    if (!panel.table?.searchable) return undefined
    if (!searchInitialized.current) {
      searchInitialized.current = true
      return undefined
    }
    const timeout = globalThis.setTimeout(() => { void pagination.search(panel.id, search) }, document.theme.debounceMs ?? 500)
    return () => globalThis.clearTimeout(timeout)
  }, [document.theme.debounceMs, pagination, panel.id, panel.table?.searchable, search])

  const changePage = (next: number) => {
    setRequestedPage(next)
    void pagination.loadPage(panel.id, next)
  }

  const changeSort = (column: string) => {
    const next: SortState = sort?.column === column
      ? { column, direction: sort.direction === 'ascending' ? 'descending' : 'ascending' }
      : { column, direction: 'ascending' as const }
    setSort(next)
    if (serverSort) void pagination.sort(panel.id, { field: column, direction: next.direction === 'ascending' ? 'asc' : 'desc' })
  }

  const sortDirection = (name: string) => sortEnabled && sort?.column === name ? sort.direction : undefined
  const columnCount = columns ? columns.length + (rowLeafAction ? 1 : 0) : (frame.data?.columns.length ?? 0) + 1

  useLayoutEffect(() => {
    const scroller = scrollRef.current
    if (!scroller) return undefined
    let animationFrame: number | undefined
    const measure = () => {
      const sticky = scroller.querySelector<HTMLElement>('thead th:first-child')
      const stickyWidth = sticky?.offsetWidth ?? 0
      const previousStickyWidth = Number.parseFloat(scroller.parentElement?.style.getPropertyValue('--lens-table-sticky-width') ?? '')
      scroller.parentElement?.style.setProperty('--lens-table-sticky-width', `${stickyWidth}px`)
      const table = scroller.querySelector<HTMLTableElement>('table')
      const tableWidth = table?.scrollWidth || scroller.scrollWidth
      // The trailing spacer is part of the table's real scroll box, but must
      // not create overflow on a table whose data columns fit. Remove its
      // known width when deciding whether the spacer remains necessary.
      const contentWidth = Math.max(0, tableWidth - (hasHorizontalOverflow ? stickyWidth : 0))
      const overflowing = contentWidth > scroller.clientWidth + 1
      setHasHorizontalOverflow((current) => current === overflowing ? current : overflowing)
      // A translated sticky first column contributes its width to Chromium's
      // scrollWidth. Clamp against the table itself so the native scrollbar
      // cannot move past the real content and crop the sticky cells.
      const headers = Array.from(scroller.querySelectorAll<HTMLElement>('thead th:not(.lens-table-scroll-spacer):not(.lens-table-action-heading)'))
      const nativeMaximum = Math.max(0, tableWidth - scroller.clientWidth)
      const lastHeader = headers.at(-1)
      const minimum = Math.max(0, (lastHeader?.offsetLeft ?? 0) + (lastHeader?.offsetWidth ?? 0) - scroller.clientWidth)
      const reachableAlignment = headers.slice(1)
        .map((header) => Math.max(0, header.offsetLeft - stickyWidth))
        .find((target) => target >= minimum - 1 && target <= nativeMaximum + 1)
      const maximum = reachableAlignment ?? Math.min(minimum, nativeMaximum)
      if (scroller.scrollLeft > maximum) scroller.scrollLeft = maximum
      const left = scroller.scrollLeft > 1
      const right = scroller.scrollLeft < maximum - 1
      setScrollEdges((current) => current.left === left && current.right === right ? current : { left, right })
      // Adding the spacer can reflow the sticky column. Measure once more
      // after that layout settles so both widths converge to the same value.
      if (!Number.isFinite(previousStickyWidth) || Math.abs(previousStickyWidth - stickyWidth) > 0.5) {
        if (animationFrame !== undefined) globalThis.cancelAnimationFrame(animationFrame)
        animationFrame = globalThis.requestAnimationFrame(measure)
      }
    }
    measure()
    scroller.addEventListener('scroll', measure, { passive: true })
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(measure)
    observer?.observe(scroller)
    const table = scroller.querySelector<HTMLElement>('table')
    const sticky = scroller.querySelector<HTMLElement>('thead th:first-child')
    if (table) observer?.observe(table)
    if (sticky) observer?.observe(sticky)
    return () => {
      scroller.removeEventListener('scroll', measure)
      observer?.disconnect()
      if (animationFrame !== undefined) globalThis.cancelAnimationFrame(animationFrame)
      scroller.parentElement?.style.removeProperty('--lens-table-sticky-width')
    }
  }, [columnCount, frame.data, hasHorizontalOverflow])

  const scrollHorizontally = (direction: -1 | 1) => {
    const scroller = scrollRef.current
    if (!scroller) return
    const tableWidth = scroller.querySelector<HTMLTableElement>('table')?.scrollWidth || scroller.scrollWidth
    const headers = Array.from(scroller.querySelectorAll<HTMLElement>('thead th:not(.lens-table-scroll-spacer):not(.lens-table-action-heading)'))
    const stickyWidth = headers[0]?.offsetWidth ?? 0
    const nativeMaximum = Math.max(0, tableWidth - scroller.clientWidth)
    const lastHeader = headers.at(-1)
    const minimum = Math.max(0, (lastHeader?.offsetLeft ?? 0) + (lastHeader?.offsetWidth ?? 0) - scroller.clientWidth)
    const reachableAlignment = headers.slice(1)
      .map((header) => Math.max(0, header.offsetLeft - stickyWidth))
      .find((target) => target >= minimum - 1 && target <= nativeMaximum + 1)
    const maximum = reachableAlignment ?? Math.min(minimum, nativeMaximum)
    const targets = headers.slice(1)
      .map((header) => Math.max(0, Math.min(maximum, header.offsetLeft - stickyWidth)))
      .filter((target, index, values) => target > 0 && (index === 0 || target !== values[index - 1]))
    const desired = scroller.scrollLeft + direction * Math.max(120, Math.round(scroller.clientWidth * 0.8))
    const candidates = direction > 0
      ? targets.filter((target) => target > scroller.scrollLeft + 1)
      : targets.filter((target) => target < scroller.scrollLeft - 1)
    const next = candidates.length === 0
      ? (direction > 0 ? maximum : 0)
      : direction > 0
        ? (candidates.find((target) => target >= desired) ?? candidates[candidates.length - 1])
        : ([...candidates].reverse().find((target) => target <= desired) ?? candidates[0])
    if (next === undefined) return
    scroller.scrollLeft = next
    setScrollEdges({ left: next > 1, right: next < maximum - 1 })
    scroller.focus({ preventScroll: true })
  }

  return (
    <PanelFrame panel={panel} frame={frame} allowEmptyContent={Boolean(frame.page)}>
      {frame.data && (
        <ColumnReferences.Provider value={columnStats.references}>
          <div className="lens-table-view">
            {panel.table?.searchable && (
              <label className="lens-table-search">
                <span className="lens-sr-only">{translate('table.search', 'Search table')}</span>
                <input
                  aria-label={translate('table.search', 'Search table')}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder={translate('table.searchPlaceholder', 'Search all rows…')}
                  type="search"
                  value={search}
                />
              </label>
            )}
            <div
              className="lens-table-scroll-frame"
              data-overflow-left={scrollEdges.left}
              data-overflow-right={scrollEdges.right}
            >
              {/* eslint-disable jsx-a11y/no-noninteractive-tabindex -- overflowing native scroll regions must be keyboard-focusable. */}
              <div
                aria-label={translate('table.scrollRegion', 'Scrollable table')}
                className="lens-table-scroll"
                ref={scrollRef}
                role="region"
                tabIndex={scrollEdges.left || scrollEdges.right ? 0 : undefined}
              >
                {/* eslint-enable jsx-a11y/no-noninteractive-tabindex */}
                <table className={`lens-table${columnCount >= 4 ? ' lens-table-wide' : ''}`}>
                  <thead>
                    <tr>
                      {columns ? (
                        <>
                          {columns.map((column, columnIndex) => {
                            // An action-only column has no field to sort by, and a
                            // static table offers no sort at all; either way the
                            // heading is a plain label, not a control.
                            const sortable = sortEnabled && Boolean(column.field.trim())
                            return (
                              <th
                                aria-sort={sortable ? (sort?.column === column.field ? sort.direction : 'none') : undefined}
                                className={isRightAligned(column, frame.data!) ? 'lens-table-col-right' : undefined}
                                key={column.field || `column-${columnIndex}`}
                                scope="col"
                                style={column.widthPx ? { minWidth: `${column.widthPx}px` } : undefined}
                              >
                                {sortable ? (
                                  <button type="button" onClick={() => changeSort(column.field)}>
                                    <span>{column.label}</span>
                                    <SortIndicator direction={sortDirection(column.field)} />
                                  </button>
                                ) : (
                                  <span className="lens-table-heading-static">{column.label}</span>
                                )}
                              </th>
                            )
                          })}
                          {rowLeafAction && (
                            <th className="lens-table-action-heading" scope="col">
                              <span className="lens-sr-only">{translate('table.actions', 'Actions')}</span>
                            </th>
                          )}
                        </>
                      ) : (
                        <>
                          {frame.data.columns.map((column) => (
                            <th
                              aria-sort={sortEnabled ? (sort?.column === column.name ? sort.direction : 'none') : undefined}
                              /* The body cell for these types is right-aligned
                               (`.lens-table-cell-number`, `-time`); the header
                               follows the data it heads. */
                              className={column.type === 'number' || column.type === 'time' ? 'lens-table-col-right' : undefined}
                              key={column.name}
                              scope="col"
                            >
                              {sortEnabled ? (
                                <button type="button" onClick={() => changeSort(column.name)}>
                                  <span>{column.name}</span>
                                  <SortIndicator direction={sortDirection(column.name)} />
                                </button>
                              ) : (
                                <span className="lens-table-heading-static">{column.name}</span>
                              )}
                            </th>
                          ))}
                          <th className="lens-table-action-heading" scope="col">
                            <span className="lens-sr-only">{translate('table.actions', 'Actions')}</span>
                          </th>
                        </>
                      )}
                      {hasHorizontalOverflow && <th aria-hidden="true" className="lens-table-scroll-spacer" />}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.length === 0 ? (
                      <tr>
                        <td className="lens-table-empty" colSpan={columnCount}>
                          {translate('table.emptyPage', 'No records on this page')}
                        </td>
                        {hasHorizontalOverflow && <td aria-hidden="true" className="lens-table-scroll-spacer" />}
                      </tr>
                    ) : renderRows.map((entry) => !columns ? (
                      <FrameRow
                        frame={frame.data!}
                        row={entry.row}
                        index={entry.index}
                        panel={panel}
                        location={location}
                        level={level}
                        openRecordLabel={translate('table.openRecord', 'Open record')}
                        trailingSpacer={hasHorizontalOverflow}
                        key={entry.index}
                      />
                    ) : entry.kind === 'toggle' ? (
                      <GroupToggleRow
                        columns={columns}
                        frame={frame.data!}
                        row={entry.row}
                        panel={panel}
                        location={location}
                        columnMaxima={columnStats.maxima}
                        columnRanges={columnStats.ranges}
                        expanded={entry.expanded}
                        onToggle={() => toggleGroup(entry.group)}
                        trailingSpacer={hasHorizontalOverflow}
                        key={`toggle-${entry.group}`}
                      />
                    ) : (
                      <tr className={entry.kind === 'member' ? 'lens-table-group-member' : undefined} key={entry.index}>
                        {columns.map((column, columnIndex) => (
                          <td
                            className={`lens-table-cell${isRightAligned(column, frame.data!) ? ' lens-table-col-right' : ''}`}
                            key={column.field || `column-${columnIndex}`}
                            style={{
                              ...(column.widthPx ? { minWidth: `${column.widthPx}px` } : {}),
                              ...heatCellStyle(column, frame.data!, entry.row, columnStats.ranges),
                            }}
                          >
                            <ColumnCell
                              column={column}
                              frame={frame.data!}
                              row={entry.row}
                              panel={panel}
                              location={location}
                              max={columnStats.maxima.get(column.field) ?? 0}
                            />
                          </td>
                        ))}
                        {rowLeafAction && (
                          <td className="lens-table-action-cell">
                            <RowLeafAction
                              frame={frame.data!}
                              row={entry.row}
                              panel={panel}
                              location={location}
                              level={level}
                              label={translate('table.openRecord', 'Open record')}
                            />
                          </td>
                        )}
                        {hasHorizontalOverflow && <td aria-hidden="true" className="lens-table-scroll-spacer" />}
                      </tr>
                    ))}
                  </tbody>
                  {columns && frame.summary && Object.keys(frame.summary.values).length > 0 && (
                    <tfoot>
                      <tr>
                        {columns.map((column, index) => (
                          <td
                            className={`lens-table-cell${isRightAligned(column, frame.data!) ? ' lens-table-col-right' : ''}`}
                            key={column.field || `summary-${index}`}
                          >
                            {index === 0
                              ? frame.summary?.fullValues
                                ? translate('table.filteredTotal', 'Filtered total')
                                : translate('table.total', 'Total')
                              : column.total === true
                                ? <TableSummaryCell panel={panel} field={column.field} value={frame.summary?.values[column.field]} />
                                : <SummaryVoid />}
                          </td>
                        ))}
                        {rowLeafAction && <td />}
                        {hasHorizontalOverflow && <td aria-hidden="true" className="lens-table-scroll-spacer" />}
                      </tr>
                      {frame.summary.fullValues && (
                        <tr className="lens-table-summary-all">
                          {columns.map((column, index) => (
                            <td className={`lens-table-cell${isRightAligned(column, frame.data!) ? ' lens-table-col-right' : ''}`} key={column.field || `full-summary-${index}`}>
                              {index === 0
                                ? translate('table.allRowsTotal', 'All rows total')
                                : column.total === true
                                  ? <TableSummaryCell panel={panel} field={column.field} value={frame.summary?.fullValues?.[column.field]} />
                                  : <SummaryVoid />}
                            </td>
                          ))}
                          {rowLeafAction && <td />}
                          {hasHorizontalOverflow && <td aria-hidden="true" className="lens-table-scroll-spacer" />}
                        </tr>
                      )}
                    </tfoot>
                  )}
                </table>
              </div>
              <button
                aria-label={translate('table.scrollLeft', 'Scroll table left')}
                className="lens-table-overflow-edge lens-table-overflow-edge-left"
                disabled={!scrollEdges.left}
                onClick={() => scrollHorizontally(-1)}
                type="button"
              />
              <button
                aria-label={translate('table.scrollRight', 'Scroll table right')}
                className="lens-table-overflow-edge lens-table-overflow-edge-right"
                disabled={!scrollEdges.right}
                onClick={() => scrollHorizontally(1)}
                type="button"
              />
            </div>
            <footer className="lens-table-footer">
              <span className="lens-table-footer-notes">
                {/* Only a paginated table has a "this page" to scope sorting to.
                  On a table that shows every row at once the caveat describes a
                  limit that does not exist, and reads as a warning that some of
                  the data is out of sight. */}
                {(frame.summary?.filteredRows ?? dataRowCount) > rowCountDisclosureThreshold && (
                  <span className="lens-table-rowcount">
                    {frame.summary && frame.summary.totalRows > frame.summary.filteredRows
                      ? translate('table.filteredRowCount', '{filtered} of {total} rows', { filtered: frame.summary.filteredRows, total: frame.summary.totalRows })
                      : translate('table.rowCount', '{count} rows', { count: frame.summary?.filteredRows ?? dataRowCount })}
                  </span>
                )}
              </span>
              {frame.page && (
                <nav
                  aria-label={translate('table.pages', '{name} pages', { name: panel.title })}
                  className="lens-table-pagination"
                >
                  <button disabled={frame.isLoading || page <= 1} onClick={() => changePage(page - 1)} type="button">
                    {translate('table.previous', 'Previous')}
                  </button>
                  <span aria-live="polite">
                    {frame.isLoading && loadingPage !== page
                      ? translate('table.loadingPage', 'Loading page {n}', { n: loadingPage })
                      : translate('table.page', 'Page {n}', { n: page })}
                  </span>
                  <button disabled={frame.isLoading || !hasNext} onClick={() => changePage(page + 1)} type="button">
                    {translate('table.next', 'Next')}
                  </button>
                </nav>
              )}
            </footer>
          </div>
        </ColumnReferences.Provider>
      )}
    </PanelFrame>
  )
}

// GroupToggleRow renders the synthetic expander that stands in for a collapsed
// group. Its first cell is the toggle control (a chevron plus the producer's
// label, e.g. "Discontinued products (12)"); the remaining cells render the
// row's aggregate values (e.g. the group's total delta) without drill chrome.
function GroupToggleRow({
  columns, frame, row, panel, location, columnMaxima, columnRanges, expanded, onToggle, trailingSpacer,
}: {
  columns: Array<TableColumn>
  frame: Frame
  row: Array<unknown>
  panel: Panel
  location: URL
  columnMaxima: Map<string, number>
  columnRanges: Map<string, NumericRange>
  expanded: boolean
  onToggle: () => void
  trailingSpacer: boolean
}) {
  return (
    <tr className="lens-table-group-toggle">
      {columns.map((column, columnIndex) => {
        const index = frame.columns.findIndex((candidate) => candidate.name === column.field)
        const rawLabel = index >= 0 ? row[index] : undefined
        const label = typeof rawLabel === 'string' ? rawLabel : ''
        return (
          <td
            className={`lens-table-cell${isRightAligned(column, frame) ? ' lens-table-col-right' : ''}`}
            key={column.field || `column-${columnIndex}`}
            style={{
              ...(column.widthPx ? { minWidth: `${column.widthPx}px` } : {}),
              ...heatCellStyle(column, frame, row, columnRanges),
            }}
          >
            {columnIndex === 0 ? (
              <button aria-expanded={expanded} className="lens-table-group-toggle-btn" onClick={onToggle} type="button">
                <span aria-hidden="true" className={`lens-table-group-chevron${expanded ? ' lens-table-group-chevron-open' : ''}`}>
                  <CaretRight />
                </span>
                <span>{label}</span>
              </button>
            ) : (
              <ColumnCell
                column={column}
                frame={frame}
                row={row}
                panel={panel}
                location={location}
                max={columnMaxima.get(column.field) ?? 0}
              />
            )}
          </td>
        )
      })}
      {trailingSpacer && <td aria-hidden="true" className="lens-table-scroll-spacer" />}
    </tr>
  )
}

function RowLeafAction({
  frame, row, panel, location, level, label,
}: { frame: Frame; row: Array<unknown>; panel: Panel; location: URL; level?: Level; label: string }) {
  const action = actionForRow(panel, frame, row, level)
  const activation = useActionActivation(action)
  const href = activation.available ? resolveRowLeafActionURL(panel, frame, row, location, level) : undefined
  return href ? <a className="lens-leaf-action" href={href} onClick={activation.onClick(href)}>{label}</a> : null
}

function FrameRow({
  frame, row, index, panel, location, level, openRecordLabel, trailingSpacer,
}: {
  frame: Frame
  row: Array<unknown>
  index: number
  panel: Panel
  location: URL
  level?: Level
  openRecordLabel: string
  trailingSpacer: boolean
}) {
  const action = actionForRow(panel, frame, row, level)
  const activation = useActionActivation(action)
  const href = activation.available ? resolveRowLeafActionURL(panel, frame, row, location, level) : undefined
  return (
    <tr key={index}>
      {frame.columns.map((column, columnIndex) => (
        <td className={`lens-table-cell-${column.type}`} key={column.name}>
          <TableCell column={column} format={panel.format[column.name]} value={row[columnIndex]} />
        </td>
      ))}
      <td className="lens-table-action-cell">
        {href && <a className="lens-leaf-action" href={href} onClick={activation.onClick(href)}>{openRecordLabel}</a>}
      </td>
      {trailingSpacer && <td aria-hidden="true" className="lens-table-scroll-spacer" />}
    </tr>
  )
}
