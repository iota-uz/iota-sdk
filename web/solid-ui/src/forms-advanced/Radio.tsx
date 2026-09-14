import { createContext, createUniqueId, Show, splitProps, useContext, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { callHandler } from '../internal/events'
import { createControllable } from './state'

export type RadioOrientation = 'vertical' | 'horizontal'

interface RadioContextValue {
  name?: string
  value: () => string | undefined
  setValue: (value: string) => void
  disabled: () => boolean
}

const RadioContext = createContext<RadioContextValue>()

export interface RadioGroupProps extends JSX.HTMLAttributes<HTMLDivElement> {
  name?: string
  value?: string
  defaultValue?: string
  onValueChange?: (value: string) => void
  label?: JSX.Element
  error?: JSX.Element
  orientation?: RadioOrientation
  disabled?: boolean
  groupClass?: string
  groupProps?: JSX.HTMLAttributes<HTMLDivElement>
}

export function RadioGroup(props: RadioGroupProps) {
  const generatedID = createUniqueId()
  const [local, native] = splitProps(props, [
    'children', 'class', 'name', 'value', 'defaultValue', 'onValueChange', 'label', 'error', 'orientation', 'disabled', 'groupClass', 'groupProps',
  ])
  const [value, setValue] = createControllable(() => local.value, local.defaultValue ?? '')
  const id = () => native.id ?? generatedID
  const labelID = () => `${id()}-label`
  const errorID = () => `${id()}-error`
  const choose = (next: string) => {
    setValue(next)
    local.onValueChange?.(next)
  }
  return (
    <RadioContext.Provider value={{ name: local.name, value, setValue: choose, disabled: () => Boolean(local.disabled) }}>
      <div {...native} class={classes('w-full', local.class)}>
        <Show when={local.label !== undefined && local.label !== ''}><h2 id={labelID()} class="form-control-label mb-2">{local.label}</h2></Show>
        <div
          {...local.groupProps}
          class={classes('flex w-full gap-2', local.orientation === 'horizontal' && 'grid-flow-col', local.groupClass, local.groupProps?.class)}
          role="radiogroup"
          aria-orientation={local.orientation ?? 'vertical'}
          aria-invalid={local.error ? true : undefined}
          aria-labelledby={local.label !== undefined && local.label !== '' ? labelID() : undefined}
          aria-describedby={local.error ? errorID() : undefined}
        >
          {local.children}
        </div>
        <Show when={local.error}><small id={errorID()} class="text-xs text-red-500 mt-1" role="alert">{local.error}</small></Show>
      </div>
    </RadioContext.Provider>
  )
}

export interface RadioProps extends Omit<JSX.InputHTMLAttributes<HTMLInputElement>, 'type' | 'value'> {
  value: string
  defaultChecked?: boolean
  label?: JSX.Element
  wrapperClass?: string
  indicatorClass?: string
}

export function Radio(props: RadioProps) {
  const context = useContext(RadioContext)
  const generatedID = createUniqueId()
  const [local, native] = splitProps(props, ['children', 'class', 'id', 'name', 'value', 'defaultChecked', 'label', 'wrapperClass', 'indicatorClass', 'checked', 'disabled', 'onChange'])
  const id = () => local.id ?? generatedID
  const [standaloneChecked, setStandaloneChecked] = createControllable(
    () => local.checked ?? (context ? context.value() === local.value : undefined),
    local.defaultChecked ?? false,
  )
  const checked = () => standaloneChecked()
  const disabled = () => Boolean(local.disabled || context?.disabled())
  const onChange: JSX.EventHandler<HTMLInputElement, Event> = (event) => {
    if (event.currentTarget.checked) {
      setStandaloneChecked(true)
      context?.setValue(local.value)
    }
    callHandler(local.onChange, event)
  }
  return (
    <label for={id()} class={classes(
      'flex items-center gap-2 bg-surface-100 text-text-300 p-2.5 rounded-lg',
      'border border-default duration-300 has-[input:checked]:border-brand has-[input:disabled]:border-disabled cursor-pointer',
      local.wrapperClass,
    )}>
      <input
        {...native}
        id={id()}
        type="radio"
        name={local.name ?? context?.name}
        value={local.value}
        class={classes('peer appearance-none absolute h-0', local.class)}
        checked={checked()}
        disabled={disabled()}
        onChange={onChange}
      />
      <div class={classes(
        'pointer-events-none w-5 h-5 border border-default rounded-full druation-300 peer-checked:border-brand peer-disabled:border-disabled relative after:absolute after:duration-300 after:w-3 after:h-3 after:rounded-full after:left-1/2 after:top-1/2 after:-translate-x-1/2 after:-translate-y-1/2 peer-checked:after:bg-brand-500',
        local.indicatorClass,
      )} />
      <span class="text-300 peer-checked:text-100 font-medium text-sm">{local.label}{local.children}</span>
    </label>
  )
}
