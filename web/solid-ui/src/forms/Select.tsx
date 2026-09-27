import { children, createEffect, createSignal, createUniqueId, For, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { CaretDownIcon } from '../internal/icons'
import { Field } from './Field'

export interface SelectOption {
  value: string
  label: JSX.Element
  disabled?: boolean
}

export interface SelectProps extends Omit<JSX.SelectHTMLAttributes<HTMLSelectElement>, 'prefix'> {
  defaultValue?: string
  label?: JSX.Element
  description?: JSX.Element
  helpLabel?: string
  error?: JSX.Element
  placeholder?: string
  prefix?: JSX.Element
  wrapperClass?: string
  options?: readonly SelectOption[]
}

export function Select(props: SelectProps) {
  let element!: HTMLSelectElement
  const generatedID = createUniqueId()
  const [local, native] = splitProps(props, [
    'children', 'class', 'id', 'value', 'defaultValue', 'onChange', 'label', 'description', 'helpLabel', 'error', 'placeholder', 'prefix', 'wrapperClass', 'options', 'aria-describedby', 'ref',
  ])
  const [uncontrolledValue, setUncontrolledValue] = createSignal(local.defaultValue ?? '')
  const id = () => local.id ?? generatedID
  const errorID = () => `${id()}-error`
  const resolvedChildren = children(() => local.children)
  const initiallySelected = (value: string) => {
    const selected = local.value ?? local.defaultValue
    return Array.isArray(selected) ? selected.map(String).includes(value) : selected !== undefined && String(selected) === value
  }

  const assignValue = (value: string | number | readonly string[]) => {
    if (Array.isArray(value)) {
      const selected = new Set(value.map(String))
      for (const option of element.options) option.selected = selected.has(option.value)
      return
    }
    element.value = String(value)
  }

  createEffect(() => {
    const value = local.value ?? uncontrolledValue()
    local.options
    resolvedChildren()
    assignValue(value as string | number | readonly string[])
  })

  return (
    <Field class={classes('shrink-0', local.wrapperClass)} label={local.label} description={local.description} helpLabel={local.helpLabel} labelFor={id()} error={local.error} errorId={errorID()} required={native.required}>
      <div class="w-full relative flex items-center">
        <Show when={local.prefix !== undefined && local.prefix !== ''}>
          <label class="inline-flex items-center justify-center text-300 text-sm whitespace-nowrap h-[2.6875rem] border-l border-t border-b border-default rounded-l-lg px-[var(--form-control-size-x)]" for={id()}>
            {local.prefix}
          </label>
        </Show>
        <select
          {...native}
          ref={(node) => {
            element = node
            if (typeof local.ref === 'function') local.ref(node)
          }}
          id={id()}
          value={local.value ?? uncontrolledValue()}
          onChange={(event) => {
            if (local.value === undefined) setUncontrolledValue(event.currentTarget.value)
            if (typeof local.onChange === 'function') local.onChange(event)
          }}
          class={classes('min-w-20 w-full appearance-none form-control form-control-input h-[2.6875rem] pr-8', local.prefix !== undefined && local.prefix !== '' && 'rounded-l-none', local.class)}
          aria-invalid={local.error ? true : native['aria-invalid']}
          aria-describedby={[local['aria-describedby'], local.error ? errorID() : undefined].filter(Boolean).join(' ') || undefined}
        >
          <Show when={local.placeholder}>
            {(placeholder) => <option value="" disabled selected={initiallySelected('')}>{placeholder()}</option>}
          </Show>
          <For each={local.options}>{(option) => <option value={option.value} disabled={option.disabled} selected={initiallySelected(option.value)}>{option.label}</option>}</For>
          {local.children}
        </select>
        <CaretDownIcon class="absolute top-1/2 right-3 pointer-events-none" style={{ transform: 'translateY(-50%)' }} size={16} />
      </div>
    </Field>
  )
}
