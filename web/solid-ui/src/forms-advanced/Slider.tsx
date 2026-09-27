import { createUniqueId, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { callHandler } from '../internal/events'
import { createControllable } from './state'

export type SliderValueFormat = 'auto' | 'float10' | 'float100' | ((value: number) => string)

export interface SliderProps extends Omit<JSX.InputHTMLAttributes<HTMLInputElement>, 'type' | 'value' | 'min' | 'max' | 'step'> {
  value?: number
  defaultValue?: number
  onValueChange?: (value: number) => void
  min?: number
  max?: number
  step?: number
  label?: JSX.Element
  helpText?: JSX.Element
  error?: JSX.Element
  valueFormat?: SliderValueFormat
}

function formatValue(value: number, step: number, format: SliderValueFormat): string {
  if (typeof format === 'function') return format(value)
  if (format === 'float10') return value.toFixed(1)
  if (format === 'float100') return value.toFixed(2)
  if (step < 0.1) return value.toFixed(2)
  if (step < 1) return value.toFixed(1)
  return String(value)
}

export function Slider(props: SliderProps) {
  const generatedID = createUniqueId()
  const [local, native] = splitProps(props, [
    'class', 'id', 'value', 'defaultValue', 'onInput', 'onChange', 'onValueChange', 'min', 'max', 'step', 'label', 'helpText', 'error', 'valueFormat', 'aria-describedby',
  ])
  const min = () => local.min ?? 0
  const max = () => local.max ?? 100
  const step = () => local.step ?? 1
  const id = () => local.id ?? generatedID
  const [value, setValue] = createControllable(() => local.value, local.defaultValue ?? min())
  const percentage = () => max() === min() ? 0 : Math.min(Math.max((value() - min()) / (max() - min()) * 100, 0), 100)
  const helpID = () => `${id()}-help`
  const errorID = () => `${id()}-error`
  const describedBy = () => [local['aria-describedby'], local.helpText ? helpID() : undefined, local.error ? errorID() : undefined].filter(Boolean).join(' ') || undefined
  const onInput: JSX.EventHandler<HTMLInputElement, InputEvent> = (event) => {
    const next = event.currentTarget.valueAsNumber
    setValue(next)
    local.onValueChange?.(next)
    callHandler(local.onInput, event)
  }
  const onChange: JSX.EventHandler<HTMLInputElement, Event> = (event) => callHandler(local.onChange, event)
  return (
    <div class="flex flex-col w-full">
      <div class="flex justify-between mb-2">
        <Show when={local.label !== undefined && local.label !== ''}><label for={id()} class="form-control-label">{local.label}</label></Show>
        <span class="text-sm text-gray-700" aria-hidden="true">{formatValue(value(), step(), local.valueFormat ?? 'auto')}</span>
      </div>
      <div class="relative h-5 flex items-center">
        <div class="absolute h-1 w-full bg-gray-200 rounded-full" />
        <div class="absolute h-1 bg-brand-500 rounded-full" style={{ width: `${percentage()}%` }} />
        <input
          {...native}
          id={id()}
          type="range"
          min={min()}
          max={max()}
          step={step()}
          value={value()}
          class={classes('slider-thumb appearance-none w-full h-1 bg-transparent cursor-pointer', local.class)}
          onInput={onInput}
          onChange={onChange}
          aria-invalid={local.error ? true : native['aria-invalid']}
          aria-describedby={describedBy()}
          aria-valuetext={formatValue(value(), step(), local.valueFormat ?? 'auto')}
        />
      </div>
      <Show when={local.helpText}><small id={helpID()} class="text-xs text-gray-700 mt-1">{local.helpText}</small></Show>
      <Show when={local.error}><small id={errorID()} class="text-xs text-red-500 mt-1" role="alert">{local.error}</small></Show>
    </div>
  )
}

