/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { createContext, createEffect, createMemo, createSignal, For, onCleanup, Show, untrack, useContext, type JSX } from 'solid-js'
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
function SortIndicator(props: { direction?: SortDirection }) {
  return (
    <span
      aria-hidden="true"
      class={`lens-table-sort${props.direction ? ` lens-table-sort-${props.direction}` : ''}`}
    >
      {props.direction === 'ascending' && <CaretDown className="lens-table-sort-up" />}
      {props.direction === 'descending' && <CaretDown />}
      {!props.direction && <ArrowsLeftRight className="lens-table-sort-neutral" />}
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

function TableCell(props: { column: Column; format?: FieldFormat; value: unknown }) {
  const display = useCellFormat(props.column.name, props.format ?? inferredFormat(props.column))
  const translate = useTranslate()
  if (props.column.type === 'bool') {
    if (props.value === null || props.value === undefined || props.value === '') return <span class="lens-table-null">—</span>
    const checked = props.value === true || props.value === 1 || props.value === 'true'
    return <span class="lens-table-bool" data-value={checked}>
      {checked ? translate('table.boolean.yes', 'Yes') : translate('table.boolean.no', 'No')}
    </span>
  }
  const text = display(props.value)
  if (props.column.type === 'time' && text !== '—') {
    return <time datetime={typeof props.value === 'string' ? props.value : undefined}>{text}</time>
  }
  return <>{text}</>
}

function BarCell(props: { field?: string; format?: FieldFormat; value: unknown; max: number }) {
  const display = useCellFormat(props.field, props.format)
  const number = numericValue(props.value)
  const ratio = props.max > 0 && number !== undefined ? Math.max(0, Math.min(1, Math.abs(number) / props.max)) : 0
  const negative = number !== undefined && number < 0
  // An empty track is a bar that is loading, not a bar that is zero: five rows
  // of «0 / 0.0%» drew five full-width grey rails with nothing in them, which is
  // the runtime's own skeleton shape. A zero has no magnitude to draw, so it
  // draws none and the numeral carries the row.
  const drawn = number !== undefined && number !== 0
  return (
    <div class="lens-table-bar">
      {drawn && (
        <span class="lens-table-bar-track" aria-hidden="true">
          {/* The magnitude is drawn from the track's midpoint outwards so a
              negative value cannot look identical to its positive twin. */}
          <span
            class={`lens-table-bar-fill${negative ? ' lens-table-bar-fill-negative' : ''}`}
            style={{ width: `${ratio * 50}%`, [negative ? 'right' : 'left']: '50%' }}
          />
        </span>
      )}
      <span class="lens-table-bar-value">{display(props.value)}</span>
    </div>
  )
}

/**
 * A thin rule under the value, colored by sign. The rule spans the value it
 * underlines rather than encoding magnitude: a proportional rule degenerates
 * into something indistinguishable from a stray hyphen as soon as one row
 * dominates the column, which is exactly what the legacy treatment avoids.
 */
function UnderlineCell(props: { field?: string; format?: FieldFormat; value: unknown }) {
  const display = useCellFormat(props.field, props.format)
  const number = numericValue(props.value)
  const negative = number !== undefined && number < 0
  const blank = number === undefined
  return (
    <span class="lens-table-underline">
      <span class="lens-table-underline-value">{display(props.value)}</span>
      {!blank && (
        <span
          aria-hidden="true"
          class={`lens-table-underline-rule${negative ? ' lens-table-underline-rule-negative' : ''}`}
        />
      )}
    </span>
  )
}

function DeltaCell(props: {
  valueFormat?: FieldFormat
  value: unknown
  secondaryFormat?: FieldFormat
  secondaryValue: unknown
  stacked?: boolean
}) {
  const displayValue = useFormat(props.valueFormat)
  const displaySecondary = useFormat(props.secondaryFormat)
  const translate = useTranslate()
  const { document } = useDashboard()
  const absolute = numericValue(props.value)
  const secondary = numericValue(props.secondaryValue)
  const hasSecondary = props.secondaryValue !== null && props.secondaryValue !== undefined && props.secondaryValue !== ''
  const isNew = !hasSecondary && absolute !== undefined && absolute !== 0
  const negative = secondary !== undefined && secondary < 0
  // Percent changes beyond ±999.9% clamp to «>999%» / «<−999%»: a precise
  // «+13 417.3%» is noise, and the absolute value beside it carries the story.
  const clamped = secondary !== undefined ? clampedDeltaPercent(secondary, document.meta.locale) : undefined
  const percent = (hasSecondary || isNew) && (
    <span class={`lens-table-delta-pct${negative ? ' lens-table-delta-pct-negative' : ''}`}>
      {isNew
        ? translate('panel.trend.new', 'New')
        : clamped ?? <>{secondary !== undefined && secondary > 0 ? '+' : ''}{displaySecondary(props.secondaryValue)}</>}
    </span>
  )
  if (props.stacked) {
    // Stacked reads top-down: the relative change first, the absolute
    // amount beneath it as supporting detail.
    return (
      <span class="lens-table-delta lens-table-delta-stacked">
        {percent}
        <span class="lens-table-delta-value">{displayValue(props.value)}</span>
      </span>
    )
  }
  return (
    <span class="lens-table-delta">
      <span class="lens-table-delta-value">{displayValue(props.value)}</span>
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

function ColumnCell(props: {
  column: TableColumn
  frame: Frame
  row: Array<unknown>
  panel: Panel
  location: URL
  max: number
}) {
  const activation = useActionActivation(props.column.action)
  const translate = useTranslate()
  const index = props.frame.columns.findIndex((candidate) => candidate.name === props.column.field)
  const type = props.frame.columns[index]?.type ?? 'string'
  const value = index >= 0 ? props.row[index] : undefined
  const format = props.panel.format[props.column.field]

  let content: JSX.Element
  if (!props.column.field.trim() && props.column.text) {
    // An action-only column carries its own literal label; there is no field
    // to read and no value to format.
    content = <span class="lens-table-cell-text">{props.column.text}</span>
  } else if (props.column.cell.kind === 'bar') {
    content = <BarCell field={props.column.field} format={format} value={value} max={props.max} />
  } else if (props.column.cell.kind === 'underline') {
    content = <UnderlineCell field={props.column.field} format={format} value={value} />
  } else if (props.column.cell.kind === 'delta') {
    const secondaryField = props.column.cell.secondaryField
    const secondaryIndex = secondaryField ? props.frame.columns.findIndex((candidate) => candidate.name === secondaryField) : -1
    content = (
      <DeltaCell
        valueFormat={format}
        value={value}
        secondaryFormat={secondaryField ? props.panel.format[secondaryField] : undefined}
        secondaryValue={secondaryIndex >= 0 ? props.row[secondaryIndex] : undefined}
        stacked={props.column.cell.layout === 'stacked'}
      />
    )
  } else {
    content = <TableCell column={{ name: props.column.field, type }} format={format} value={value} />
  }

  // A per-row status tone tints only the value's color; the producer sets it
  // from its own business thresholds (e.g. a loss ratio over 100%).
  const tone = rowFieldString(props.frame, props.row, props.column.cell.toneField)
  if (tone === 'pos' || tone === 'warn' || tone === 'neg') {
    content = <span class={`lens-table-tone lens-table-tone-${tone}`}>{content}</span>
  }

  const href = props.column.action && untrack(activation.available)
    ? resolveColumnActionURL(props.column.action, props.frame, props.row, props.location)
    : undefined

  if (props.column.clamp) {
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
          class="lens-table-clamp"
          style={{ '-webkit-line-clamp': props.column.clamp } as JSX.CSSProperties}
          title={fullText}
        >
          {content}
        </span>
        {fullText && <span class="lens-table-clamp-full">{fullText}</span>}
      </>
    )
    // A cell that drills already has something focusable in it, and revealing
    // on `:focus-within` costs the reader no extra tab stop. A cell that does
    // not is given a real control — a disclosure — rather than a focusable
    // `span`, which is a tab stop that announces nothing.
    content = href || !fullText
      ? <span class="lens-table-clamp-host">{clamped}</span>
      : (
        <button class="lens-table-clamp-host" type="button">
          {clamped}
        </button>
      )
  }
  const pill = props.column.affordance === 'pill'
  const quiet = props.column.affordance === 'quiet'
  let rendered: JSX.Element
  if (href && quiet) {
    // The whole cell is the drill target; the underline and arrow stay hidden
    // until hover/focus so a dense table of quiet numerals reads as data first.
    rendered = (
      <a class="lens-table-cell-quiet" href={href} onClick={activation.onClick(href)}>
        <span class="lens-table-cell-quiet-value">{content}</span>
        <span aria-hidden="true" class="lens-table-cell-quiet-arrow"><ArrowUpRight /></span>
      </a>
    )
  } else if (href) {
    rendered = (
      <a class={`lens-table-cell-link${pill ? ' lens-table-cell-pill' : ''}`} href={href} onClick={activation.onClick(href)}>
        {content}
        {/* The arrow claims the cell opens something, so it only appears when
            a target actually resolved. */}
        <span class="lens-table-cell-link-arrow">{pill ? <ArrowUpRight /> : <CaretRight />}</span>
      </a>
    )
  } else if (pill) {
    // A pill without a resolvable target still marks the column as a drill
    // surface (the action may be renderer-local), but it does not pretend to
    // be a link.
    rendered = <span class="lens-table-cell-pill">{content}</span>
  } else {
    rendered = content
  }

  // A badge (e.g. an unmatched-source marker) rides beside the value; its
  // tooltip carries the producer's explanation.
  const badge = rowFieldString(props.frame, props.row, props.column.badgeField)
  const sampleIndex = props.column.sampleSizeField
    ? props.frame.columns.findIndex((candidate) => candidate.name === props.column.sampleSizeField)
    : -1
  const sampleSize = sampleIndex >= 0 ? numericValue(props.row[sampleIndex]) : undefined
  const minimum = (props.column.minSampleSize ?? 0) > 0 ? props.column.minSampleSize! : defaultMinimumSampleSize
  if (sampleSize !== undefined && sampleSize < minimum) {
    const smallSampleLabel = translate(
      'table.smallSample',
      'Small sample: n={count}, minimum {minimum}',
      { count: sampleSize, minimum },
    )
    rendered = (
      <span class="lens-table-small-sample">
        {rendered}
        <span class="lens-table-small-sample-marker">
          {translate('table.smallSampleShort', 'n<{minimum}', { minimum })}
          <InfoTip inline text={smallSampleLabel} />
        </span>
      </span>
    )
  }
  if (!badge) return rendered
  return (
    <span class="lens-table-cell-badged">
      {rendered}
      <span class="lens-table-cell-badge" title={badge}><InfoTip inline text={badge} /></span>
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
/* eslint-disable react-refresh/only-export-components */
export function heatRank(value: number, sorted: readonly number[]): number {
  if (sorted.length <= 1) return 0.5
  const first = sorted.indexOf(value)
  if (first < 0) return 0.5
  const last = sorted.lastIndexOf(value)
  return ((first + last) / 2) / (sorted.length - 1)
}

function heatCellStyle(column: TableColumn, frame: Frame, row: Array<unknown>, ranges: Map<string, NumericRange>): JSX.CSSProperties | undefined {
  if (!column.heat) return undefined
  const index = frame.columns.findIndex((candidate) => candidate.name === column.field)
  const value = index >= 0 ? numericValue(row[index]) : undefined
  const range = ranges.get(column.field)
  if (value === undefined || !range) return undefined
  const intensity = heatRank(value, range.sorted)
  const percentage = Math.round(heatMinimumMixPercent + Math.max(0, Math.min(1, intensity)) * heatMixRangePercent)
  return { 'background-color': `color-mix(in srgb, var(--lens-accent-500) ${percentage}%, var(--lens-bg-card))` }
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

function TableSummaryCell(props: { panel: Panel; field: string; value: unknown }) {
  const display = useFormat(props.panel.format[props.field])
  return <>{props.value === undefined ? <SummaryVoid /> : display(props.value)}</>
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
    <span class="lens-table-summary-void">
      <span aria-hidden="true">—</span>
      {/* TODO(i18n): register `table.notSummable` (en «Not summable», ru «Не суммируется»). */}
      <span class="lens-sr-only">{translate('table.notSummable', 'Not summable')}</span>
    </span>
  )
}

type RenderRow =
  | { row: Array<unknown>; index: number; kind: 'normal' | 'member' }
  | { row: Array<unknown>; index: number; kind: 'toggle'; group: string; expanded: boolean }

export function TablePanel(props: TablePanelProps) {
  const panel = props.panel
  const frame = usePanelFrame(panel.id)
  const { document, navigation } = useDashboard()
  const pagination = usePanelPagination()
  const translate = useTranslate()
  const [sort, setSort] = createSignal<SortState>()
  const [search, setSearch] = createSignal('')
  const [locationHref, setLocationHref] = createSignal(globalThis.location.href)
  let scrollRef: HTMLDivElement | undefined
  const [scrollEdges, setScrollEdges] = createSignal({ left: false, right: false })
  const [hasHorizontalOverflow, setHasHorizontalOverflow] = createSignal(false)
  let searchInitialized = false
  const [requestedPage, setRequestedPage] = createSignal(frame.page?.number ?? 1)
  let requestedSnapshotId = document.snapshotId
  const level = (): Level | undefined => navigation.panelId === panel.id && navigation.path.length
    ? levelForPath(document, navigation.path)
    : undefined
  // A static identity table (a fixed decomposition, not a record list) declares
  // sortable:false: its rows carry an inherent order, so offering to reorder
  // them — and printing "sort applies to this page" — would be a lie.
  const serverSort = Boolean(document?.endpoints?.panel && panel.table?.searchable)
  const sortEnabled = panel.presentation?.sortable !== false && (!frame.page || serverSort)
  // A server-searchable table is ordered by the same producer over the same
  // result scope; only an entirely static frame is safe to reorder locally.
  const rows = createMemo(() => frame.data ? sortedRows(frame.data, !serverSort && sortEnabled ? sort() : undefined) : [])
  const page = () => frame.page?.number ?? 1
  const pageSize = frame.page?.size
  const loadingPage = () => requestedSnapshotId === document.snapshotId ? requestedPage() : 1
  // The row-count check keeps pagination working with older servers that omit hasNext.
  const hasNext = () => frame.page?.hasNext ?? Boolean(pageSize && (frame.data?.rows.length ?? 0) >= pageSize)
  createEffect(() => {
    const syncLocation = () => setLocationHref(globalThis.location.href)
    globalThis.addEventListener('popstate', syncLocation)
    onCleanup(() => globalThis.removeEventListener('popstate', syncLocation))
  })
  const location = createMemo(() => new URL(locationHref()))
  const columns = panel.columns?.length ? panel.columns : undefined
  // Column maxima scale bar cells. Computing them per cell is quadratic in
  // row count, so they are derived once per frame.
  const columnStats = createMemo(() => {
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
  })
  // A panel-level leaf action applies to whole rows. In columns mode it has
  // no column of its own, so the table appends one; otherwise the action the
  // document declares would never reach the DOM.
  const rowLeafAction = Boolean(columns) && panel.actions.some((action) => (
    action.kind === 'navigate_to_leaf' || action.kind === 'open_drawer'
  ))

  // Row grouping collapses tagged member rows behind a synthetic toggle row.
  // The tag lives in a companion frame column named by presentation.rowGroupField.
  const [expandedGroups, setExpandedGroups] = createSignal<Record<string, boolean>>({})
  const rowGroupField = panel.presentation?.rowGroupField
  const groupIndex = columns && frame.data && rowGroupField
    ? frame.data.columns.findIndex((candidate) => candidate.name === rowGroupField)
    : -1
  const toggleGroup = (group: string) => setExpandedGroups((current) => ({ ...current, [group]: !current[group] }))
  const renderRows = createMemo<Array<RenderRow>>(() => {
    if (groupIndex < 0) return rows().map((entry) => ({ ...entry, kind: 'normal' as const }))
    const normal: Array<RenderRow> = []
    const toggles: Array<{ entry: { row: Array<unknown>; index: number }; group: string }> = []
    const members = new Map<string, Array<{ row: Array<unknown>; index: number }>>()
    for (const entry of rows()) {
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
      const expanded = Boolean(expandedGroups()[group])
      out.push({ ...entry, kind: 'toggle', group, expanded })
      if (expanded) for (const member of members.get(group) ?? []) out.push({ ...member, kind: 'member' })
    }
    return out
  })
  // The footer's "N rows" counts real data rows, never the synthetic toggle.
  const dataRowCount = groupIndex < 0
    ? (frame.data?.rows.length ?? 0)
    : (frame.data?.rows.reduce((count, row) => {
      const raw = row[groupIndex]
      return typeof raw === 'string' && raw.endsWith(':toggle') ? count : count + 1
    }, 0) ?? 0)
  // A short, unpaginated table has nothing to put in the footer, and the footer
  // is a bordered inset band — on a three-row explanatory table it reads as an
  // empty row the producer forgot to fill. Decide here whether it has anything
  // to say, so the band and its only two occupants appear and vanish together.
  const showsRowCount = (frame.summary?.filteredRows ?? dataRowCount) > rowCountDisclosureThreshold

  createEffect(() => {
    if (requestedSnapshotId !== document.snapshotId) {
      requestedSnapshotId = document.snapshotId
      setRequestedPage(1)
      setSearch('')
      searchInitialized = false
      return
    }
    if (frame.page?.number) setRequestedPage(frame.page.number)
  })

  createEffect(() => {
    if (!panel.table?.searchable) return
    if (!searchInitialized) {
      searchInitialized = true
      return
    }
    const currentSearch = search()
    const timeout = globalThis.setTimeout(() => { void pagination.search(panel.id, currentSearch) }, document.theme.debounceMs ?? 500)
    onCleanup(() => globalThis.clearTimeout(timeout))
  })

  const changePage = (next: number) => {
    setRequestedPage(next)
    void pagination.loadPage(panel.id, next)
  }

  const changeSort = (column: string) => {
    const next: SortState = sort()?.column === column
      ? { column, direction: sort()!.direction === 'ascending' ? 'descending' : 'ascending' }
      : { column, direction: 'ascending' as const }
    setSort(next)
    if (serverSort) void pagination.sort(panel.id, { field: column, direction: next.direction === 'ascending' ? 'asc' : 'desc' })
  }

  const sortDirection = (name: string) => sortEnabled && sort()?.column === name ? sort()!.direction : undefined
  const columnCount = columns ? columns.length + (rowLeafAction ? 1 : 0) : (frame.data?.columns.length ?? 0) + 1

  createEffect(() => {
    const data = frame.data
    const scroller = scrollRef
    if (!data || !scroller) return
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
      const contentWidth = Math.max(0, tableWidth - (untrack(hasHorizontalOverflow) ? stickyWidth : 0))
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
    onCleanup(() => {
      scroller.removeEventListener('scroll', measure)
      observer?.disconnect()
      if (animationFrame !== undefined) globalThis.cancelAnimationFrame(animationFrame)
      scroller.parentElement?.style.removeProperty('--lens-table-sticky-width')
    })
  })

  const scrollHorizontally = (direction: -1 | 1) => {
    const scroller = scrollRef
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
      <Show when={frame.data}>
        <ColumnReferences.Provider value={columnStats().references}>
          <div class="lens-table-view">
            {panel.table?.searchable && (
              <label class="lens-table-search">
                <span class="lens-sr-only">{translate('table.search', 'Search table')}</span>
                <input
                  aria-label={translate('table.search', 'Search table')}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder={translate('table.searchPlaceholder', 'Search all rows…')}
                  type="search"
                  value={search()}
                />
              </label>
            )}
            <div
              class="lens-table-scroll-frame"
              data-overflow-left={scrollEdges().left}
              data-overflow-right={scrollEdges().right}
            >
              {/* eslint-disable jsx-a11y/no-noninteractive-tabindex -- overflowing native scroll regions must be keyboard-focusable. */}
              <div
                aria-label={translate('table.scrollRegion', 'Scrollable table')}
                class="lens-table-scroll"
                ref={(el) => { scrollRef = el }}
                role="region"
                tabIndex={scrollEdges().left || scrollEdges().right ? 0 : undefined}
              >
                {/* eslint-enable jsx-a11y/no-noninteractive-tabindex */}
                <table class={`lens-table${columnCount >= 4 ? ' lens-table-wide' : ''}`}>
                  <thead>
                    <tr>
                      {columns ? (
                        <>
                          <For each={columns}>
                            {(column) => {
                              // An action-only column has no field to sort by, and a
                              // static table offers no sort at all; either way the
                              // heading is a plain label, not a control.
                              const sortable = sortEnabled && Boolean(column.field.trim())
                              return (
                                <th
                                  aria-sort={sortable ? (sort()?.column === column.field ? sort()!.direction : 'none') : undefined}
                                  class={frame.data && isRightAligned(column, frame.data) ? 'lens-table-col-right' : undefined}
                                  scope="col"
                                  style={column.widthPx ? { 'min-width': `${column.widthPx}px` } : undefined}
                                >
                                  {sortable ? (
                                    <button type="button" onClick={() => changeSort(column.field)}>
                                      <span>{column.label}</span>
                                      <SortIndicator direction={sortDirection(column.field)} />
                                    </button>
                                  ) : (
                                    <span class="lens-table-heading-static">{column.label}</span>
                                  )}
                                </th>
                              )
                            }}
                          </For>
                          {rowLeafAction && (
                            <th class="lens-table-action-heading" scope="col">
                              <span class="lens-sr-only">{translate('table.actions', 'Actions')}</span>
                            </th>
                          )}
                        </>
                      ) : (
                        <>
                          <For each={frame.data?.columns ?? []}>
                            {(column) => (
                              <th
                                aria-sort={sortEnabled ? (sort()?.column === column.name ? sort()!.direction : 'none') : undefined}
                                /* The body cell for these types is right-aligned
                                 (`.lens-table-cell-number`, `-time`); the header
                                 follows the data it heads. */
                                class={column.type === 'number' || column.type === 'time' ? 'lens-table-col-right' : undefined}
                                scope="col"
                              >
                                {sortEnabled ? (
                                  <button type="button" onClick={() => changeSort(column.name)}>
                                    <span>{column.name}</span>
                                    <SortIndicator direction={sortDirection(column.name)} />
                                  </button>
                                ) : (
                                  <span class="lens-table-heading-static">{column.name}</span>
                                )}
                              </th>
                            )}
                          </For>
                          <th class="lens-table-action-heading" scope="col">
                            <span class="lens-sr-only">{translate('table.actions', 'Actions')}</span>
                          </th>
                        </>
                      )}
                      {hasHorizontalOverflow() && <th aria-hidden="true" class="lens-table-scroll-spacer" />}
                    </tr>
                  </thead>
                  <tbody>
                    {rows().length === 0 ? (
                      <tr>
                        <td class="lens-table-empty" colSpan={columnCount}>
                          {translate('table.emptyPage', 'No records on this page')}
                        </td>
                        {hasHorizontalOverflow() && <td aria-hidden="true" class="lens-table-scroll-spacer" />}
                      </tr>
                    ) : (
                      <For each={renderRows()}>
                        {(entry) => !columns ? (
                          <FrameRow
                            frame={frame.data!}
                            index={entry.index}
                            location={location()}
                            openRecordLabel={translate('table.openRecord', 'Open record')}
                            panel={panel}
                            row={entry.row}
                            trailingSpacer={hasHorizontalOverflow()}
                          />
                        ) : entry.kind === 'toggle' ? (
                          <GroupToggleRow
                            columnMaxima={columnStats().maxima}
                            columnRanges={columnStats().ranges}
                            columns={columns}
                            expanded={entry.expanded}
                            frame={frame.data!}
                            location={location()}
                            onToggle={() => toggleGroup(entry.group)}
                            panel={panel}
                            row={entry.row}
                            trailingSpacer={hasHorizontalOverflow()}
                          />
                        ) : (
                          <tr class={entry.kind === 'member' ? 'lens-table-group-member' : undefined}>
                            <For each={columns}>
                              {(column) => (
                                <td
                                  class={`lens-table-cell${frame.data && isRightAligned(column, frame.data) ? ' lens-table-col-right' : ''}`}
                                  style={{
                                    ...(column.widthPx ? { 'min-width': `${column.widthPx}px` } : {}),
                                    ...(frame.data ? heatCellStyle(column, frame.data, entry.row, columnStats().ranges) : {}),
                                  }}
                                >
                                  <ColumnCell
                                    column={column}
                                    frame={frame.data!}
                                    location={location()}
                                    max={columnStats().maxima.get(column.field) ?? 0}
                                    panel={panel}
                                    row={entry.row}
                                  />
                                </td>
                              )}
                            </For>
                            {rowLeafAction && (
                              <td class="lens-table-action-cell">
                                <RowLeafAction
                                  frame={frame.data!}
                                  label={translate('table.openRecord', 'Open record')}
                                  level={level()}
                                  location={location()}
                                  panel={panel}
                                  row={entry.row}
                                />
                              </td>
                            )}
                            {hasHorizontalOverflow() && <td aria-hidden="true" class="lens-table-scroll-spacer" />}
                          </tr>
                        )}
                      </For>
                    )}
                  </tbody>
                  {columns && frame.summary && Object.keys(frame.summary.values).length > 0 && (
                    <tfoot>
                      <tr>
                        <For each={columns}>
                          {(column, index) => (
                            <td
                              class={`lens-table-cell${frame.data && isRightAligned(column, frame.data) ? ' lens-table-col-right' : ''}`}
                            >
                              {index() === 0
                                ? frame.summary?.fullValues
                                  ? translate('table.filteredTotal', 'Filtered total')
                                  : translate('table.total', 'Total')
                                : column.total === true
                                  ? <TableSummaryCell field={column.field} panel={panel} value={frame.summary?.values[column.field]} />
                                  : <SummaryVoid />}
                            </td>
                          )}
                        </For>
                        {rowLeafAction && <td />}
                        {hasHorizontalOverflow() && <td aria-hidden="true" class="lens-table-scroll-spacer" />}
                      </tr>
                      {frame.summary.fullValues && (
                        <tr class="lens-table-summary-all">
                          <For each={columns}>
                            {(column, index) => (
                              <td class={`lens-table-cell${frame.data && isRightAligned(column, frame.data) ? ' lens-table-col-right' : ''}`}>
                                {index() === 0
                                  ? translate('table.allRowsTotal', 'All rows total')
                                  : column.total === true
                                    ? <TableSummaryCell field={column.field} panel={panel} value={frame.summary?.fullValues?.[column.field]} />
                                    : <SummaryVoid />}
                              </td>
                            )}
                          </For>
                          {rowLeafAction && <td />}
                          {hasHorizontalOverflow() && <td aria-hidden="true" class="lens-table-scroll-spacer" />}
                        </tr>
                      )}
                    </tfoot>
                  )}
                </table>
              </div>
              <button
                aria-label={translate('table.scrollLeft', 'Scroll table left')}
                class="lens-table-overflow-edge lens-table-overflow-edge-left"
                disabled={!scrollEdges().left}
                onClick={() => scrollHorizontally(-1)}
                type="button"
              />
              <button
                aria-label={translate('table.scrollRight', 'Scroll table right')}
                class="lens-table-overflow-edge lens-table-overflow-edge-right"
                disabled={!scrollEdges().right}
                onClick={() => scrollHorizontally(1)}
                type="button"
              />
            </div>
            {(showsRowCount || frame.page) && (
              <footer class="lens-table-footer">
                <span class="lens-table-footer-notes">
                  {/* Only a paginated table has a "this page" to scope sorting to.
                  On a table that shows every row at once the caveat describes a
                  limit that does not exist, and reads as a warning that some of
                  the data is out of sight. */}
                  {showsRowCount && (
                    <span class="lens-table-rowcount">
                      {frame.summary && frame.summary.totalRows > frame.summary.filteredRows
                        ? translate('table.filteredRowCount', '{filtered} of {total} rows', { filtered: frame.summary.filteredRows, total: frame.summary.totalRows })
                        : translate('table.rowCount', '{count} rows', { count: frame.summary?.filteredRows ?? dataRowCount })}
                    </span>
                  )}
                </span>
                {frame.page && (
                  <nav
                    aria-label={translate('table.pages', '{name} pages', { name: panel.title })}
                    class="lens-table-pagination"
                  >
                    <button disabled={frame.isLoading || page() <= 1} onClick={() => changePage(page() - 1)} type="button">
                      {translate('table.previous', 'Previous')}
                    </button>
                    <span aria-live="polite">
                      {frame.isLoading && loadingPage() !== page()
                        ? translate('table.loadingPage', 'Loading page {n}', { n: loadingPage() })
                        : translate('table.page', 'Page {n}', { n: page() })}
                    </span>
                    <button disabled={frame.isLoading || !hasNext()} onClick={() => changePage(page() + 1)} type="button">
                      {translate('table.next', 'Next')}
                    </button>
                  </nav>
                )}
              </footer>
            )}
          </div>
        </ColumnReferences.Provider>
      </Show>
    </PanelFrame>
  )
}

// GroupToggleRow renders the synthetic expander that stands in for a collapsed
// group. Its first cell is the toggle control (a chevron plus the producer's
// label, e.g. "Discontinued products (12)"); the remaining cells render the
// row's aggregate values (e.g. the group's total delta) without drill chrome.
function GroupToggleRow(props: {
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
    <tr class="lens-table-group-toggle">
      <For each={props.columns}>
        {(column, columnIndex) => {
          const index = props.frame.columns.findIndex((candidate) => candidate.name === column.field)
          const rawLabel = index >= 0 ? props.row[index] : undefined
          const label = typeof rawLabel === 'string' ? rawLabel : ''
          return (
            <td
              class={`lens-table-cell${isRightAligned(column, props.frame) ? ' lens-table-col-right' : ''}`}
              style={{
                ...(column.widthPx ? { 'min-width': `${column.widthPx}px` } : {}),
                ...heatCellStyle(column, props.frame, props.row, props.columnRanges),
              }}
            >
              {columnIndex() === 0 ? (
                <button aria-expanded={props.expanded} class="lens-table-group-toggle-btn" onClick={props.onToggle} type="button">
                  <span aria-hidden="true" class={`lens-table-group-chevron${props.expanded ? ' lens-table-group-chevron-open' : ''}`}>
                    <CaretRight />
                  </span>
                  <span>{label}</span>
                </button>
              ) : (
                <ColumnCell
                  column={column}
                  frame={props.frame}
                  location={props.location}
                  max={props.columnMaxima.get(column.field) ?? 0}
                  panel={props.panel}
                  row={props.row}
                />
              )}
            </td>
          )
        }}
      </For>
      {props.trailingSpacer && <td aria-hidden="true" class="lens-table-scroll-spacer" />}
    </tr>
  )
}

function RowLeafAction(props: {
  frame: Frame
  row: Array<unknown>
  panel: Panel
  location: URL
  level?: Level
  label: string
}) {
  const action = actionForRow(props.panel, props.frame, props.row, props.level)
  const activation = useActionActivation(action)
  const href = untrack(activation.available) ? resolveRowLeafActionURL(props.panel, props.frame, props.row, props.location, props.level) : undefined
  return href ? <a class="lens-leaf-action" href={href} onClick={activation.onClick(href)}>{props.label}</a> : null
}

function FrameRow(props: {
  frame: Frame
  row: Array<unknown>
  index: number
  panel: Panel
  location: URL
  level?: Level
  openRecordLabel: string
  trailingSpacer: boolean
}) {
  const action = actionForRow(props.panel, props.frame, props.row, props.level)
  const activation = useActionActivation(action)
  const href = untrack(activation.available) ? resolveRowLeafActionURL(props.panel, props.frame, props.row, props.location, props.level) : undefined
  return (
    <tr>
      <For each={props.frame.columns}>
        {(column, columnIndex) => (
          <td class={`lens-table-cell-${column.type}`}>
            <TableCell column={column} format={props.panel.format[column.name]} value={props.row[columnIndex()]} />
          </td>
        )}
      </For>
      <td class="lens-table-action-cell">
        {href && <a class="lens-leaf-action" href={href} onClick={activation.onClick(href)}>{props.openRecordLabel}</a>}
      </td>
      {props.trailingSpacer && <td aria-hidden="true" class="lens-table-scroll-spacer" />}
    </tr>
  )
}
