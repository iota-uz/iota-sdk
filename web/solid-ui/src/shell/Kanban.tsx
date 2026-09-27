import { createUniqueId, For, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export interface KanbanMove { key: string; oldIndex: number; newIndex: number }
export interface KanbanCardMove extends KanbanMove { oldColumn: string; newColumn: string }

export interface KanbanBoardProps extends JSX.HTMLAttributes<HTMLDivElement> { label?: string }
export function KanbanBoard(props: KanbanBoardProps) { const [local, native] = splitProps(props, ['label', 'class', 'children']); return <div class="contents"><div {...native} class={classes('flex bg-gray-200 rounded-lg h-full overflow-hidden', local.class)}><div class="w-full overflow-x-auto overscroll-x-contain [touch-action:pan-x_pinch-zoom] [-webkit-overflow-scrolling:touch]"><ol aria-label={local.label} class="flex w-max min-w-full divide-x divide-subtle">{local.children}</ol></div></div></div> }

export interface KanbanColumnProps extends Omit<JSX.LiHTMLAttributes<HTMLLIElement>, 'title'> { columnKey: string; title: JSX.Element; cardsLabel?: string; cardsId?: string; onMove?: (move: KanbanMove) => void; index?: number; columnCount?: number }
export function KanbanColumn(props: KanbanColumnProps) {
  const [local, native] = splitProps(props, ['columnKey', 'title', 'cardsLabel', 'cardsId', 'onMove', 'index', 'columnCount', 'class', 'children', 'onKeyDown'])
  const instance = createUniqueId()
  const move = (delta: number) => { const oldIndex = local.index ?? 0; const newIndex = Math.max(0, Math.min((local.columnCount ?? oldIndex + 1) - 1, oldIndex + delta)); if (newIndex !== oldIndex) local.onMove?.({ key: local.columnKey, oldIndex, newIndex }) }
  return <li {...native} data-col-key={local.columnKey} class={classes('flex w-72 shrink-0 flex-col gap-2 p-3', local.class)} tabIndex={local.onMove ? native.tabIndex ?? 0 : native.tabIndex} onKeyDown={(event) => { if (event.altKey && (event.key === 'ArrowLeft' || event.key === 'ArrowRight')) { event.preventDefault(); move(event.key === 'ArrowLeft' ? -1 : 1) } if (typeof local.onKeyDown === 'function') local.onKeyDown(event) }}><header class="font-medium">{local.title}</header><ol aria-label={local.cardsLabel ?? `${typeof local.title === 'string' ? local.title : local.columnKey} cards`} class="flex min-h-6 flex-col gap-2" id={local.cardsId ?? `kanban-column-cards-${instance}-${local.columnKey}`} data-col-key={local.columnKey}>{local.children}</ol></li>
}

export interface KanbanCardProps extends JSX.LiHTMLAttributes<HTMLLIElement> { cardKey: string; columnKey?: string; index?: number; onActivate?: (key: string, event: MouseEvent | KeyboardEvent) => void; onMove?: (move: KanbanCardMove) => void }
export function KanbanCard(props: KanbanCardProps) {
  const [local, native] = splitProps(props, ['cardKey', 'columnKey', 'index', 'onActivate', 'onMove', 'class', 'children', 'onClick', 'onKeyDown'])
  const activate = (event: MouseEvent | KeyboardEvent) => local.onActivate?.(local.cardKey, event)
  return <li {...native} class={classes('cursor-pointer', local.class)} data-card-key={local.cardKey} draggable={native.draggable ?? Boolean(local.onMove)} tabIndex={native.tabIndex ?? 0} onClick={(event) => { activate(event); if (typeof local.onClick === 'function') local.onClick(event) }} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); activate(event) } if (typeof local.onKeyDown === 'function') local.onKeyDown(event) }} onDragStart={(event) => { event.dataTransfer?.setData('application/x-iota-kanban-card', JSON.stringify({ key: local.cardKey, column: local.columnKey, index: local.index })) }}>{local.children}</li>
}

export interface KanbanCardData { key: string; content: JSX.Element }
export interface KanbanColumnData { key: string; title: JSX.Element; cards: readonly KanbanCardData[] }
export interface KanbanProps extends Omit<KanbanBoardProps, 'children'> { columns: readonly KanbanColumnData[]; onColumnMove?: (move: KanbanMove) => void; onCardMove?: (move: KanbanCardMove) => void; onCardActivate?: KanbanCardProps['onActivate'] }

function draggedCard(event: DragEvent): { key: string; column: string; index: number } | undefined {
  const raw = event.dataTransfer?.getData('application/x-iota-kanban-card')
  if (!raw) return undefined
  try { return JSON.parse(raw) as { key: string; column: string; index: number } } catch { return undefined }
}

export function Kanban(props: KanbanProps) {
  const [local, boardProps] = splitProps(props, ['columns', 'onColumnMove', 'onCardMove', 'onCardActivate'])
  return <KanbanBoard {...boardProps}><For each={local.columns}>{(column, columnIndex) => <KanbanColumn columnKey={column.key} title={column.title} index={columnIndex()} columnCount={local.columns.length} onMove={local.onColumnMove} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); const source = draggedCard(event); if (source) local.onCardMove?.({ key: source.key, oldColumn: source.column, newColumn: column.key, oldIndex: source.index, newIndex: column.cards.length }) }}><For each={column.cards}>{(card, cardIndex) => <KanbanCard cardKey={card.key} columnKey={column.key} index={cardIndex()} onMove={local.onCardMove} onActivate={local.onCardActivate}>{card.content}</KanbanCard>}</For></KanbanColumn>}</For></KanbanBoard>
}
