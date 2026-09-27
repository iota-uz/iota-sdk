import { createEffect, createSignal, createUniqueId, For, onCleanup, onMount, Show, splitProps, type JSX } from 'solid-js'
import { Body, Cell, Head, Header, Row, Table as BaseTable } from '../data/Table'
import { EmptyState } from '../data/EmptyState'
import { Loader } from '../data/Loader'
import { Button } from '../forms/Button'
import { Input } from '../forms/Input'
import { Switch } from '../forms-advanced/Switch'
import { Drawer } from '../overlays/Drawer'
import { classes } from '../internal/classes'

export type ScaffoldSortDirection = 'asc' | 'desc'
export interface ScaffoldTableQuery { search: string; page: number; limit: number; sort?: string; order?: ScaffoldSortDirection; [key: string]: string | number | undefined }
export interface ScaffoldTableColumn { key: string; label: JSX.Element; sortable?: boolean; priority?: number; sticky?: 'left' | 'right'; visible?: boolean; class?: string }
export interface ScaffoldTableRow { key: string; cells: Readonly<Record<string, JSX.Element>>; onSelect?: (row: ScaffoldTableRow, event: MouseEvent | KeyboardEvent) => void; class?: string }
export interface ScaffoldTableResult { rows: readonly ScaffoldTableRow[]; hasMore?: boolean; total?: number; footer?: Readonly<Record<string, JSX.Element>> }
export interface ScaffoldTableLoadRequest { query: Readonly<ScaffoldTableQuery>; append: boolean; signal: AbortSignal }
export interface ScaffoldTableAdapter { load(request: ScaffoldTableLoadRequest): Promise<ScaffoldTableResult> }

export function DateTime(props: { value: Date | string; locale?: string; options?: Intl.DateTimeFormatOptions; class?: string }) {
  const date = () => typeof props.value === 'string' ? new Date(props.value) : props.value
  const formatted = () => Number.isNaN(date().getTime()) ? String(props.value) : new Intl.DateTimeFormat(props.locale, props.options ?? { dateStyle: 'medium', timeStyle: 'short' }).format(date())
  return <div class={props.class}><time dateTime={Number.isNaN(date().getTime()) ? undefined : date().toISOString()}>{formatted()}</time></div>
}

export interface SearchClearButtonProps extends JSX.ButtonHTMLAttributes<HTMLButtonElement> { visible?: boolean; onClear?: () => void }
export function SearchClearButton(props: SearchClearButtonProps) {
  const [local, native] = splitProps(props, ['visible', 'onClear', 'class', 'children', 'onClick'])
  return <button {...native} type="button" hidden={local.visible === false} aria-label={native['aria-label'] ?? 'Clear search'} class={classes('w-4.5 h-4.5 flex items-center justify-center rounded-full text-gray-400 hover:text-white hover:bg-gray-400 transition-all duration-150 cursor-pointer', local.class)} onClick={(event) => { local.onClear?.(); if (typeof local.onClick === 'function') local.onClick(event) }}>{local.children ?? <span aria-hidden="true">×</span>}</button>
}

export interface RowsProps { columns: readonly ScaffoldTableColumn[]; rows: readonly ScaffoldTableRow[]; emptyTitle?: JSX.Element; emptyDescription?: JSX.Element }
export function Rows(props: RowsProps) {
  return <Body><Show when={props.rows.length > 0} fallback={<Row class="h-full"><Cell colSpan={props.columns.length} class="!p-0 !border-0 h-full"><EmptyState title={props.emptyTitle ?? 'No data'} description={props.emptyDescription} class="h-full" /></Cell></Row>}><For each={props.rows}>{(row) => <Row class={row.class} tabIndex={row.onSelect ? 0 : undefined} onClick={(event) => row.onSelect?.(row, event)} onKeyDown={(event) => { if (row.onSelect && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); row.onSelect(row, event) } }}><For each={props.columns.filter((column) => column.visible !== false)}>{(column) => <Cell priority={column.priority} class={classes(column.sticky && 'sticky bg-surface-600', column.sticky === 'right' && 'right-0 border-l border-default', column.sticky === 'left' && 'left-0 border-r border-default', column.class)} data-col={column.key}>{row.cells[column.key]}</Cell>}</For></Row>}</For></Show></Body>
}

export interface InfiniteScrollSpinnerProps extends JSX.HTMLAttributes<HTMLTableRowElement> { columnCount: number; hasMore?: boolean; loading?: boolean; onLoadMore?: () => void }
export function InfiniteScrollSpinner(props: InfiniteScrollSpinnerProps) {
  const [local, native] = splitProps(props, ['columnCount', 'hasMore', 'loading', 'onLoadMore', 'class'])
  let row!: HTMLTableRowElement
  let requested = false
  onMount(() => {
    if (typeof IntersectionObserver === 'undefined') return
    const observer = new IntersectionObserver((entries) => { if (entries.some((entry) => entry.isIntersecting) && local.hasMore && !local.loading && !requested) { requested = true; local.onLoadMore?.(); queueMicrotask(() => { if (!local.loading) requested = false }) } })
    observer.observe(row)
    onCleanup(() => observer.disconnect())
  })
  return <tr {...native} ref={row} class={classes(!local.hasMore && 'hidden', local.class)}><td colSpan={local.columnCount} class="!p-0"><div class="sticky left-0"><button type="button" disabled={local.loading} class="flex w-full justify-center items-center py-4" onClick={() => local.onLoadMore?.()}>{local.loading ? <Loader label="Loading more" /> : <span class="sr-only">Load more</span>}</button></div></td></tr>
}

export interface FillerRowsWrapperProps extends JSX.HTMLAttributes<HTMLDivElement> { stickyHeader?: boolean; noWrap?: boolean; rowHeight?: number }
export function FillerRowsWrapper(props: FillerRowsWrapperProps) { const [local, native] = splitProps(props, ['stickyHeader', 'noWrap', 'rowHeight', 'class', 'children']); return <div {...native} class={classes('table-filler-rows', local.stickyHeader && 'table-filler-container', local.noWrap && 'table-nowrap', local.class)} style={{ '--filler-row-height': `${local.rowHeight ?? 48}px` }}>{local.children}</div> }

export interface TableProps extends JSX.HTMLAttributes<HTMLDivElement> { columns: readonly ScaffoldTableColumn[]; result: ScaffoldTableResult; query?: ScaffoldTableQuery; loading?: boolean; fillerRows?: boolean; fillerCount?: number; stickyHeader?: boolean; noWrap?: boolean; onSort?: (key: string, direction: ScaffoldSortDirection) => void; onLoadMore?: () => void; emptyTitle?: JSX.Element; emptyDescription?: JSX.Element }
export function Table(props: TableProps) {
  const [local, native] = splitProps(props, ['columns', 'result', 'query', 'loading', 'fillerRows', 'fillerCount', 'stickyHeader', 'noWrap', 'onSort', 'onLoadMore', 'emptyTitle', 'emptyDescription', 'class'])
  const sort = local.onSort
  const query = local.query
  const columns = () => local.columns.filter((column) => column.visible !== false)
  const content = <div {...native} class={classes('relative', local.class)}><BaseTable><Header class={classes(local.stickyHeader && 'sticky top-0 z-10 shadow-lg')}><Row><For each={columns()}>{(column) => <Head priority={column.priority} sortable={column.sortable} {...{ sortDirection: query?.sort === column.key ? query.order : undefined }} onSort={(direction) => { if (direction !== 'none') sort?.(column.key, direction) }} class={classes(column.sticky && 'sticky bg-surface-500', column.sticky === 'right' && 'right-0', column.sticky === 'left' && 'left-0', column.class)} data-col={column.key}>{column.label}</Head>}</For></Row></Header><Rows columns={columns()} rows={local.result.rows} emptyTitle={local.emptyTitle} emptyDescription={local.emptyDescription} />{local.result.hasMore && <Body><InfiniteScrollSpinner columnCount={columns().length} hasMore loading={local.loading} onLoadMore={local.onLoadMore} /></Body>}{local.result.footer && <tfoot><Row class="bg-surface-300 text-100"><For each={columns()}>{(column) => <Cell class="bg-surface-300 font-bold border-t-2 border-strong">{local.result.footer?.[column.key]}</Cell>}</For></Row></tfoot>}</BaseTable>{local.loading && <div class="table-loading-overlay" aria-hidden="true"><div class="table-loading-overlay__card"><Loader /></div></div>}</div>
  return local.fillerRows ? <FillerRowsWrapper stickyHeader={local.stickyHeader} noWrap={local.noWrap}>{content}{Array.from({ length: local.fillerCount ?? 0 }, () => <div class="grid-filler" />)}</FillerRowsWrapper> : content
}

export interface TableContentProps extends TableProps { search?: string; searchPlaceholder?: string; withoutSearch?: boolean; searchClearable?: boolean; filters?: JSX.Element; actions?: JSX.Element; configurable?: boolean; stackedToolbar?: boolean; fullHeight?: boolean; gridVertical?: boolean; gridHorizontal?: boolean; onSearchChange?: (value: string) => void; onOpenSettings?: () => void; onColumnsChange?: (columns: ScaffoldTableColumn[]) => void; onGridChange?: (grid: { vertical: boolean; horizontal: boolean }) => void }
export function TableContent(props: TableContentProps) {
  const [local, tableProps] = splitProps(props, ['search', 'searchPlaceholder', 'withoutSearch', 'searchClearable', 'filters', 'actions', 'configurable', 'stackedToolbar', 'fullHeight', 'gridVertical', 'gridHorizontal', 'onSearchChange', 'onOpenSettings', 'onColumnsChange', 'onGridChange'])
  const [settingsOpen, setSettingsOpen] = createSignal(false)
  const [columns, setColumns] = createSignal([...tableProps.columns])
  const instance = createUniqueId()
  createEffect(() => setColumns([...tableProps.columns]))
  const toggleColumn = (key: string) => { const next = columns().map((column) => column.key === key ? { ...column, visible: column.visible === false } : column); setColumns(next); local.onColumnsChange?.(next) }
  const setGrid = (patch: Partial<{ vertical: boolean; horizontal: boolean }>) => local.onGridChange?.({ vertical: local.gridVertical ?? true, horizontal: local.gridHorizontal ?? true, ...patch })
  return <div class={classes('bg-surface-600 border border-subtle rounded-lg', local.fullHeight && 'flex flex-col h-full min-h-0 overflow-hidden')}><Show when={local.configurable || !local.withoutSearch || local.filters || local.actions}><div class={classes('p-4 flex gap-3', local.stackedToolbar ? 'flex-col' : 'flex-col md:flex-row items-center', local.fullHeight && 'shrink-0')}><Show when={!local.withoutSearch}><div class="flex-1"><Input name="Search" value={local.search ?? ''} placeholder={local.searchPlaceholder ?? 'Search'} addonLeft={<span aria-hidden="true">⌕</span>} addonRight={local.searchClearable ? <SearchClearButton visible={Boolean(local.search)} onClear={() => local.onSearchChange?.('')} /> : undefined} onInput={(event) => local.onSearchChange?.(event.currentTarget.value)} /></div></Show>{local.filters && <div class={classes('hidden md:flex gap-3 h-full', local.stackedToolbar && 'md:block')}>{local.filters}</div>}{local.actions && <div class="hidden md:flex gap-3 ml-auto h-full">{local.actions}</div>}{local.configurable && <TableSettingsTrigger onClick={() => { setSettingsOpen(true); local.onOpenSettings?.() }} />}</div></Show><div class={local.fullHeight ? 'flex-1 min-h-0' : 'overflow-x-auto'}><Table {...tableProps} columns={columns()} /></div><Show when={local.configurable}><Drawer open={settingsOpen()} onOpenChange={setSettingsOpen} direction="rtl" class="max-w-96 ml-auto flex items-stretch"><div class="p-4 w-full"><div class="flex flex-col h-full bg-surface-300 rounded-lg"><div class="flex justify-between px-4 py-3 border-b border-subtle"><h3 class="font-medium">Table settings</h3><button type="button" class="cursor-pointer" aria-label="Close table settings" onClick={() => setSettingsOpen(false)}>×</button></div><div class="flex-1 min-h-0 overflow-y-auto"><div class="p-4 flex flex-col gap-4"><div><h4 class="text-sm font-medium text-100 mb-2">Columns</h4><ul class="flex flex-col gap-2"><For each={columns().filter((column) => !column.sticky)}>{(column) => <li class="flex items-center justify-between gap-2"><label for={`${instance}-col-${column.key}`} class="text-sm font-medium">{column.label}</label><Switch id={`${instance}-col-${column.key}`} size="sm" checked={column.visible !== false} onChange={() => toggleColumn(column.key)} /></li>}</For></ul></div><div><h4 class="text-sm font-medium text-100 mb-2">Grid</h4><ul class="flex flex-col gap-2"><li class="flex items-center justify-between gap-2"><label for={`${instance}-grid-vertical`} class="text-sm font-medium">Vertical lines</label><Switch id={`${instance}-grid-vertical`} size="sm" checked={local.gridVertical ?? true} onChange={(event) => setGrid({ vertical: event.currentTarget.checked })} /></li><li class="flex items-center justify-between gap-2"><label for={`${instance}-grid-horizontal`} class="text-sm font-medium">Horizontal lines</label><Switch id={`${instance}-grid-horizontal`} size="sm" checked={local.gridHorizontal ?? true} onChange={(event) => setGrid({ horizontal: event.currentTarget.checked })} /></li></ul></div></div></div></div></div></Drawer></Show></div>
}

export interface TableSectionProps extends TableContentProps { query: ScaffoldTableQuery; sideFilter?: JSX.Element; deferredPanels?: readonly DeferredPanelSpec[]; contentId?: string; onQueryChange?: (query: ScaffoldTableQuery) => void }
export function TableSection(props: TableSectionProps) {
  const [local, contentProps] = splitProps(props, ['query', 'sideFilter', 'deferredPanels', 'contentId', 'onQueryChange', 'fullHeight'])
  const update = (patch: Partial<ScaffoldTableQuery>) => local.onQueryChange?.({ ...local.query, ...patch, page: patch.page ?? 1 })
  return <form class={local.fullHeight ? 'flex-1 min-h-0' : undefined} onSubmit={(event) => event.preventDefault()}><Show when={local.deferredPanels?.length}><DeferredPanels panels={local.deferredPanels!} query={local.query} /></Show><div class={classes('flex gap-5', local.fullHeight && 'h-full')}>{local.sideFilter && <div class="hidden md:block w-64 flex-shrink-0">{local.sideFilter}</div>}<div id={local.contentId} class={classes('flex-1 max-w-full', local.fullHeight && 'h-full min-h-0')}><TableContent {...contentProps} fullHeight={local.fullHeight} query={local.query} search={local.query.search} onSearchChange={(search) => update({ search })} onSort={(sort, order) => update({ sort, order })} onLoadMore={() => update({ page: local.query.page + 1 })} /></div></div></form>
}

export interface DeferredPanelSpec { id: string; class?: string; load: (request: { query: Readonly<ScaffoldTableQuery>; signal: AbortSignal }) => Promise<JSX.Element>; skeleton?: JSX.Element }
export function DeferredPanels(props: { panels: readonly DeferredPanelSpec[]; query: ScaffoldTableQuery; class?: string }) { return <div class={classes('flex flex-col gap-3 mb-3', props.class)}><For each={props.panels}>{(panel) => <DeferredPanel panel={panel} query={props.query} />}</For></div> }
function DeferredPanel(props: { panel: DeferredPanelSpec; query: ScaffoldTableQuery }) { const [content, setContent] = createSignal<JSX.Element>(); const [error, setError] = createSignal<unknown>(); createEffect(() => { const query = { ...props.query }; const controller = new AbortController(); setContent(undefined); setError(undefined); void props.panel.load({ query, signal: controller.signal }).then((value) => { if (!controller.signal.aborted) setContent(() => value) }, (cause) => { if (!controller.signal.aborted) setError(cause) }); onCleanup(() => controller.abort()) }); return <div id={props.panel.id} class={props.panel.class} aria-busy={content() === undefined && error() === undefined}>{error() ? <div role="alert">Failed to load panel</div> : content() ?? props.panel.skeleton ?? <DefaultPanelSkeleton />}</div> }
export function DefaultPanelSkeleton() { return <div class="flex flex-wrap gap-3" aria-busy="true" aria-live="polite">{Array.from({ length: 3 }, () => <div class="animate-pulse h-9 w-32 rounded-lg bg-surface-500" />)}</div> }

export function TableSettingsTrigger(props: JSX.ButtonHTMLAttributes<HTMLButtonElement>) { const [local, native] = splitProps(props, ['class', 'children']); return <Button {...native} type="button" variant="secondary" class={classes('ml-auto', local.class)} aria-label={native['aria-label'] ?? 'Table settings'} title={native.title ?? 'Table settings'}>{local.children ?? <><span aria-hidden="true">⚙</span><span class="sr-only">Table settings</span></>}</Button> }

export interface EmbeddedContentProps extends TableSectionProps { drawer?: JSX.Element }
export function EmbeddedContent(props: EmbeddedContentProps) { const [local, sectionProps] = splitProps(props, ['drawer']); return <><TableSection {...sectionProps} /><div>{local.drawer}</div></> }
export interface ContentProps extends Omit<EmbeddedContentProps, 'title'> { title: JSX.Element; mobileFiltersLabel?: JSX.Element }
export function Content(props: ContentProps) { const [local, embedded] = splitProps(props, ['title', 'mobileFiltersLabel', 'actions']); return <div class="m-6"><div class="flex flex-wrap gap-y-3 justify-between"><h1 class="text-2xl font-medium min-w-0">{local.title}</h1><div class="flex md:hidden flex-wrap justify-end gap-2"><Button variant="secondary">{local.mobileFiltersLabel ?? 'Filters'}</Button>{local.actions}</div></div><div class="mt-5"><EmbeddedContent {...embedded} actions={local.actions} /></div></div> }
export interface PageProps extends ContentProps { pageTitle?: string }
export function Page(props: PageProps) { const [local, content] = splitProps(props, ['pageTitle']); return <main aria-label={local.pageTitle ?? (typeof content.title === 'string' ? content.title : 'Table page')}><Content {...content} /></main> }

export interface ContentHTMXProps extends Omit<TableSectionProps, 'query' | 'result' | 'loading' | 'onQueryChange'> { adapter: ScaffoldTableAdapter; query?: ScaffoldTableQuery; defaultQuery?: Partial<ScaffoldTableQuery>; onQueryChange?: (query: ScaffoldTableQuery) => void; autoLoad?: boolean; errorFallback?: (error: unknown, retry: () => void) => JSX.Element }
export function ContentHTMX(props: ContentHTMXProps) {
  const [local, sectionProps] = splitProps(props, ['adapter', 'query', 'defaultQuery', 'onQueryChange', 'autoLoad', 'errorFallback'])
  const initial: ScaffoldTableQuery = { search: '', page: 1, limit: 25, ...local.defaultQuery }
  const [internalQuery, setInternalQuery] = createSignal(initial)
  const [result, setResult] = createSignal<ScaffoldTableResult>({ rows: [] })
  const [loading, setLoading] = createSignal(false)
  const [error, setError] = createSignal<unknown>()
  let controller: AbortController | undefined
  let revision = 0
  let initialized = false
  const query = () => local.query ?? internalQuery()
  const load = async (next: ScaffoldTableQuery) => {
    controller?.abort(); controller = new AbortController(); const current = ++revision; setLoading(true); setError(undefined)
    try { const loaded = await local.adapter.load({ query: next, append: next.page > 1, signal: controller.signal }); if (current === revision && !controller.signal.aborted) setResult((previous) => next.page > 1 ? { ...loaded, rows: [...previous.rows, ...loaded.rows] } : loaded) }
    catch (cause) { if (current === revision && !controller.signal.aborted) setError(cause) }
    finally { if (current === revision) setLoading(false) }
  }
  const change = (next: ScaffoldTableQuery) => { if (local.query === undefined) setInternalQuery(next); local.onQueryChange?.(next) }
  createEffect(() => {
    const next = { ...query() }
    JSON.stringify(next)
    if (!initialized) { initialized = true; if (!(local.autoLoad ?? true)) return }
    void load(next)
  })
  onCleanup(() => controller?.abort())
  return <Show when={!error()} fallback={local.errorFallback?.(error(), () => void load(query())) ?? <div role="alert"><button type="button" onClick={() => void load(query())}>Retry table</button></div>}><TableSection {...sectionProps} query={query()} result={result()} loading={loading()} onQueryChange={change} /></Show>
}
