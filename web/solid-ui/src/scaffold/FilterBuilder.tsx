import { createMemo, createSignal, createUniqueId, For, onCleanup, onMount, Show, splitProps, type JSX } from 'solid-js'
import { Button } from '../forms/Button'
import { classes } from '../internal/classes'
import { createFloatingPlacement, floatingPlacementClasses } from '../internal/floating'

export type FilterFieldType = 'reference' | 'date' | 'number' | 'bool' | 'text'
export interface FilterBuilderOption { value: string; label: string; count?: number; disabled?: boolean; group?: string }
/** A named date range offered as one click in a date field's editor ("This quarter"). */
export interface FilterBuilderPreset { id: string; label: string; values: readonly string[] }
export interface FilterBuilderField {
  key: string; label: string; type: FilterFieldType; group?: string; operators?: readonly string[]; options?: readonly FilterBuilderOption[]; presets?: readonly string[]; nullable?: boolean
  /** Date-range presets rendered as buttons above a date field's inputs. */
  presetOptions?: readonly FilterBuilderPreset[]
  /** A condition the view cannot do without (a report period): its chip has no remove button and "Clear all" keeps it. */
  required?: boolean
}
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
  /** False hides the remove button (a required condition). */
  removable?: boolean
}

export function FilterChip(props: FilterChipProps) {
  const [local, native] = splitProps(props, ['index', 'field', 'condition', 'encoded', 'summary', 'operatorLabel', 'editing', 'removeLabel', 'onEdit', 'onRemove', 'editor', 'class', 'removable'])
  // Every chip opens: a flag chip too, so "Overdue: no" can be switched.
  const editable = () => true
  let anchor!: HTMLDivElement
  let popover!: HTMLDivElement
  const floating = createFloatingPlacement(() => Boolean(local.editing), () => anchor, () => popover)
  return <div {...native} class={classes('relative', local.class)} data-fb-chip-wrapper>
    <input type="hidden" name="f" value={local.encoded} data-fb-chip={local.index} />
    <div ref={anchor} class="inline-flex h-8 items-stretch overflow-hidden rounded-lg border border-default bg-surface-100 text-sm">
      <button type="button" class={classes('inline-flex min-w-0 items-center gap-1.5 pl-3 pr-2 duration-150', editable() ? 'cursor-pointer hover:bg-surface-400' : 'cursor-default')} onClick={() => editable() && local.onEdit?.(local.index)}>
        <span class="font-medium whitespace-nowrap">{local.field.label}</span>
        {local.field.type === 'bool'
          ? <span class="text-200 whitespace-nowrap">{local.summary}</span>
          : <>{local.operatorLabel !== '' && <span class="text-200 whitespace-nowrap">{local.operatorLabel ?? local.condition.operator}</span>}<span class="max-w-56 truncate" title={local.summary}>{local.summary}</span></>}
      </button>
      {local.removable !== false && <button type="button" class="inline-flex items-center border-l border-default px-1.5 text-200 hover:text-100 hover:bg-surface-400 cursor-pointer duration-150" aria-label={local.removeLabel ?? 'Remove filter'} onClick={() => local.onRemove(local.index)}><Icon type="x" size={14} /></button>}
    </div>
    {editable() && local.editing && <div ref={popover} role="dialog" aria-label={`Edit ${local.field.label}`} data-fb-popover class={classes('absolute z-30 max-h-[calc(100vh-1rem)] w-80 max-w-[calc(100vw-1rem)] overflow-y-auto rounded-md border border-secondary bg-surface-300 p-3 drop-shadow-sm', ...floatingPlacementClasses(floating.placement(), 'mt-1'))}>{local.editor}</div>}
  </div>
}

export interface FilterEditorProps {
  field: FilterBuilderField; condition?: FilterCondition; applyLabel?: JSX.Element; onApply: (condition: FilterCondition) => void
  /** Localized operator names; the raw token is shown when absent. */
  operatorLabel?: (operator: string) => string
  /** Localized flag values for bool fields. */
  boolLabels?: { yes: string; no: string }
}

export function FilterEditor(props: FilterEditorProps) {
  const baseOperators = () => props.field.operators?.length ? props.field.operators : props.field.type === 'number' || props.field.type === 'date' ? ['is', 'before', 'after', 'between'] : ['is', 'is_not']
  // A nullable field also offers "is empty", which carries no values (pkg/filterq OpEmpty).
  const operators = () => props.field.nullable && props.field.type !== 'bool' && !baseOperators().includes('empty') ? [...baseOperators(), 'empty'] : baseOperators()
  const [operator, setOperator] = createSignal(props.condition?.operator ?? operators()[0] ?? 'is')
  const [values, setValues] = createSignal<string[]>([...(props.condition?.values ?? [])])
  const apply = () => props.onApply({ field: props.field.key, operator: operator(), values: operator() === 'empty' ? [] : values() })
  const applyPreset = (preset: FilterBuilderPreset) => props.onApply({ field: props.field.key, operator: 'between', values: [...preset.values], preset: preset.id })
  const applyFlag = (value: string) => props.onApply({ field: props.field.key, operator: 'is', values: [value] })
  if (props.field.type === 'bool') {
    const current = () => props.condition?.values[0] ?? ''
    return <div class="flex gap-1.5" role="group" aria-label={props.field.label}>
      <For each={['true', 'false']}>{(value) => <button type="button" class={classes('flex-1 rounded-md border border-default px-3 py-1.5 text-sm cursor-pointer hover:border-brand hover:bg-surface-400 duration-100', current() === value && 'bg-surface-400 font-medium border-brand')} aria-pressed={current() === value} onClick={() => applyFlag(value)}>{value === 'true' ? props.boolLabels?.yes ?? 'Yes' : props.boolLabels?.no ?? 'No'}</button>}</For>
    </div>
  }
  return <div class="flex flex-col gap-2.5" onKeyDown={(event) => { if (event.key === 'Enter' && !(event.target instanceof HTMLButtonElement)) { event.preventDefault(); apply() } }}>
    {props.field.type === 'date' && (props.field.presetOptions?.length ?? 0) > 0 && <div class="flex flex-wrap gap-1.5" role="group" aria-label={props.field.label}>
      <For each={props.field.presetOptions}>{(preset) => <button type="button" class={classes('rounded-md border border-default px-2 py-1 text-xs cursor-pointer hover:border-brand hover:bg-surface-400 duration-100', props.condition?.preset === preset.id && 'bg-surface-400 font-medium')} aria-pressed={props.condition?.preset === preset.id} onClick={() => applyPreset(preset)}>{preset.label}</button>}</For>
    </div>}
    {operators().length > 1 && <select class="min-w-20 w-full appearance-none form-control form-control-input h-[2.6875rem] pr-8" value={operator()} onChange={(event) => setOperator(event.currentTarget.value)}>{operators().map((item) => <option value={item}>{props.operatorLabel?.(item) ?? item}</option>)}</select>}
    {props.field.type === 'reference' && operator() !== 'empty' && <GroupedOptions class="form-control form-control-input min-h-24" options={props.field.options ?? []} selected={values()} onChange={(event) => setValues(Array.from(event.currentTarget.selectedOptions).map((option) => option.value).filter((value) => !value.startsWith('__group:')))} />}
    {(props.field.type === 'number' || props.field.type === 'date' || props.field.type === 'text') && operator() !== 'empty' && <div class={classes(operator() === 'between' && 'flex items-center gap-2')}>
      <input type={props.field.type === 'date' ? 'date' : props.field.type === 'number' ? 'number' : 'text'} step={props.field.type === 'number' ? 'any' : undefined} data-fb-value class="form-control-input input w-full min-w-0 rounded-md border border-default bg-transparent px-3 py-2 text-sm outline-none" value={values()[0] ?? ''} onInput={(event) => setValues([event.currentTarget.value, ...values().slice(1)])} />
      {operator() === 'between' && <><span class="text-200">–</span><input type={props.field.type === 'date' ? 'date' : 'number'} step={props.field.type === 'number' ? 'any' : undefined} data-fb-value class="form-control-input input w-full min-w-0 rounded-md border border-default bg-transparent px-3 py-2 text-sm outline-none" value={values()[1] ?? ''} onInput={(event) => setValues([values()[0] ?? '', event.currentTarget.value])} /></>}
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
  /** A chip's value text; the default joins the values (a preset shows its label). */
  formatSummary?: (field: FilterBuilderField, condition: FilterCondition) => string
  /** Localized operator names for chips and the editor; '' hides the operator on a chip. */
  operatorLabel?: (operator: string) => string
  applyLabel?: JSX.Element
  removeLabel?: string
  boolLabels?: { yes: string; no: string }
}

/** The default chip text: a preset's label, a range as "from – to", otherwise the values. */
export function defaultFilterSummary(field: FilterBuilderField, condition: FilterCondition, boolLabels?: { yes: string; no: string }): string {
  if (field.type === 'bool') return condition.values[0] === 'false' ? boolLabels?.no ?? 'No' : boolLabels?.yes ?? 'Yes'
  const preset = condition.preset ? field.presetOptions?.find((item) => item.id === condition.preset) : undefined
  if (preset) return preset.label
  if (condition.operator === 'between' && condition.values.length === 2) return `${condition.values[0]} – ${condition.values[1]}`
  const labels = condition.values.map((value) => field.options?.find((option) => option.value === value)?.label ?? value)
  return labels.join(', ')
}

const defaultCodec: FilterConditionCodec = { encode: (condition) => JSON.stringify(condition) }

export function FilterBuilder(props: FilterBuilderProps) {
  const [local, native] = splitProps(props, ['fields', 'conditions', 'defaultConditions', 'onConditionsChange', 'codec', 'addLabel', 'clearLabel', 'searchPlaceholder', 'showResetAlways', 'class', 'onKeyDown', 'formatSummary', 'operatorLabel', 'applyLabel', 'removeLabel', 'boolLabels'])
  const instance = createUniqueId()
  const [internal, setInternal] = createSignal<FilterCondition[]>([...(local.defaultConditions ?? [])])
  const [adding, setAdding] = createSignal(false)
  const [editing, setEditing] = createSignal<number>()
  const [draftField, setDraftField] = createSignal<FilterBuilderField>()
  const [search, setSearch] = createSignal('')
  const conditions = () => [...(local.conditions ?? internal())]
  const setConditions = (next: FilterCondition[]) => { if (local.conditions === undefined) setInternal(next); local.onConditionsChange?.(next) }
  const field = (key: string) => local.fields.find((item) => item.key === key)
  // A required field always has its chip; it is edited there, not added again.
  const visibleFields = createMemo(() => local.fields.filter((item) => item.label.toLowerCase().includes(search().toLowerCase()) && !(item.required && conditions().some((condition) => condition.field === item.key))))
  const isRequired = (condition: FilterCondition) => Boolean(field(condition.field)?.required)
  const removable = () => conditions().filter((condition) => !isRequired(condition))
  const summary = (item: FilterBuilderField, condition: FilterCondition) => local.formatSummary?.(item, condition) ?? defaultFilterSummary(item, condition, local.boolLabels)
  const editor = (item: FilterBuilderField, condition: FilterCondition | undefined, onApply: (next: FilterCondition) => void) => <FilterEditor field={item} condition={condition} onApply={onApply} operatorLabel={local.operatorLabel} applyLabel={local.applyLabel} boolLabels={local.boolLabels} />
  let root!: HTMLDivElement
  let addTrigger!: HTMLButtonElement
  let addPopover!: HTMLDivElement
  const addFloating = createFloatingPlacement(adding, () => addTrigger, () => addPopover)
  const close = (restoreFocus = false) => { const wasOpen = adding() || editing() !== undefined; setAdding(false); setEditing(undefined); setDraftField(undefined); if (restoreFocus && wasOpen) queueMicrotask(() => addTrigger?.isConnected && addTrigger.focus()) }
  const add = (condition: FilterCondition) => { setConditions([...conditions(), condition]); close() }
  const replace = (index: number, condition: FilterCondition) => { const next = conditions(); next[index] = condition; setConditions(next); close() }
  const outside = (event: PointerEvent) => { if ((adding() || editing() !== undefined) && !root.contains(event.target as Node)) close() }
  onMount(() => root.ownerDocument.addEventListener('pointerdown', outside))
  onCleanup(() => root?.ownerDocument.removeEventListener('pointerdown', outside))
  return <div {...native} ref={root} id={native.id ?? `filter-builder-${instance}`} class={classes('flex flex-wrap items-center gap-2', local.class)} onKeyDown={(event) => { if (event.key === 'Escape') { event.preventDefault(); close(true) } if (typeof local.onKeyDown === 'function') local.onKeyDown(event) }}>
    <input type="hidden" name="fb" value="1" />
    <For each={conditions()}>{(condition, index) => { const definition = () => field(condition.field); return <Show when={definition()}>{(item) => <FilterChip index={index()} field={item()} condition={condition} encoded={(local.codec ?? defaultCodec).encode(condition)} summary={summary(item(), condition)} operatorLabel={condition.preset || item().type === 'bool' ? '' : local.operatorLabel?.(condition.operator)} removable={!item().required} removeLabel={local.removeLabel} editing={editing() === index()} onEdit={(target) => setEditing(editing() === target ? undefined : target)} onRemove={(target) => setConditions(conditions().filter((_, current) => current !== target))} editor={editor(item(), condition, (next) => replace(index(), next))} />}</Show> }}</For>
    {/* Nothing left to add (every field is a required chip already): no trigger. */}
    <Show when={local.fields.some((item) => !(item.required && conditions().some((condition) => condition.field === item.key)))}>
    <div class="relative">
      <button ref={addTrigger} type="button" class="inline-flex items-center gap-1.5 h-8 px-3 rounded-lg text-sm cursor-pointer border border-dashed border-default text-200 hover:border-brand hover:text-100 hover:bg-surface-100 duration-150" aria-expanded={adding()} onClick={() => { setAdding(!adding()); setDraftField(undefined) }}><Icon type="plus" size={14} />{local.addLabel ?? 'Add filter'}</button>
      {adding() && <div ref={addPopover} role="dialog" aria-label="Add filter" data-fb-popover class={classes('absolute z-30 max-h-[calc(100vh-1rem)] w-80 max-w-[calc(100vw-1rem)] overflow-y-auto rounded-md border border-secondary bg-surface-300 drop-shadow-sm', ...floatingPlacementClasses(addFloating.placement(), 'mt-1'))}>
        {!draftField() ? <><div class="flex items-center gap-2 border-b border-secondary px-3 py-2"><Icon type="search" class="text-200 shrink-0" /><input type="text" class="w-full bg-transparent text-sm outline-none" placeholder={local.searchPlaceholder ?? 'Search fields'} value={search()} onInput={(event) => setSearch(event.currentTarget.value)} autocomplete="off" /></div><div class="max-h-72 overflow-y-auto p-1.5"><For each={visibleFields()}>{(item) => <button type="button" class="flex w-full items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-sm cursor-pointer duration-100 hover:bg-surface-400" data-fb-label={item.label} onClick={() => item.type === 'bool' ? add({ field: item.key, operator: 'is', values: ['true'] }) : setDraftField(item)}><Icon type={item.type} class="text-200 shrink-0" />{item.label}</button>}</For></div></>
          : <div class="p-3"><button type="button" class="mb-2 inline-flex items-center gap-1 text-xs text-200 hover:text-100 cursor-pointer" onClick={() => setDraftField(undefined)}><Icon type="back" size={12} />{draftField()!.label}</button>{editor(draftField()!, undefined, add)}</div>}
      </div>}
    </div>
    </Show>
    {(removable().length > 1 || (local.showResetAlways && removable().length > 0)) && <button type="button" class="ml-auto text-sm text-200 hover:text-100 cursor-pointer whitespace-nowrap" onClick={() => setConditions(conditions().filter(isRequired))}>{local.clearLabel ?? 'Clear all'}</button>}
  </div>
}
