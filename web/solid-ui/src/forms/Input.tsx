import { createMemo, createSignal, createUniqueId, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { callHandler } from '../internal/events'
import { EyeIcon, EyeSlashIcon } from '../internal/icons'
import { Field } from './Field'

export type InputType = 'text' | 'number' | 'email' | 'tel' | 'date' | 'datetime-local' | 'color' | 'password'

export interface InputAddonProps extends JSX.HTMLAttributes<HTMLDivElement> {}

export function InputAddon(props: InputAddonProps) {
  return <div {...props} class={classes('flex', props.class)}>{props.children}</div>
}

export interface InputProps extends Omit<JSX.InputHTMLAttributes<HTMLInputElement>, 'type'> {
  type?: InputType
  defaultValue?: string | number
  label?: JSX.Element
  description?: JSX.Element
  helpLabel?: string
  error?: JSX.Element
  wrapperClass?: string
  controlClass?: string
  wrapperProps?: JSX.HTMLAttributes<HTMLDivElement>
  addonLeft?: JSX.Element
  addonRight?: JSX.Element
  addonLeftProps?: JSX.HTMLAttributes<HTMLDivElement>
  addonRightProps?: JSX.HTMLAttributes<HTMLDivElement>
}

export function Input(props: InputProps) {
  const generatedID = createUniqueId()
  const [local, native] = splitProps(props, [
    'type', 'defaultValue', 'value', 'label', 'description', 'helpLabel', 'error', 'class', 'id', 'wrapperClass', 'controlClass', 'wrapperProps',
    'addonLeft', 'addonRight', 'addonLeftProps', 'addonRightProps', 'aria-describedby',
  ])
  const id = () => local.id ?? generatedID
  const errorID = () => `${id()}-error`
  const describedBy = () => [local['aria-describedby'], local.error ? errorID() : undefined].filter(Boolean).join(' ') || undefined

  if (local.type === 'color') {
    return (
      <div class={local.wrapperClass}>
        <label for={id()} class="block text-sm font-medium mb-2 dark:text-white">
          {local.label}
          <Show when={local.error}><small id={errorID()} class="text-xs text-red-500 mt-1" role="alert">{local.error}</small></Show>
        </label>
        <input
          {...native}
          id={id()}
          type="color"
          value={local.value ?? local.defaultValue}
          class={classes('p-1 h-10 w-14 block bg-white border border-default cursor-pointer rounded-lg disabled:border-disabled disabled:opacity-50 disabled:pointer-events-none dark:bg-neutral-900', local.class)}
          title={native.title ?? 'Choose your color'}
          aria-invalid={local.error ? true : undefined}
          aria-describedby={describedBy()}
        />
      </div>
    )
  }

  return (
    <Field class={local.wrapperClass} label={local.label} description={local.description} helpLabel={local.helpLabel} labelFor={id()} error={local.error} errorId={errorID()} required={native.required}>
      <div
        {...local.wrapperProps}
        class={classes('flex items-center w-full relative form-control', local.controlClass, local.wrapperProps?.class)}
      >
        <Show when={local.addonLeft}>
          <div {...local.addonLeftProps} class={classes('flex pl-2.5', local.addonLeftProps?.class)}>{local.addonLeft}</div>
        </Show>
        <input
          {...native}
          id={id()}
          type={local.type ?? 'text'}
          value={local.value ?? local.defaultValue}
          class={classes('form-control-input outline-none w-full', local.class)}
          aria-invalid={local.error ? true : native['aria-invalid']}
          aria-describedby={describedBy()}
        />
        <Show when={local.addonRight}>
          <div {...local.addonRightProps} class={classes('flex pr-2.5', local.addonRightProps?.class)}>{local.addonRight}</div>
        </Show>
      </div>
    </Field>
  )
}

export type InputPresetProps = Omit<InputProps, 'type'>
export const TextInput = (props: InputPresetProps) => <Input {...props} type="text" />
export const NumberInput = (props: InputPresetProps) => <Input {...props} type="number" />
export const EmailInput = (props: InputPresetProps) => <Input {...props} type="email" />
export const TelInput = (props: InputPresetProps) => <Input {...props} type="tel" />
export const DateInput = (props: InputPresetProps) => <Input {...props} type="date" />
export const DateTimeInput = (props: InputPresetProps) => <Input {...props} type="datetime-local" />
export const ColorInput = (props: InputPresetProps) => <Input {...props} type="color" />

export function PasswordInput(props: InputPresetProps) {
  const [visible, setVisible] = createSignal(false)
  const [local, native] = splitProps(props, ['addonRight'])
  const visibility = (
    <label class="flex items-center justify-center mx-2.5">
      <input
        type="checkbox"
        class="appearance-none peer password-lock"
        aria-label="Show password"
        checked={visible()}
        onChange={(event) => setVisible(event.currentTarget.checked)}
      />
      <EyeIcon size={20} class="absolute duration-200 scale-0 peer-checked:scale-100" />
      <EyeSlashIcon size={20} class="absolute duration-200 peer-checked:scale-0" />
    </label>
  )
  return <Input {...native} type={visible() ? 'text' : 'password'} addonRight={local.addonRight ?? visibility} />
}

export interface MoneyInputProps extends Omit<InputProps, 'type' | 'value' | 'defaultValue' | 'onInput' | 'name'> {
  name: string
  currency: string
  value?: number
  defaultValue?: number
  onValueChange?: (value: number | undefined) => void
  onInput?: JSX.EventHandler<HTMLInputElement, InputEvent>
  min?: number
  max?: number
  decimal?: string
  thousand?: string
  precision?: number
  convertTo?: string
  conversionRate?: number
}

function formatMoney(value: number, decimal: string, thousand: string, precision: number): string {
  const amount = (value / 10 ** precision).toFixed(precision)
  const [integer = '', fraction = ''] = amount.split('.')
  const grouped = integer.replace(/\B(?=(\d{3})+(?!\d))/g, thousand)
  return precision > 0 ? `${grouped}${decimal}${fraction}` : grouped
}

function parseMoney(value: string, decimal: string, thousand: string, precision: number): number | undefined | null {
  const trimmed = value.trim()
  if (!trimmed) return undefined
  const normalized = trimmed.split(thousand).join('').replace(decimal, '.')
  if (!/^-?(?:\d+(?:\.\d*)?|\.\d+)$/.test(normalized)) return null
  const parsed = Number(normalized)
  return Number.isFinite(parsed) ? Math.round(parsed * 10 ** precision) : null
}

export function MoneyInput(props: MoneyInputProps) {
  const generatedID = createUniqueId()
  const [internalValue, setInternalValue] = createSignal<number | undefined>(props.defaultValue)
  const [draft, setDraft] = createSignal<string | undefined>()
  const [validationError, setValidationError] = createSignal('')
  const [local, native] = splitProps(props, [
    'name', 'currency', 'value', 'defaultValue', 'onValueChange', 'onInput', 'min', 'max', 'decimal', 'thousand', 'precision',
    'convertTo', 'conversionRate', 'label', 'error', 'id', 'class', 'wrapperClass', 'controlClass', 'wrapperProps',
    'addonLeft', 'addonRight', 'addonLeftProps', 'addonRightProps', 'readOnly', 'aria-describedby', 'onBlur',
  ])
  const id = () => local.id ?? generatedID
  const decimal = () => local.decimal ?? '.'
  const thousand = () => local.thousand ?? ','
  const precision = () => local.precision ?? 2
  const cents = () => local.value ?? internalValue()
  const errorID = () => `${id()}-error`
  const validationErrorID = () => `${id()}-validation-error`
  const conversionID = () => `${id()}-conversion`
  const converted = createMemo(() => local.conversionRate && local.convertTo && cents() !== undefined ? cents()! * local.conversionRate : undefined)
  const describedBy = () => [
    local['aria-describedby'], local.error ? errorID() : undefined,
    validationError() ? validationErrorID() : undefined,
    converted() !== undefined ? conversionID() : undefined,
  ].filter(Boolean).join(' ') || undefined

  const handleInput: JSX.EventHandler<HTMLInputElement, InputEvent> = (event) => {
    const raw = event.currentTarget.value
    setDraft(raw)
    const next = parseMoney(raw, decimal(), thousand(), precision())
    if (next === null) {
      setValidationError('Enter a valid amount')
      event.currentTarget.setCustomValidity('Enter a valid amount')
      callHandler(local.onInput, event)
      return
    }
    if (next === undefined) {
      setValidationError('')
      event.currentTarget.setCustomValidity('')
      if (local.value === undefined) setInternalValue(undefined)
      local.onValueChange?.(undefined)
      callHandler(local.onInput, event)
      return
    }
    const invalid = local.min !== undefined && next < local.min
      ? `Minimum is ${formatMoney(local.min, decimal(), thousand(), precision())}`
      : local.max !== undefined && next > local.max
        ? `Maximum is ${formatMoney(local.max, decimal(), thousand(), precision())}`
        : ''
    setValidationError(invalid)
    event.currentTarget.setCustomValidity(invalid)
    if (local.value === undefined) setInternalValue(next)
    local.onValueChange?.(next)
    callHandler(local.onInput, event)
  }

  return (
    <Field class={local.wrapperClass} label={local.label} labelFor={id()} required={native.required}>
      <div class="w-full">
        <input type="hidden" name={local.name} value={cents() ?? ''} />
        <div {...local.wrapperProps} class={classes('flex items-center w-full relative form-control', local.controlClass, local.wrapperProps?.class)}>
          <Show when={local.addonLeft}>
            <div {...local.addonLeftProps} class={classes('flex pl-2.5', local.addonLeftProps?.class)}>{local.addonLeft}</div>
          </Show>
          <input
            {...native}
            id={id()}
            type="text"
            inputmode="decimal"
            autocomplete="off"
            class={classes('form-control-input outline-none w-full', local.class)}
            value={draft() ?? (cents() === undefined ? '' : formatMoney(cents()!, decimal(), thousand(), precision()))}
            onInput={handleInput}
            onBlur={(event) => {
              if (parseMoney(event.currentTarget.value, decimal(), thousand(), precision()) !== null) setDraft(undefined)
              callHandler(local.onBlur, event)
            }}
            readOnly={local.readOnly}
            aria-invalid={Boolean(local.error || validationError()) || undefined}
            aria-describedby={describedBy()}
          />
          <Show when={local.addonRight}>
            <div {...local.addonRightProps} class={classes('flex pr-2.5', local.addonRightProps?.class)}>{local.addonRight}</div>
          </Show>
        </div>
        <Show when={local.error}>
          <small id={errorID()} class="text-xs text-red-500 mt-1" data-testid="field-error" data-field-id={id()} role="alert">{local.error}</small>
        </Show>
        <Show when={validationError()}>
          <small id={validationErrorID()} class="text-xs text-red-500 mt-1" role="alert">{validationError()}</small>
        </Show>
        <Show when={converted() !== undefined}>
          <small id={conversionID()} class="text-xs text-gray-300 mt-1">≈ {formatMoney(converted()!, decimal(), thousand(), precision())} {local.convertTo}</small>
        </Show>
      </div>
    </Field>
  )
}
