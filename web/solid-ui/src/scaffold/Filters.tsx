import { createSignal, For, onCleanup, onMount, splitProps, type JSX } from 'solid-js'
import { Button } from '../forms/Button'
import { Checkbox } from '../forms/Checkbox'
import { Drawer } from '../overlays/Drawer'
import { classes } from '../internal/classes'

export interface FilterOption { value: string; label: JSX.Element; disabled?: boolean }

function XIcon(props: { size?: number }) { return <svg aria-hidden="true" width={props.size ?? 16} height={props.size ?? 16} viewBox="0 0 256 256"><line x1="64" y1="64" x2="192" y2="192" stroke="currentColor" stroke-linecap="round" stroke-width="16" /><line x1="192" y1="64" x2="64" y2="192" stroke="currentColor" stroke-linecap="round" stroke-width="16" /></svg> }
function CaretIcon() { return <svg aria-hidden="true" width="16" height="16" viewBox="0 0 256 256"><polyline points="208 96 128 176 48 96" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /></svg> }
function CloseCircleIcon() { return <svg aria-hidden="true" width="18" height="18" viewBox="0 0 256 256" fill="none" stroke="currentColor" stroke-width="16"><circle cx="128" cy="128" r="96" /><path d="m96 96 64 64m0-64-64 64" stroke-linecap="round" /></svg> }

export interface FilterDropdownProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onChange'> {
  label: JSX.Element
  name: string
  options: readonly FilterOption[]
  value?: readonly string[]
  defaultValue?: readonly string[]
  onValueChange?: (values: string[]) => void
  open?: boolean
  defaultOpen?: boolean
  onOpenChange?: (open: boolean) => void
}

export function FilterDropdown(props: FilterDropdownProps) {
  const [local, native] = splitProps(props, ['label', 'name', 'options', 'value', 'defaultValue', 'onValueChange', 'open', 'defaultOpen', 'onOpenChange', 'class', 'onKeyDown'])
  const [internalValue, setInternalValue] = createSignal<string[]>([...(local.defaultValue ?? [])])
  const [internalOpen, setInternalOpen] = createSignal(local.defaultOpen ?? false)
  const selected = () => [...(local.value ?? internalValue())]
  const expanded = () => local.open ?? internalOpen()
  let root!: HTMLDivElement
  let trigger!: HTMLButtonElement
  const setOpen = (next: boolean) => { if (local.open === undefined) setInternalOpen(next); local.onOpenChange?.(next) }
  const setValues = (next: string[]) => { if (local.value === undefined) setInternalValue(next); local.onValueChange?.(next) }
  const toggle = (value: string) => setValues(selected().includes(value) ? selected().filter((item) => item !== value) : [...selected(), value])
  const outside = (event: PointerEvent) => { if (expanded() && !root.contains(event.target as Node)) setOpen(false) }
  const onKeyDown: JSX.EventHandlerUnion<HTMLDivElement, KeyboardEvent> = (event) => {
    const items = Array.from(root.querySelectorAll<HTMLInputElement>('input[type="checkbox"]:not(:disabled)'))
    const current = items.indexOf(event.target as HTMLInputElement)
    if (event.key === 'Escape' && expanded()) { event.preventDefault(); setOpen(false); trigger.focus() }
    else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      if (!expanded()) { setOpen(true); queueMicrotask(() => { if (root.isConnected && expanded()) root.querySelector<HTMLInputElement>('input[type="checkbox"]:not(:disabled)')?.focus() }); return }
      if (items.length > 0) items[current < 0 ? 0 : (current + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length]?.focus()
    } else if ((event.key === 'Home' || event.key === 'End') && current >= 0) { event.preventDefault(); items[event.key === 'Home' ? 0 : items.length - 1]?.focus() }
    if (typeof local.onKeyDown === 'function') local.onKeyDown(event)
  }
  onMount(() => root.ownerDocument.addEventListener('pointerdown', outside))
  onCleanup(() => root?.ownerDocument.removeEventListener('pointerdown', outside))
  return (
    <div {...native} ref={root} class={classes('relative min-w-32', local.class)} onKeyDown={onKeyDown}>
      <div class="flex">
        {selected().length > 0 && <button type="button" class="flex items-center justify-center cursor-pointer border border-default rounded-md rounded-r-none px-2" aria-label="Clear filter" onClick={() => setValues([])}><XIcon /></button>}
        <button ref={trigger} type="button" aria-haspopup="true" aria-expanded={expanded()} class={classes('w-full border border-default rounded-md shadow-sm cursor-pointer flex items-center justify-between px-4 py-2', selected().length > 0 && 'rounded-l-none border-l-0')} onClick={() => setOpen(!expanded())}>
          <span class="text-gray-700 font-medium">{local.label}</span><span class={classes('text-gray-700 duration-200', expanded() && 'rotate-180')}><CaretIcon /></span>
        </button>
      </div>
      {expanded() && <ul aria-label={`${typeof local.label === 'string' ? local.label : 'Filter'} options`} class="absolute z-20 mt-2 bg-white max-h-80 overflow-y-auto min-w-fit border border-subtle rounded-md shadow-lg">
        <For each={local.options}>{(option) => <li class="hover:bg-gray-100"><Checkbox name={local.name} value={option.value} label={option.label} checked={selected().includes(option.value)} disabled={option.disabled} class="p-2" onChange={() => toggle(option.value)} /></li>}</For>
      </ul>}
    </div>
  )
}

export interface SideFilterProps extends Omit<FilterDropdownProps, 'label' | 'open' | 'defaultOpen' | 'onOpenChange'> { selectAllLabel?: JSX.Element }

export function SideFilter(props: SideFilterProps) {
  const [local, native] = splitProps(props, ['name', 'options', 'value', 'defaultValue', 'onValueChange', 'selectAllLabel', 'class'])
  const [internal, setInternal] = createSignal<string[]>([...(local.defaultValue ?? [])])
  const selected = () => [...(local.value ?? internal())]
  const setValues = (next: string[]) => { if (local.value === undefined) setInternal(next); local.onValueChange?.(next) }
  const all = () => local.options.filter((item) => !item.disabled).map((item) => item.value)
  return <div {...native} class={classes('bg-surface-600 border border-subtle rounded-lg p-4 mb-4', local.class)}><div class="space-y-3">
    <Checkbox label={local.selectAllLabel ?? 'Select all'} checked={all().length > 0 && all().every((value) => selected().includes(value))} indeterminate={selected().length > 0 && !all().every((value) => selected().includes(value))} onChange={() => setValues(all().every((value) => selected().includes(value)) ? [] : all())} />
    <hr class="my-3 border-t border-surface-400" />
    <div class="space-y-3"><For each={local.options}>{(option) => <Checkbox name={local.name} value={option.value} label={option.label} checked={selected().includes(option.value)} disabled={option.disabled} onChange={() => setValues(selected().includes(option.value) ? selected().filter((value) => value !== option.value) : [...selected(), option.value])} />}</For></div>
  </div></div>
}

export interface FiltersBarProps extends JSX.HTMLAttributes<HTMLDivElement> { stacked?: boolean; fullHeight?: boolean; search?: JSX.Element; actions?: JSX.Element }

export function FiltersBar(props: FiltersBarProps) {
  const [local, native] = splitProps(props, ['stacked', 'fullHeight', 'search', 'actions', 'class', 'children'])
  return <div {...native} class={classes('p-4 flex gap-3', local.stacked ? 'flex-col' : 'flex-col md:flex-row items-center', local.fullHeight && 'shrink-0', local.class)}>{local.search}{local.children}{local.actions && <div class="hidden md:flex gap-3 ml-auto h-full">{local.actions}</div>}</div>
}

export interface FiltersDrawerProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'title'> { heading: JSX.Element; open?: boolean; defaultOpen?: boolean; onOpenChange?: (open: boolean) => void; closeLabel?: string }

export function FiltersDrawer(props: FiltersDrawerProps) {
  const [local, native] = splitProps(props, ['heading', 'open', 'defaultOpen', 'onOpenChange', 'closeLabel', 'children', 'class'])
  const [internal, setInternal] = createSignal(local.defaultOpen ?? false)
  const expanded = () => local.open ?? internal()
  const change = (open: boolean) => { if (local.open === undefined) setInternal(open); local.onOpenChange?.(open) }
  return <Drawer open={expanded()} onOpenChange={change} direction="rtl" class="flex items-stretch"><div {...native} class={classes('flex flex-col gap-3 bg-white w-3/4 ml-auto p-4', local.class)}><div class="flex justify-between"><h3 class="text-2xl font-medium">{local.heading}</h3><div><Button variant="secondary" size="sm" fixed icon={<CloseCircleIcon />} aria-label={local.closeLabel ?? 'Close filters'} onClick={() => change(false)} /></div></div>{local.children}</div></Drawer>
}
