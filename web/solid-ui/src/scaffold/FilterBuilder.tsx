import { createMemo, createSignal, createUniqueId, For, onCleanup, onMount, Show, splitProps, type JSX } from 'solid-js'
import { Button } from '../forms/Button'
import { classes } from '../internal/classes'

export type FilterFieldType = 'reference' | 'date' | 'number' | 'bool' | 'text'
export interface FilterBuilderOption { value: string; label: string; count?: number; disabled?: boolean; group?: string }
export interface FilterBuilderField { key: string; label: string; type: FilterFieldType; group?: string; operators?: readonly string[]; options?: readonly FilterBuilderOption[]; presets?: readonly string[] }
export interface FilterCondition { field: string; operator: string; values: readonly string[]; preset?: string }
export interface FilterConditionCodec { encode(condition: FilterCondition): string }

function Icon(props: { type: 'plus' | 'x' | 'search' | 'back' | FilterFieldType; size?: number; class?: string }) {
  const path = () => props.type === 'plus' ? <><line x1="40" y1="128" x2="216" y2="128" /><line x1="128" y1="40" x2="128" y2="216" /></> : props.type === 'x' ? <><line x1="64" y1="64" x2="192" y2="192" /><line x1="192" y1="64" x2="64" y2="192" /></> : props.type === 'search' ? <><circle cx="112" cy="112" r="72" /><line x1="163" y1="163" x2="216" y2="216" /></> : props.type === 'back' ? <polyline points="160 208 80 128 160 48" /> : props.type === 'date' ? <><rect x="40" y="48" width="176" height="168" rx="8" /><line x1="40" y1="88" x2="216" y2="88" /></> : props.type === 'number' ? <><line x1="88" y1="32" x2="72" y2="224" /><line x1="184" y1="32" x2="168" y2="224" /><line x1="40" y1="96" x2="216" y2="96" /><line x1="32" y1="160" x2="208" y2="160" /></> : <><line x1="88" y1="64" x2="216" y2="64" /><line x1="88" y1="128" x2="216" y2="128" /><line x1="88" y1="192" x2="216" y2="192" /></>
  return <svg aria-hidden="true" width={props.size ?? 16} height={props.size ?? 16} viewBox="0 0 256 256" class={props.class} fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16">{path()}</svg>
}

export interface GroupedOptionsProps extends JSX.SelectHTMLAttributes<HTMLSelectElement> { options: readonly FilterBuilderOption[]; selected?: readonly string[] }

export function GroupedOptions(props: GroupedOptionsProps) {
  const [local, native] = splitProps(props, ['options', 'selected', 'children'])
  return <select {...native} multiple={native.multiple ?? true}>{local.options.map((option, index) => {
    const heading = option.group && (index === 0 || option.group !== local.options[index - 1]?.group) ? option.group : undefined
    return <>{heading && <option disabled data-fb-header="1" value={`__group:${heading}`}>{heading}</option>}<option value={option.value} selected={local.selected?.includes(option.value)} disabled={option.disabled} data-fb-count={option.count !== undefined && option.count >= 0 ? option.count : undefined}>{option.label}</option></>
  })}{local.children}</select>
}

export interface FilterChipProps extends JSX.HTMLAttributes<HTMLDivElement> {
  index: number
  field: FilterBuilderField
  condition: FilterCondition
  encoded: string
  summary: string
  operatorLabel?: string
  editing?: boolean
  removeLabel?: string
  onEdit?: (index: number) => void
  onRemove: (index: number) => void
  editor?: JSX.Element
}

export function FilterChip(props: FilterChipProps) {
  const [local, native] = splitProps(props, ['index', 'field', 'condition', 'encoded', 'summary', 'operatorLabel', 'editing', 'removeLabel', 'onEdit', 'onRemove', 'editor', 'class'])
  const editable = () => local.field.type !== 'bool'
  return <div {...native} class={classes('relative', local.class)} data-fb-chip-wrapper>
    <input type="hidden" name="f" value={local.encoded} data-fb-chip={local.index} />
    <div class="inline-flex h-8 items-stretch overflow-hidden rounded-lg border border-default bg-surface-100 text-sm">
      <button type="button" class={classes('inline-flex min-w-0 items-center gap-1.5 pl-3 pr-2 duration-150', editable() ? 'cursor-pointer hover:bg-surface-400' : 'cursor-default')} onClick={() => editable() && local.onEdit?.(local.index)}>
        <span class="font-medium whitespace-nowrap">{local.field.label}</span>
        {editable() && <><span class="text-200 whitespace-nowrap">{local.operatorLabel ?? local.condition.operator}</span><span class="max-w-56 truncate" title={local.summary}>{local.summary}</span></>}
      </button>
      <button type="button" class="inline-flex items-center border-l border-default px-1.5 text-200 hover:text-100 hover:bg-surface-400 cursor-pointer duration-150" aria-label={local.removeLabel ?? 'Remove filter'} onClick={() => local.onRemove(local.index)}><Icon type="x" size={14} /></button>
    </div>
    {editable() && local.editing && <div role="dialog" aria-label={`Edit ${local.field.label}`} data-fb-popover class="absolute z-30 left-0 top-full mt-1 w-80 rounded-md border border-secondary bg-surface-300 drop-shadow-sm p-3">{local.editor}</div>}
  </div>
}

export interface FilterEditorProps { field: FilterBuilderField; condition?: FilterCondition; applyLabel?: JSX.Element; onApply: (condition: FilterCondition) => void }

export function FilterEditor(props: FilterEditorProps) {
  const operators = () => props.field.operators?.length ? props.field.operators : props.field.type === 'number' || props.field.type === 'date' ? ['is', 'before', 'after', 'between'] : ['is', 'is_not']
  const [operator, setOperator] = createSignal(props.condition?.operator ?? operators()[0] ?? 'is')
  const [values, setValues] = createSignal<string[]>([...(props.condition?.values ?? [])])
  const apply = () => props.onApply({ field: props.field.key, operator: operator(), values: values() })
  return <div class="flex flex-col gap-2.5" onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); apply() } }}>
    {operators().length > 1 && <select class="min-w-20 w-full appearance-none form-control form-control-input h-[2.6875rem] pr-8" value={operator()} onChange={(event) => setOperator(event.currentTarget.value)}>{operators().map((item) => <option value={item}>{item}</option>)}</select>}
    {props.field.type === 'reference' && <GroupedOptions class="form-control form-control-input min-h-24" options={props.field.options ?? []} selected={values()} onChange={(event) => setValues(Array.from(event.currentTarget.selectedOptions).map((option) => option.value).filter((value) => !value.startsWith('__group:')))} />}
    {(props.field.type === 'number' || props.field.type === 'date' || props.field.type === 'text') && <div class={classes(operator() === 'between' && 'flex items-center gap-2')}>
      <input type={props.field.type === 'date' ? 'date' : props.field.type === 'number' ? 'number' : 'text'} step={props.field.type === 'number' ? 'any' : undefined} data-fb-value class="form-control-input input w-full rounded-md border border-default bg-transparent px-3 py-2 text-sm outline-none" value={values()[0] ?? ''} onInput={(event) => setValues([event.currentTarget.value, ...values().slice(1)])} />
      {operator() === 'between' && <><span class="text-200">–</span><input type={props.field.type === 'date' ? 'date' : 'number'} step={props.field.type === 'number' ? 'any' : undefined} data-fb-value class="form-control-input input w-full rounded-md border border-default bg-transparent px-3 py-2 text-sm outline-none" value={values()[1] ?? ''} onInput={(event) => setValues([values()[0] ?? '', event.currentTarget.value])} /></>}
    </div>}
    <Button type="button" variant="primary" size="sm" class="justify-center" onClick={apply}>{props.applyLabel ?? 'Apply'}</Button>
  </div>
}

export interface FilterBuilderProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onChange'> {
  fields: readonly FilterBuilderField[]
  conditions?: readonly FilterCondition[]
  defaultConditions?: readonly FilterCondition[]
  onConditionsChange?: (conditions: FilterCondition[]) => void
  codec?: FilterConditionCodec
  addLabel?: JSX.Element
  clearLabel?: JSX.Element
  searchPlaceholder?: string
  showResetAlways?: boolean
}

const defaultCodec: FilterConditionCodec = { encode: (condition) => JSON.stringify(condition) }

export function FilterBuilder(props: FilterBuilderProps) {
  const [local, native] = splitProps(props, ['fields', 'conditions', 'defaultConditions', 'onConditionsChange', 'codec', 'addLabel', 'clearLabel', 'searchPlaceholder', 'showResetAlways', 'class', 'onKeyDown'])
  const instance = createUniqueId()
  const [internal, setInternal] = createSignal<FilterCondition[]>([...(local.defaultConditions ?? [])])
  const [adding, setAdding] = createSignal(false)
  const [editing, setEditing] = createSignal<number>()
  const [draftField, setDraftField] = createSignal<FilterBuilderField>()
  const [search, setSearch] = createSignal('')
  const conditions = () => [...(local.conditions ?? internal())]
  const setConditions = (next: FilterCondition[]) => { if (local.conditions === undefined) setInternal(next); local.onConditionsChange?.(next) }
  const field = (key: string) => local.fields.find((item) => item.key === key)
  const visibleFields = createMemo(() => local.fields.filter((item) => item.label.toLowerCase().includes(search().toLowerCase())))
  let root!: HTMLDivElement
  let addTrigger!: HTMLButtonElement
  const close = (restoreFocus = false) => { const wasOpen = adding() || editing() !== undefined; setAdding(false); setEditing(undefined); setDraftField(undefined); if (restoreFocus && wasOpen) queueMicrotask(() => addTrigger?.isConnected && addTrigger.focus()) }
  const add = (condition: FilterCondition) => { setConditions([...conditions(), condition]); close() }
  const replace = (index: number, condition: FilterCondition) => { const next = conditions(); next[index] = condition; setConditions(next); close() }
  const outside = (event: PointerEvent) => { if ((adding() || editing() !== undefined) && !root.contains(event.target as Node)) close() }
  onMount(() => root.ownerDocument.addEventListener('pointerdown', outside))
  onCleanup(() => root?.ownerDocument.removeEventListener('pointerdown', outside))
  return <div {...native} ref={root} id={native.id ?? `filter-builder-${instance}`} class={classes('flex flex-wrap items-center gap-2', local.class)} onKeyDown={(event) => { if (event.key === 'Escape') { event.preventDefault(); close(true) } if (typeof local.onKeyDown === 'function') local.onKeyDown(event) }}>
    <input type="hidden" name="fb" value="1" />
    <For each={conditions()}>{(condition, index) => { const definition = () => field(condition.field); return <Show when={definition()}>{(item) => <FilterChip index={index()} field={item()} condition={condition} encoded={(local.codec ?? defaultCodec).encode(condition)} summary={condition.values.join(', ')} editing={editing() === index()} onEdit={setEditing} onRemove={(target) => setConditions(conditions().filter((_, current) => current !== target))} editor={<FilterEditor field={item()} condition={condition} onApply={(next) => replace(index(), next)} />} />}</Show> }}</For>
    <div class="relative">
      <button ref={addTrigger} type="button" class="inline-flex items-center gap-1.5 h-8 px-3 rounded-lg text-sm cursor-pointer border border-dashed border-default text-200 hover:border-brand hover:text-100 hover:bg-surface-100 duration-150" aria-expanded={adding()} onClick={() => { setAdding(!adding()); setDraftField(undefined) }}><Icon type="plus" size={14} />{local.addLabel ?? 'Add filter'}</button>
      {adding() && <div role="dialog" aria-label="Add filter" data-fb-popover class="absolute z-30 left-0 top-full mt-1 w-80 rounded-md border border-secondary bg-surface-300 drop-shadow-sm">
        {!draftField() ? <><div class="flex items-center gap-2 border-b border-secondary px-3 py-2"><Icon type="search" class="text-200 shrink-0" /><input type="text" class="w-full bg-transparent text-sm outline-none" placeholder={local.searchPlaceholder ?? 'Search fields'} value={search()} onInput={(event) => setSearch(event.currentTarget.value)} autocomplete="off" /></div><div class="max-h-72 overflow-y-auto p-1.5"><For each={visibleFields()}>{(item) => <button type="button" class="flex w-full items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-sm cursor-pointer duration-100 hover:bg-surface-400" data-fb-label={item.label} onClick={() => item.type === 'bool' ? add({ field: item.key, operator: 'is', values: ['true'] }) : setDraftField(item)}><Icon type={item.type} class="text-200 shrink-0" />{item.label}</button>}</For></div></>
          : <div class="p-3"><button type="button" class="mb-2 inline-flex items-center gap-1 text-xs text-200 hover:text-100 cursor-pointer" onClick={() => setDraftField(undefined)}><Icon type="back" size={12} />{draftField()!.label}</button><FilterEditor field={draftField()!} onApply={add} /></div>}
      </div>}
    </div>
    {(conditions().length > 1 || (local.showResetAlways && conditions().length > 0)) && <button type="button" class="ml-auto text-sm text-200 hover:text-100 cursor-pointer whitespace-nowrap" onClick={() => setConditions([])}>{local.clearLabel ?? 'Clear all'}</button>}
  </div>
}
