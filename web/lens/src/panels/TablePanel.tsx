import { useEffect, useMemo, useRef, useState } from 'react'
import type { Column, FieldFormat, Frame, Level, Panel } from '../contract'
import { resolveRowLeafActionURL } from '../explore/actions'
import { levelForPath, useDashboard, useFormat, usePanelFrame, usePanelPagination } from '../runtime'
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

  return (
    <PanelFrame panel={panel} frame={frame} allowEmptyContent={Boolean(frame.page)}>
      {frame.data && (
        <div className="lens-table-view">
          <div className="lens-table-scroll">
            <table className="lens-table">
              <thead>
                <tr>
                  {frame.data.columns.map((column) => (
                    <th aria-sort={sort?.column === column.name ? sort.direction : 'none'} key={column.name} scope="col">
                      <button type="button" onClick={() => changeSort(column.name)}>
                        <span>{column.name}</span>
                        <span aria-hidden="true">{sort?.column === column.name ? sort.direction === 'ascending' ? '↑' : '↓' : '↕'}</span>
                      </button>
                    </th>
                  ))}
                  <th className="lens-table-action-heading" scope="col"><span className="lens-sr-only">Actions</span></th>
                </tr>
              </thead>
              <tbody>
                {rows.length === 0 ? (
                  <tr><td className="lens-table-empty" colSpan={frame.data.columns.length + 1}>No records on this page</td></tr>
                ) : rows.map(({ row, index }) => {
                  const href = resolveRowLeafActionURL(panel, frame.data!, row, location, level)
                  return (
                    <tr key={index}>
                      {frame.data!.columns.map((column, columnIndex) => (
                        <td className={`lens-table-cell-${column.type}`} key={column.name}>
                          <TableCell column={column} format={panel.format[column.name]} value={row[columnIndex]} />
                        </td>
                      ))}
                      <td className="lens-table-action-cell">
                        {href && <a className="lens-leaf-action" href={href}>Open record</a>}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
          <footer className="lens-table-footer">
            <span className="lens-table-sort-scope">Sort applies to this page only</span>
            {frame.page && (
              <nav aria-label={`${panel.title} pages`} className="lens-table-pagination">
                <button disabled={frame.isLoading || page <= 1} onClick={() => changePage(page - 1)} type="button">Previous</button>
                <span aria-live="polite">{frame.isLoading && loadingPage !== page ? `Loading page ${loadingPage}` : `Page ${page}`}</span>
                <button disabled={frame.isLoading || !hasNext} onClick={() => changePage(page + 1)} type="button">Next</button>
              </nav>
            )}
          </footer>
        </div>
      )}
    </PanelFrame>
  )
}
