import { createEffect, createMemo, createSignal, createUniqueId, For, onCleanup, onMount, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { CaretDownIcon, CheckIcon } from '../internal/icons'

export interface ComboboxOption {
  value: string
  label: string
  disabled?: boolean
  count?: number
  groupHeader?: boolean
}

export interface ComboboxLabels {
  notFound: string
  loading: string
  remove: (label: string) => string
  create: (query: string) => string
}

export interface ComboboxProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onChange'> {
  options?: readonly ComboboxOption[]
  value?: string | readonly string[]
  defaultValue?: string | readonly string[]
  onValueChange?: (value: string | string[]) => void
  onCreate?: (query: string) => void
  loadOptions?: (query: string, signal: AbortSignal) => Promise<readonly ComboboxOption[]>
  endpoint?: string
  multiple?: boolean
  searchable?: boolean
  canCreateNew?: boolean
  placeholder?: string
  label?: JSX.Element
  name?: string
  form?: string
  required?: boolean
  disabled?: boolean
  readOnly?: boolean
  listClass?: string
  labels?: Partial<ComboboxLabels>
}

const defaultLabels: ComboboxLabels = {
  notFound: 'No results found.',
  loading: 'Loading…',
  remove: (label) => `Remove ${label}`,
  create: (query) => `Create “${query}”`,
}

function normalizeValue(value: string | readonly string[] | undefined, multiple: boolean): string[] {
  if (typeof value === 'string') return value ? [value] : []
  if (value) return multiple ? [...value] : value.slice(0, 1)
  return []
}

export function Combobox(props: ComboboxProps) {
  const generatedID = createUniqueId()
  const [local, native] = splitProps(props, [
    'options', 'value', 'defaultValue', 'onValueChange', 'onCreate', 'loadOptions', 'endpoint', 'multiple', 'searchable',
    'canCreateNew', 'placeholder', 'label', 'name', 'form', 'required', 'disabled', 'readOnly', 'listClass', 'labels', 'class', 'ref',
  ])
  const [internalValue, setInternalValue] = createSignal(normalizeValue(local.defaultValue, Boolean(local.multiple)))
  const [query, setQuery] = createSignal('')
  const [open, setOpen] = createSignal(false)
  const [activeIndex, setActiveIndex] = createSignal(-1)
  const [remoteOptions, setRemoteOptions] = createSignal<readonly ComboboxOption[] | undefined>()
  const [loading, setLoading] = createSignal(false)
  let root!: HTMLDivElement
  let input!: HTMLInputElement
  let debounceTimer: ReturnType<typeof setTimeout> | undefined
  let request: AbortController | undefined
  const id = () => native.id ?? generatedID
  const listID = () => `${id()}-listbox`
  const labels = () => ({ ...defaultLabels, ...local.labels })
  const selected = createMemo(() => normalizeValue(local.value === undefined ? internalValue() : local.value, Boolean(local.multiple)))
  const sourceOptions = () => remoteOptions() ?? local.options ?? []
  const visibleOptions = createMemo(() => {
    if (local.loadOptions || local.endpoint) return sourceOptions()
    const needle = query().trim().toLocaleLowerCase()
    if (!needle) return sourceOptions()
    return sourceOptions().filter((item) => item.groupHeader || item.label.toLocaleLowerCase().includes(needle))
  })
  const selectedOptions = createMemo(() => selected().map((value) => sourceOptions().find((item) => item.value === value) ?? { value, label: value }))
  const inactive = () => Boolean(local.disabled || local.readOnly)

  const update = (values: string[]) => {
    if (local.value === undefined) setInternalValue(values)
    local.onValueChange?.(local.multiple ? values : (values[0] ?? ''))
  }
  const select = (option: ComboboxOption) => {
    if (option.disabled || option.groupHeader || inactive()) return
    if (local.multiple) {
      update(selected().includes(option.value) ? selected().filter((value) => value !== option.value) : [...selected(), option.value])
      setQuery('')
      input.focus()
    } else {
      update([option.value])
      setQuery('')
      setOpen(false)
      input.focus()
    }
  }
  const remove = (value: string) => update(selected().filter((item) => item !== value))
  const selectableIndices = () => visibleOptions().map((item, index) => !item.disabled && !item.groupHeader ? index : -1).filter((index) => index >= 0)
  const move = (delta: number) => {
    const indices = selectableIndices()
    if (!indices.length) return
    const position = indices.indexOf(activeIndex())
    setActiveIndex(indices[(position + delta + indices.length) % indices.length]!)
  }
  const fetchOptions = async (nextQuery: string) => {
    request?.abort()
    const controller = new AbortController()
    request = controller
    setLoading(true)
    try {
      const result = local.loadOptions
        ? await local.loadOptions(nextQuery, controller.signal)
        : await fetch(`${local.endpoint}${local.endpoint!.includes('?') ? '&' : '?'}q=${encodeURIComponent(nextQuery)}`, { signal: controller.signal }).then((response) => {
          if (!response.ok) throw new Error(`Combobox request failed: ${response.status}`)
          return response.json() as Promise<readonly ComboboxOption[]>
        })
      if (!controller.signal.aborted && request === controller) setRemoteOptions(result)
    } catch (error) {
      if (!controller.signal.aborted && request === controller && !(error instanceof DOMException && error.name === 'AbortError')) setRemoteOptions([])
    } finally {
      if (!controller.signal.aborted && request === controller) setLoading(false)
    }
  }
  const search = (value: string) => {
    setQuery(value)
    setOpen(true)
    if (local.loadOptions || local.endpoint) {
      clearTimeout(debounceTimer)
      debounceTimer = setTimeout(() => void fetchOptions(value), 250)
    }
  }
  const keydown: JSX.EventHandler<HTMLInputElement, KeyboardEvent> = (event) => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      setOpen(true)
      move(event.key === 'ArrowDown' ? 1 : -1)
    } else if (event.key === 'Enter' && open()) {
      event.preventDefault()
      const option = visibleOptions()[activeIndex()]
      if (option) select(option)
      else if (local.canCreateNew && query().trim()) local.onCreate?.(query().trim())
    } else if (event.key === 'Escape') {
      event.preventDefault()
      setOpen(false)
    }
  }

  createEffect(() => {
    visibleOptions()
    const first = selectableIndices()[0]
    setActiveIndex(first ?? -1)
  })
  onMount(() => {
    const outside = (event: PointerEvent) => {
      if (!root.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('pointerdown', outside)
    onCleanup(() => document.removeEventListener('pointerdown', outside))
  })
  onCleanup(() => {
    clearTimeout(debounceTimer)
    request?.abort()
  })

  return (
    <div {...native} ref={(element) => { root = element; if (typeof local.ref === 'function') local.ref(element) }} id={id()} class={classes('w-full flex flex-col', local.class)} data-required={local.required || undefined}>
      <Show when={local.label}><label class="form-control-label mb-2" for={`${id()}-input`}>{local.label}</label></Show>
      <select class="hidden" aria-hidden="true" tabindex="-1" multiple={local.multiple} name={local.name} form={local.form} disabled={inactive()} value={local.multiple ? selected() : selected()[0]}>
        <For each={local.options}>{(option) => <option value={option.value} selected={selected().includes(option.value)} disabled={option.disabled}>{option.label}</option>}</For>
      </select>
      <div class="relative h-full">
        <div class="flex items-center w-full relative form-control flex-wrap gap-1 py-1">
          <ul class="contents">
            <For each={selectedOptions()}>{(item) => (
              <li class="flex min-w-0 items-center gap-1.5 px-1.5 py-1 rounded-md bg-surface-100">
                <span class="block max-w-48 truncate" title={item.label}>{item.label}</span>
                <button class="text-brand-500 cursor-pointer flex-shrink-0" type="button" aria-label={labels().remove(item.label)} disabled={inactive()} onClick={() => remove(item.value)}>×</button>
              </li>
            )}</For>
          </ul>
          <input
            ref={input}
            id={`${id()}-input`}
            type="text"
            role="combobox"
            autocomplete="off"
            class={classes('form-control-input outline-none min-w-0', selected().length ? '!w-auto flex-1' : 'w-full')}
            placeholder={selected().length ? '' : local.placeholder}
            value={query()}
            disabled={local.disabled}
            readOnly={local.readOnly || (!local.searchable && !local.loadOptions && !local.endpoint)}
            aria-expanded={open()}
            aria-controls={listID()}
            aria-activedescendant={activeIndex() >= 0 ? `${id()}-option-${activeIndex()}` : undefined}
            aria-autocomplete={local.searchable || local.loadOptions || local.endpoint ? 'list' : 'none'}
            onFocus={() => setOpen(true)}
            onClick={() => !inactive() && setOpen(true)}
            onInput={(event) => search(event.currentTarget.value)}
            onKeyDown={keydown}
          />
          <button class={classes('inline-flex duration-200 cursor-pointer ml-auto', open() && 'rotate-180')} tabindex="-1" type="button" aria-label="Toggle options" disabled={inactive()} onClick={() => setOpen((value) => !value)}>
            <CaretDownIcon size={16} />
          </button>
        </div>
        <Show when={open() && !inactive()}>
          <ul id={listID()} role="listbox" aria-multiselectable={local.multiple || undefined} class={classes('combobox-dropdown bg-white z-10 m-0 flex flex-col gap-0.5 overflow-hidden overflow-y-auto border border-secondary p-1.5 rounded-md drop-shadow-sm', local.listClass)}>
            <Show when={loading()}><li class="px-4 py-2 text-sm text-200" role="status">{labels().loading}</li></Show>
            <For each={visibleOptions()}>{(item, index) => (
              <li
                id={`${id()}-option-${index()}`}
                role="option"
                aria-selected={selected().includes(item.value)}
                aria-disabled={item.disabled || item.groupHeader || undefined}
                tabindex={item.disabled || item.groupHeader ? -1 : 0}
                class={classes(
                  'combobox-option inline-flex cursor-pointer justify-between gap-6 px-4 py-2 text-sm rounded-md duration-100 hover:bg-surface-400 focus-visible:bg-surface-400 focus-visible:outline-none',
                  activeIndex() === index() && 'bg-surface-400',
                  item.groupHeader ? '!cursor-default !py-1 mt-1 text-xs uppercase tracking-wide text-200 pointer-events-none hover:bg-transparent' : item.disabled && 'opacity-40 !cursor-not-allowed hover:bg-transparent',
                )}
                onMouseEnter={() => !item.disabled && !item.groupHeader && setActiveIndex(index())}
                onClick={() => select(item)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); select(item) }
                  else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); move(event.key === 'ArrowDown' ? 1 : -1) }
                  else if (event.key === 'Escape') { setOpen(false); input.focus() }
                }}
              >
                <span class="inline-flex min-w-0 items-baseline gap-1.5"><span class="whitespace-nowrap">{item.label}</span><Show when={item.count !== undefined}><span class="text-xs text-200 whitespace-nowrap">({item.count!.toLocaleString()})</span></Show></span>
                <Show when={selected().includes(item.value)}><CheckIcon size={16} /></Show>
              </li>
            )}</For>
            <Show when={!loading() && visibleOptions().length === 0 && local.canCreateNew && query().trim()}>
              <li role="option" tabindex="0" class="cursor-pointer px-4 py-2 text-sm rounded-md duration-100 hover:bg-surface-400 focus-visible:bg-surface-400 focus-visible:outline-none" onClick={() => local.onCreate?.(query().trim())}>
                <input type="hidden" name="_SearchQuery" value={query()} />{labels().create(query().trim())}
              </li>
            </Show>
            <Show when={!loading() && visibleOptions().length === 0 && !local.canCreateNew}><li class="px-4 py-2 text-sm text-200">{labels().notFound}</li></Show>
          </ul>
        </Show>
      </div>
    </div>
  )
}
