import { createEffect, createSignal, createUniqueId, For, onCleanup, onMount, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { callHandler } from '../internal/events'

export interface SearchSelectOption {
  value: string
  label: string
  disabled?: boolean
}

export interface SearchSelectLabels {
  nothingFound: string
  loading: string
  loadError: string
}

export interface SearchSelectProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onChange'> {
  value?: string
  defaultValue?: string
  selectedOption?: SearchSelectOption
  defaultSelectedOption?: SearchSelectOption
  onValueChange?: (value: string, option: SearchSelectOption) => void
  loadOptions?: (query: string, signal: AbortSignal) => Promise<readonly SearchSelectOption[]>
  endpoint?: string
  minQueryLength?: number
  debounceMs?: number
  label?: JSX.Element
  placeholder?: string
  name?: string
  form?: string
  disabled?: boolean
  readOnly?: boolean
  required?: boolean
  inputProps?: Omit<JSX.InputHTMLAttributes<HTMLInputElement>, 'value' | 'name' | 'form'>
  labels?: Partial<SearchSelectLabels>
}

const defaultLabels: SearchSelectLabels = {
  nothingFound: 'No results found.',
  loading: 'Loading…',
  loadError: 'Error loading results.',
}

export function SearchSelect(props: SearchSelectProps) {
  const generatedID = createUniqueId()
  const [local, native] = splitProps(props, [
    'value', 'defaultValue', 'selectedOption', 'defaultSelectedOption', 'onValueChange', 'loadOptions', 'endpoint', 'minQueryLength',
    'debounceMs', 'label', 'placeholder', 'name', 'form', 'disabled', 'readOnly', 'required', 'inputProps', 'labels', 'class', 'id', 'ref',
  ])
  const initialOption = local.selectedOption ?? local.defaultSelectedOption
  const [internalValue, setInternalValue] = createSignal(local.defaultValue ?? initialOption?.value ?? '')
  const [query, setQuery] = createSignal(initialOption?.label ?? '')
  const [options, setOptions] = createSignal<readonly SearchSelectOption[]>([])
  const [open, setOpen] = createSignal(false)
  const [loading, setLoading] = createSignal(false)
  const [failed, setFailed] = createSignal(false)
  const [activeIndex, setActiveIndex] = createSignal(-1)
  let root!: HTMLDivElement
  let input!: HTMLInputElement
  let timer: ReturnType<typeof setTimeout> | undefined
  let request: AbortController | undefined
  const id = () => local.id ?? `search-select-${generatedID}`
  const listID = () => `${id()}-results`
  const value = () => local.value ?? internalValue()
  const labels = () => ({ ...defaultLabels, ...local.labels })
  const inactive = () => Boolean(local.disabled || local.readOnly)

  const fetchOptions = async (nextQuery: string) => {
    request?.abort()
    const controller = new AbortController()
    request = controller
    setLoading(true)
    setFailed(false)
    try {
      const result = local.loadOptions
        ? await local.loadOptions(nextQuery, controller.signal)
        : await fetch(`${local.endpoint}${local.endpoint!.includes('?') ? '&' : '?'}q=${encodeURIComponent(nextQuery)}`, { signal: controller.signal }).then((response) => {
          if (!response.ok) throw new Error(`SearchSelect request failed: ${response.status}`)
          return response.json() as Promise<readonly SearchSelectOption[]>
        })
      if (!controller.signal.aborted && request === controller) {
        setOptions(result)
        setActiveIndex(result.findIndex((item) => !item.disabled))
      }
    } catch (error) {
      if (!controller.signal.aborted && request === controller && !(error instanceof DOMException && error.name === 'AbortError')) {
        setOptions([])
        setFailed(true)
      }
    } finally {
      if (!controller.signal.aborted && request === controller) setLoading(false)
    }
  }
  const search = (nextQuery: string) => {
    setQuery(nextQuery)
    setOpen(true)
    clearTimeout(timer)
    if (nextQuery.length < (local.minQueryLength ?? 2)) {
      request?.abort()
      setOptions([])
      setLoading(false)
      return
    }
    timer = setTimeout(() => void fetchOptions(nextQuery), local.debounceMs ?? 300)
  }
  const select = (option: SearchSelectOption) => {
    if (option.disabled || inactive()) return
    if (local.value === undefined) setInternalValue(option.value)
    setQuery(option.label)
    setOpen(false)
    local.onValueChange?.(option.value, option)
    input.focus()
  }
  const move = (delta: number) => {
    const enabled = options().map((item, index) => item.disabled ? -1 : index).filter((index) => index >= 0)
    if (!enabled.length) return
    const position = enabled.indexOf(activeIndex())
    setActiveIndex(enabled[(position + delta + enabled.length) % enabled.length]!)
  }
  const keydown: JSX.EventHandler<HTMLInputElement, KeyboardEvent> = (event) => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      setOpen(true)
      move(event.key === 'ArrowDown' ? 1 : -1)
    } else if (event.key === 'Enter' && open()) {
      event.preventDefault()
      const option = options()[activeIndex()]
      if (option) select(option)
    } else if (event.key === 'Escape') {
      event.preventDefault()
      setOpen(false)
    }
    callHandler(local.inputProps?.onKeyDown, event)
  }

  createEffect(() => {
    const selected = local.selectedOption
    if (selected) setQuery(selected.label)
    else if (local.value === '') setQuery('')
  })

  onMount(() => {
    const outside = (event: PointerEvent) => {
      if (!root.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('pointerdown', outside)
    onCleanup(() => document.removeEventListener('pointerdown', outside))
  })
  onCleanup(() => {
    clearTimeout(timer)
    request?.abort()
  })

  return (
    <div {...native} ref={(element) => { root = element; if (typeof local.ref === 'function') local.ref(element) }} id={id()} class={classes('relative', local.class)} data-endpoint={local.endpoint}>
      <Show when={local.label}><label for={`${id()}-input`} class="block text-sm font-medium text-gray-700">{local.label}</label></Show>
      <input
        {...local.inputProps}
        ref={(element) => { input = element; if (typeof local.inputProps?.ref === 'function') local.inputProps.ref(element) }}
        id={`${id()}-input`}
        type="text"
        role="combobox"
        placeholder={local.placeholder}
        value={query()}
        class={classes('border p-2 w-full mt-1', local.inputProps?.class)}
        disabled={local.disabled}
        readOnly={local.readOnly}
        required={local.required}
        autocomplete="off"
        aria-expanded={open()}
        aria-controls={listID()}
        aria-autocomplete="list"
        aria-activedescendant={activeIndex() >= 0 ? `${id()}-option-${activeIndex()}` : undefined}
        onFocus={(event) => { if (!inactive()) setOpen(true); callHandler(local.inputProps?.onFocus, event) }}
        onInput={(event) => { search(event.currentTarget.value); callHandler(local.inputProps?.onInput, event) }}
        onKeyDown={keydown}
      />
      <input type="hidden" name={local.name} form={local.form} value={value()} disabled={local.disabled} />
      <Show when={open() && !inactive()}>
        <ul id={listID()} role="listbox" class="absolute border bg-white w-full mt-1 max-h-60 overflow-auto z-10">
          <Show when={loading()}><li class="p-2 text-gray-300" role="status">{labels().loading}</li></Show>
          <Show when={!loading() && failed()}><li class="p-2 text-gray-300" role="alert">{labels().loadError}</li></Show>
          <Show when={!loading() && !failed() && options().length === 0}><li class="p-2 text-gray-300">{labels().nothingFound}</li></Show>
          <For each={options()}>{(option, index) => (
            <li
              id={`${id()}-option-${index()}`}
              role="option"
              aria-selected={value() === option.value}
              aria-disabled={option.disabled || undefined}
              class={classes('p-2 hover:bg-gray-100 cursor-pointer', activeIndex() === index() && 'bg-gray-100', option.disabled && 'opacity-40 !cursor-not-allowed')}
              onMouseEnter={() => !option.disabled && setActiveIndex(index())}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => select(option)}
            >{option.label}</li>
          )}</For>
        </ul>
      </Show>
    </div>
  )
}
