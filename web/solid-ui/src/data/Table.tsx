import { createContext, splitProps, useContext, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export type SortDirection = 'asc' | 'desc' | 'none'
export type TableColumnPriority = 0 | 1 | 2 | 3

function priorityClass(priority?: number): string | undefined {
  if (priority !== undefined && priority >= 3) return 'max-lg:hidden'
  if (priority === 2) return 'max-md:hidden'
  return undefined
}

export interface TableProps extends JSX.HTMLAttributes<HTMLTableElement> {
  wrapperClass?: string
  scrollbarPosition?: 'top' | 'bottom'
  scrollbarGutter?: boolean
}

export function Table(props: TableProps) {
  const [local, native] = splitProps(props, ['wrapperClass', 'scrollbarPosition', 'scrollbarGutter', 'class', 'children'])
  const top = () => local.scrollbarPosition === 'top'
  return (
    <div class="relative table-scroll-wrap">
      <div class={classes('overflow-x-auto relative', top() && 'rotate-x-180', local.scrollbarGutter && 'table-scrollbar-gutter', local.wrapperClass)}>
        <table {...native} class={classes('min-w-full table bg-surface-600 text-sm rounded-lg', top() && '-rotate-x-180', local.class)}>{local.children}</table>
      </div>
    </div>
  )
}

export function Header(props: JSX.HTMLAttributes<HTMLTableSectionElement>) {
  const [local, native] = splitProps(props, ['class', 'children'])
  return <TableSectionContext.Provider value="header"><thead {...native} class={local.class}>{local.children}</thead></TableSectionContext.Provider>
}

export const TableHeader = Header

export function Body(props: JSX.HTMLAttributes<HTMLTableSectionElement>) {
  const [local, native] = splitProps(props, ['class', 'children'])
  return <tbody {...native} class={local.class}>{local.children}</tbody>
}

export const TableBody = Body

export interface RowProps extends JSX.HTMLAttributes<HTMLTableRowElement> {
  selected?: boolean
  onSelectedChange?: (selected: boolean, event: MouseEvent | KeyboardEvent) => void
}

const TableSectionContext = createContext<'header' | 'body'>('body')

export function Row(props: RowProps) {
  const [local, native] = splitProps(props, ['selected', 'onSelectedChange', 'class', 'children', 'onClick', 'onKeyDown'])
  const section = useContext(TableSectionContext)
  const selectable = () => local.onSelectedChange !== undefined
  const select = (event: MouseEvent | KeyboardEvent) => {
    const target = event.target
    if (target instanceof Element && target !== event.currentTarget && target.closest('a,button,input,select,textarea,summary')) return
    local.onSelectedChange?.(!local.selected, event)
  }
  return (
    <tr
      {...native}
      class={classes(section === 'header' && 'bg-surface-500 text-200', local.class)}
      aria-selected={local.selected === undefined ? undefined : local.selected}
      data-state={local.selected ? 'selected' : undefined}
      tabIndex={selectable() ? native.tabIndex ?? 0 : native.tabIndex}
      onClick={(event) => { select(event); if (typeof local.onClick === 'function') local.onClick(event) }}
      onKeyDown={(event) => {
        if (selectable() && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); select(event) }
        if (typeof local.onKeyDown === 'function') local.onKeyDown(event)
      }}
    >{local.children}</tr>
  )
}

export const TableRow = Row

function SortIcon(props: { direction: SortDirection }) {
  const points = () => props.direction === 'asc' ? '80 96 128 144 176 96' : '80 160 128 112 176 160'
  if (props.direction === 'none') return <svg aria-hidden="true" width="16" height="16" viewBox="0 0 256 256" class="opacity-30"><polyline points="80 96 128 48 176 96" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /><polyline points="80 160 128 208 176 160" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /></svg>
  return <svg aria-hidden="true" width="16" height="16" viewBox="0 0 256 256"><polyline points={points()} fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /></svg>
}

export interface HeadProps extends JSX.ThHTMLAttributes<HTMLTableCellElement> {
  sortable?: boolean
  sortDirection?: SortDirection
  onSort?: (direction: SortDirection, event: MouseEvent | KeyboardEvent) => void
  priority?: number
  truncateWidth?: number
  addonBottom?: JSX.Element
}

export function Head(props: HeadProps) {
  const [local, native] = splitProps(props, ['sortable', 'sortDirection', 'onSort', 'priority', 'truncateWidth', 'addonBottom', 'class', 'children', 'onClick', 'onKeyDown'])
  const sortable = () => local.sortable ?? local.onSort !== undefined
  const direction = () => local.sortDirection ?? 'none'
  const nextDirection = (): SortDirection => direction() === 'asc' ? 'desc' : 'asc'
  const activate = (event: MouseEvent | KeyboardEvent) => { if (sortable()) local.onSort?.(nextDirection(), event) }
  const ariaSort = () => direction() === 'asc' ? 'ascending' as const : direction() === 'desc' ? 'descending' as const : sortable() ? 'none' as const : undefined
  return (
    <th
      {...native}
      scope={native.scope ?? 'col'}
      aria-sort={ariaSort()}
      data-col-priority={local.priority && local.priority > 0 ? local.priority : undefined}
      tabIndex={sortable() ? native.tabIndex ?? 0 : native.tabIndex}
      class={classes('px-4 py-3 font-medium text-left border-r border-subtle last-of-type:border-r-0 border-b-0', sortable() && 'cursor-pointer hover:bg-surface-400 transition-colors', priorityClass(local.priority), local.class)}
      onClick={(event) => { activate(event); if (typeof local.onClick === 'function') local.onClick(event) }}
      onKeyDown={(event) => {
        if (sortable() && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); activate(event) }
        if (typeof local.onKeyDown === 'function') local.onKeyDown(event)
      }}
    >
      <div class="flex flex-col gap-1">
        <div class="flex items-center gap-2">
          {local.truncateWidth ? <span class="truncate" style={{ 'max-width': `${local.truncateWidth}px` }}>{local.children}</span> : sortable() ? <span>{local.children}</span> : local.children}
          {sortable() && <span class="sort-indicator"><SortIcon direction={direction()} /></span>}
        </div>
        {local.addonBottom}
      </div>
    </th>
  )
}

export const TableHead = Head

export interface CellProps extends JSX.TdHTMLAttributes<HTMLTableCellElement> { priority?: number }

export function Cell(props: CellProps) {
  const [local, native] = splitProps(props, ['priority', 'class', 'children'])
  return <td {...native} data-col-priority={local.priority && local.priority > 0 ? local.priority : undefined} class={classes('p-4 border-r border-subtle last-of-type:border-r-0', priorityClass(local.priority), local.class)}>{local.children}</td>
}

export const TableCell = Cell

export function Caption(props: JSX.HTMLAttributes<HTMLTableCaptionElement>) {
  const [local, native] = splitProps(props, ['class', 'children'])
  return <caption {...native} class={classes('mt-4 text-sm text-200', local.class)}>{local.children}</caption>
}

export const TableCaption = Caption
