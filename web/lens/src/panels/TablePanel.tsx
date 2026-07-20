import { useEffect, useMemo, useRef, useState, type CSSProperties } from 'react'
import type { Column, FieldFormat, Frame, Level, Panel, TableColumn } from '../contract'
import { resolveColumnActionURL, resolveRowLeafActionURL } from '../explore/actions'
import { levelForPath, useDashboard, useFormat, usePanelFrame, usePanelPagination, useTranslate } from '../runtime'
import { PanelFrame } from './PanelFrame'

type SortDirection = 'ascending' | 'descending'

interface SortState {
  column: string
  direction: SortDirection
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

function TableCell({ column, format, value }: { column: Column; format?: FieldFormat; value: unknown }) {
  const display = useFormat(format ?? inferredFormat(column))
  if (column.type === 'bool') {
    if (value === null || value === undefined || value === '') return <span className="lens-table-null">—</span>
    const checked = value === true || value === 1 || value === 'true'
    return <span className="lens-table-bool" data-value={checked}>{checked ? 'Yes' : 'No'}</span>
  }
  const text = display(value)
  if (column.type === 'time' && text !== '—') {
    return <time dateTime={typeof value === 'string' ? value : undefined}>{text}</time>
  }
  return <>{text}</>
}

function BarCell({ format, value, max }: { format?: FieldFormat; value: unknown; max: number }) {
  const display = useFormat(format)
  const number = numericValue(value)
  const ratio = max > 0 && number !== undefined ? Math.max(0, Math.min(1, Math.abs(number) / max)) : 0
  const negative = number !== undefined && number < 0
  return (
    <div className="lens-table-bar">
      <span className="lens-table-bar-track" aria-hidden="true">
        {/* The magnitude is drawn from the track's midpoint outwards so a
            negative value cannot look identical to its positive twin. */}
        <span
          className={`lens-table-bar-fill${negative ? ' lens-table-bar-fill-negative' : ''}`}
          style={{ width: `${ratio * 50}%`, [negative ? 'right' : 'left']: '50%' }}
        />
      </span>
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
function UnderlineCell({ format, value }: { format?: FieldFormat; value: unknown }) {
  const display = useFormat(format)
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
  const secondary = numericValue(secondaryValue)
  const hasSecondary = secondaryValue !== null && secondaryValue !== undefined && secondaryValue !== ''
  const negative = secondary !== undefined && secondary < 0
  const percent = hasSecondary && (
    <span className={`lens-table-delta-pct${negative ? ' lens-table-delta-pct-negative' : ''}`}>
      {secondary !== undefined && secondary > 0 ? '+' : ''}{displaySecondary(secondaryValue)}
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

function ColumnCell({
  column, frame, row, panel, location, max,
}: { column: TableColumn; frame: Frame; row: Array<unknown>; panel: Panel; location: URL; max: number }) {
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
    content = <BarCell format={format} value={value} max={max} />
  } else if (column.cell.kind === 'underline') {
    content = <UnderlineCell format={format} value={value} />
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

  if (column.clamp) {
    content = (
      <span className="lens-table-clamp" style={{ WebkitLineClamp: column.clamp } as CSSProperties}>{content}</span>
    )
  }

  const href = column.action ? resolveColumnActionURL(column.action, frame, row, location) : undefined
  const pill = column.affordance === 'pill'
  if (href) {
    return (
      <a className={`lens-table-cell-link${pill ? ' lens-table-cell-pill' : ''}`} href={href}>
        {content}
        {/* The arrow claims the cell opens something, so it only appears when
            a target actually resolved. */}
        <span aria-hidden="true" className="lens-table-cell-link-arrow">{pill ? '↗' : '→'}</span>
      </a>
    )
  }
  if (pill) {
    // A pill without a resolvable target still marks the column as a drill
    // surface (the action may be renderer-local), but it does not pretend to
    // be a link.
    return <span className="lens-table-cell-pill">{content}</span>
  }
  return content
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

export function TablePanel({ panel }: TablePanelProps) {
  const frame = usePanelFrame(panel.id)
  const { document, navigation } = useDashboard()
  const pagination = usePanelPagination()
  const translate = useTranslate()
  const [sort, setSort] = useState<SortState>()
  const [requestedPage, setRequestedPage] = useState(frame.page?.number ?? 1)
  const requestedSnapshotId = useRef(document.snapshotId)
  const level: Level | undefined = navigation.panelId === panel.id && navigation.path.length
    ? levelForPath(document, navigation.path)
    : undefined
  const rows = useMemo(() => frame.data ? sortedRows(frame.data, sort) : [], [frame.data, sort])
  const page = frame.page?.number ?? 1
  const pageSize = frame.page?.size
  const loadingPage = requestedSnapshotId.current === document.snapshotId ? requestedPage : 1
  // The row-count check keeps pagination working with older servers that omit hasNext.
  const hasNext = frame.page?.hasNext ?? Boolean(pageSize && (frame.data?.rows.length ?? 0) >= pageSize)
  const location = new URL(globalThis.location.href)
  const columns = panel.columns?.length ? panel.columns : undefined
  // Column maxima scale bar cells. Computing them per cell is quadratic in
  // row count, so they are derived once per frame.
  const columnMaxima = useMemo(() => {
    const maxima = new Map<string, number>()
    if (!frame.data || !columns) return maxima
    for (const column of columns) {
      if (column.cell.kind !== 'bar') continue
      const index = frame.data.columns.findIndex((candidate) => candidate.name === column.field)
      if (index < 0) continue
      maxima.set(column.field, frame.data.rows.reduce((accumulator, row) => {
        const number = numericValue(row[index])
        return number === undefined ? accumulator : Math.max(accumulator, Math.abs(number))
      }, 0))
    }
    return maxima
  }, [columns, frame.data])
  // A panel-level leaf action applies to whole rows. In columns mode it has
  // no column of its own, so the table appends one; otherwise the action the
  // document declares would never reach the DOM.
  const rowLeafAction = Boolean(columns) && panel.actions.some((action) => action.kind === 'navigate_to_leaf')

  useEffect(() => {
    if (requestedSnapshotId.current !== document.snapshotId) {
      requestedSnapshotId.current = document.snapshotId
      setRequestedPage(1)
      return
    }
    if (frame.page?.number) setRequestedPage(frame.page.number)
  }, [document.snapshotId, frame.page?.number])

  const changePage = (next: number) => {
    setRequestedPage(next)
    void pagination.loadPage(panel.id, next)
  }

  const changeSort = (column: string) => {
    setSort((current) => current?.column === column
      ? { column, direction: current.direction === 'ascending' ? 'descending' : 'ascending' }
      : { column, direction: 'ascending' })
  }

  const sortIndicator = (name: string) => sort?.column === name
    ? sort.direction === 'ascending' ? '↑' : '↓'
    : '↕'
  const columnCount = columns ? columns.length + (rowLeafAction ? 1 : 0) : (frame.data?.columns.length ?? 0) + 1

  return (
    <PanelFrame panel={panel} frame={frame} allowEmptyContent={Boolean(frame.page)}>
      {frame.data && (
        <div className="lens-table-view">
          <div className="lens-table-scroll">
            <table className="lens-table">
              <thead>
                <tr>
                  {columns ? (
                    <>
                      {columns.map((column, columnIndex) => {
                        // An action-only column has no field to sort by;
                        // offering a sort control there would be a lie.
                        const sortable = Boolean(column.field.trim())
                        return (
                          <th
                            aria-sort={sortable && sort?.column === column.field ? sort.direction : 'none'}
                            className={column.align === 'right' ? 'lens-table-col-right' : undefined}
                            key={column.field || `column-${columnIndex}`}
                            scope="col"
                            style={column.widthPx ? { minWidth: `${column.widthPx}px` } : undefined}
                          >
                            {sortable ? (
                              <button type="button" onClick={() => changeSort(column.field)}>
                                <span>{column.label}</span>
                                <span aria-hidden="true">{sortIndicator(column.field)}</span>
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
                        <th aria-sort={sort?.column === column.name ? sort.direction : 'none'} key={column.name} scope="col">
                          <button type="button" onClick={() => changeSort(column.name)}>
                            <span>{column.name}</span>
                            <span aria-hidden="true">{sortIndicator(column.name)}</span>
                          </button>
                        </th>
                      ))}
                      <th className="lens-table-action-heading" scope="col">
                        <span className="lens-sr-only">{translate('table.actions', 'Actions')}</span>
                      </th>
                    </>
                  )}
                </tr>
              </thead>
              <tbody>
                {rows.length === 0 ? (
                  <tr>
                    <td className="lens-table-empty" colSpan={columnCount}>
                      {translate('table.emptyPage', 'No records on this page')}
                    </td>
                  </tr>
                ) : rows.map(({ row, index }) => columns ? (
                  <tr key={index}>
                    {columns.map((column, columnIndex) => (
                      <td
                        className={`lens-table-cell${column.align === 'right' ? ' lens-table-col-right' : ''}`}
                        key={column.field || `column-${columnIndex}`}
                        style={column.widthPx ? { minWidth: `${column.widthPx}px` } : undefined}
                      >
                        <ColumnCell
                          column={column}
                          frame={frame.data!}
                          row={row}
                          panel={panel}
                          location={location}
                          max={columnMaxima.get(column.field) ?? 0}
                        />
                      </td>
                    ))}
                    {rowLeafAction && (
                      <td className="lens-table-action-cell">
                        <RowLeafAction
                          frame={frame.data!}
                          row={row}
                          panel={panel}
                          location={location}
                          level={level}
                          label={translate('table.openRecord', 'Open record')}
                        />
                      </td>
                    )}
                  </tr>
                ) : (
                  <FrameRow
                    frame={frame.data!}
                    row={row}
                    index={index}
                    panel={panel}
                    location={location}
                    level={level}
                    openRecordLabel={translate('table.openRecord', 'Open record')}
                    key={index}
                  />
                ))}
              </tbody>
            </table>
          </div>
          <footer className="lens-table-footer">
            <span className="lens-table-sort-scope">{translate('table.sortScope', 'Sort applies to this page only')}</span>
            {frame.page && (
              <nav aria-label={`${panel.title} pages`} className="lens-table-pagination">
                <button disabled={frame.isLoading || page <= 1} onClick={() => changePage(page - 1)} type="button">
                  {translate('table.previous', 'Previous')}
                </button>
                <span aria-live="polite">
                  {frame.isLoading && loadingPage !== page
                    ? translate('table.loadingPage', 'Loading page {n}').replace('{n}', String(loadingPage))
                    : translate('table.page', 'Page {n}').replace('{n}', String(page))}
                </span>
                <button disabled={frame.isLoading || !hasNext} onClick={() => changePage(page + 1)} type="button">
                  {translate('table.next', 'Next')}
                </button>
              </nav>
            )}
          </footer>
        </div>
      )}
    </PanelFrame>
  )
}

function RowLeafAction({
  frame, row, panel, location, level, label,
}: { frame: Frame; row: Array<unknown>; panel: Panel; location: URL; level?: Level; label: string }) {
  const href = resolveRowLeafActionURL(panel, frame, row, location, level)
  return href ? <a className="lens-leaf-action" href={href}>{label}</a> : null
}

function FrameRow({
  frame, row, index, panel, location, level, openRecordLabel,
}: {
  frame: Frame
  row: Array<unknown>
  index: number
  panel: Panel
  location: URL
  level?: Level
  openRecordLabel: string
}) {
  const href = resolveRowLeafActionURL(panel, frame, row, location, level)
  return (
    <tr key={index}>
      {frame.columns.map((column, columnIndex) => (
        <td className={`lens-table-cell-${column.type}`} key={column.name}>
          <TableCell column={column} format={panel.format[column.name]} value={row[columnIndex]} />
        </td>
      ))}
      <td className="lens-table-action-cell">
        {href && <a className="lens-leaf-action" href={href}>{openRecordLabel}</a>}
      </td>
    </tr>
  )
}
