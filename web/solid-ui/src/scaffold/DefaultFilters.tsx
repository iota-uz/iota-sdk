import { createEffect, createMemo, createSignal, For, splitProps, type JSX } from 'solid-js'
import { Input } from '../forms/Input'
import { Select } from '../forms/Select'
import { classes } from '../internal/classes'

export interface SearchField { label: string; key: string }
export interface FilterQueryState { search: string; field: string; limit: number; createdAtFrom: string; createdAtTo: string }

function SearchIcon(props: { list?: boolean }) {
  return <svg aria-hidden="true" width="20" height="20" viewBox="0 0 256 256" fill="none" stroke="currentColor" stroke-width="16" stroke-linecap="round"><circle cx="108" cy="108" r="68" /><line x1="158" y1="158" x2="216" y2="216" />{props.list && <><line x1="44" y1="52" x2="112" y2="52" /><line x1="44" y1="84" x2="92" y2="84" /></>}</svg>
}

export function SearchFieldsTrigger(props: JSX.ButtonHTMLAttributes<HTMLButtonElement>) {
  const [local, native] = splitProps(props, ['class', 'children'])
  return <button {...native} type="button" class={classes('flex items-center gap-2', local.class)}>{local.children ?? <SearchIcon list />}</button>
}

export interface SearchFieldsProps extends Omit<JSX.SelectHTMLAttributes<HTMLSelectElement>, 'onChange'> { fields: readonly SearchField[]; value?: string; defaultValue?: string; onValueChange?: (value: string) => void }
export function SearchFields(props: SearchFieldsProps) {
  const [local, native] = splitProps(props, ['fields', 'value', 'defaultValue', 'onValueChange', 'class'])
  const [internal, setInternal] = createSignal(local.defaultValue ?? local.fields[0]?.key ?? '')
  const value = () => local.value ?? internal()
  const change = (next: string) => { if (local.value === undefined) setInternal(next); local.onValueChange?.(next) }
  return <select {...native} name={native.name ?? 'Field'} value={value()} class={classes(local.fields.length === 1 ? 'hidden' : 'min-w-20 appearance-none bg-transparent outline-none cursor-pointer', local.class)} aria-label={native['aria-label'] ?? 'Search field'} onChange={(event) => change(event.currentTarget.value)}><For each={local.fields}>{(field) => <option value={field.key}>{field.label}</option>}</For></select>
}

export interface SearchProps extends Omit<JSX.InputHTMLAttributes<HTMLInputElement>, 'onChange' | 'type'> { fields: readonly SearchField[]; field?: string; defaultField?: string; onFieldChange?: (field: string) => void; onSearchChange?: (value: string) => void }
export function Search(props: SearchProps) {
  const [local, native] = splitProps(props, ['fields', 'field', 'defaultField', 'onFieldChange', 'onSearchChange', 'class', 'onInput'])
  return <Input {...native} name={native.name ?? 'Search'} placeholder={native.placeholder ?? 'Search'} class={local.class} addonLeft={<SearchIcon />} addonRight={<SearchFields fields={local.fields} value={local.field} defaultValue={local.defaultField} onValueChange={local.onFieldChange} />} onInput={(event) => { local.onSearchChange?.(event.currentTarget.value); if (typeof local.onInput === 'function') local.onInput(event) }} />
}

export interface PageSizeProps extends Omit<JSX.SelectHTMLAttributes<HTMLSelectElement>, 'onChange' | 'prefix'> { value?: number; defaultValue?: number; sizes?: readonly number[]; prefix?: JSX.Element; onValueChange?: (value: number) => void }
export function PageSize(props: PageSizeProps) {
  const [local, native] = splitProps(props, ['value', 'defaultValue', 'sizes', 'prefix', 'onValueChange'])
  const [internal, setInternal] = createSignal(local.defaultValue ?? 25)
  const value = () => local.value ?? internal()
  const change = (next: number) => { if (local.value === undefined) setInternal(next); local.onValueChange?.(next) }
  return <Select {...native} name={native.name ?? 'limit'} prefix={local.prefix ?? 'Per page'} value={String(value())} onChange={(event) => change(Number(event.currentTarget.value))}>{(local.sizes ?? [15, 25, 50, 100]).map((size) => <option value={size}>{size}</option>)}</Select>
}

export interface DateRangePreset { key: string; label: JSX.Element; resolve(now: Date): readonly [Date, Date] | undefined }
const isoDate = (date: Date) => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
const atDay = (now: Date, offset = 0) => new Date(now.getFullYear(), now.getMonth(), now.getDate() - offset)
export const defaultCreatedAtPresets: readonly DateRangePreset[] = [
  { key: '', label: 'All time', resolve: () => undefined },
  { key: 'today', label: 'Today', resolve: (now) => [atDay(now), atDay(now)] },
  { key: 'yesterday', label: 'Yesterday', resolve: (now) => [atDay(now, 1), atDay(now, 1)] },
  { key: 'this-week', label: 'This week', resolve: (now) => { const day = (now.getDay() + 6) % 7; return [atDay(now, day), atDay(now, day - 6)] } },
  { key: 'last-week', label: 'Last week', resolve: (now) => { const day = (now.getDay() + 6) % 7; return [atDay(now, day + 7), atDay(now, day + 1)] } },
  { key: 'this-month', label: 'This month', resolve: (now) => [new Date(now.getFullYear(), now.getMonth(), 1), new Date(now.getFullYear(), now.getMonth() + 1, 0)] },
]

export interface CreatedAtProps extends Omit<JSX.SelectHTMLAttributes<HTMLSelectElement>, 'onChange'> { from?: string; to?: string; now?: Date; presets?: readonly DateRangePreset[]; placeholder?: string; onValueChange?: (from: string, to: string, preset: string) => void }
export function CreatedAt(props: CreatedAtProps) {
  const [local, native] = splitProps(props, ['from', 'to', 'now', 'presets', 'placeholder', 'onValueChange', 'class'])
  const presets = () => local.presets ?? defaultCreatedAtPresets
  const initial = createMemo(() => presets().find((preset) => { const range = preset.resolve(local.now ?? new Date()); return (range ? isoDate(range[0]) : '') === (local.from ?? '') && (range ? isoDate(range[1]) : '') === (local.to ?? '') })?.key ?? '')
  const [selected, setSelected] = createSignal(initial())
  createEffect(() => setSelected(initial()))
  const select = (key: string) => { setSelected(key); const range = presets().find((preset) => preset.key === key)?.resolve(local.now ?? new Date()); local.onValueChange?.(range ? isoDate(range[0]) : '', range ? isoDate(range[1]) : '', key) }
  const range = createMemo(() => presets().find((preset) => preset.key === selected())?.resolve(local.now ?? new Date()))
  return <div class="contents"><Select {...native} class={classes('w-fit', local.class)} placeholder={local.placeholder ?? 'Created at'} value={selected()} onChange={(event) => select(event.currentTarget.value)}>{presets().map((preset) => <option value={preset.key}>{preset.label}</option>)}</Select><input type="hidden" name="CreatedAt.From" value={range() ? isoDate(range()![0]) : local.from ?? ''} /><input type="hidden" name="CreatedAt.To" value={range() ? isoDate(range()![1]) : local.to ?? ''} /></div>
}

export interface DefaultFiltersProps extends JSX.HTMLAttributes<HTMLDivElement> { fields: readonly SearchField[]; value?: Partial<FilterQueryState>; defaultValue?: Partial<FilterQueryState>; onQueryChange?: (state: FilterQueryState) => void; now?: Date }
export function Default(props: DefaultFiltersProps) {
  const [local, native] = splitProps(props, ['fields', 'value', 'defaultValue', 'onQueryChange', 'now', 'class'])
  const base: FilterQueryState = { search: '', field: local.fields[0]?.key ?? '', limit: 25, createdAtFrom: '', createdAtTo: '', ...local.defaultValue }
  const [internal, setInternal] = createSignal(base)
  const state = () => ({ ...base, ...internal(), ...local.value })
  const update = (patch: Partial<FilterQueryState>) => { const next = { ...state(), ...patch }; if (local.value === undefined) setInternal(next); local.onQueryChange?.(next) }
  return <div {...native} class={classes('contents', local.class)}><Search fields={local.fields} value={state().search} field={state().field} onSearchChange={(search) => update({ search })} onFieldChange={(field) => update({ field })} /><PageSize value={state().limit} onValueChange={(limit) => update({ limit })} /><CreatedAt from={state().createdAtFrom} to={state().createdAtTo} now={local.now} onValueChange={(createdAtFrom, createdAtTo) => update({ createdAtFrom, createdAtTo })} /></div>
}

export const DefaultFilters = Default
