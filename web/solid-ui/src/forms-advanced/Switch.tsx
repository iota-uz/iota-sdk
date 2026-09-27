import { createUniqueId, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { callHandler } from '../internal/events'
import { createControllable } from './state'

export type SwitchSize = 'sm' | 'md' | 'lg'

export interface SwitchProps extends Omit<JSX.InputHTMLAttributes<HTMLInputElement>, 'type'> {
  defaultChecked?: boolean
  label?: JSX.Element
  error?: JSX.Element
  size?: SwitchSize
  labelClass?: string
  labelProps?: JSX.LabelHTMLAttributes<HTMLLabelElement>
}

const sizeClasses: Record<SwitchSize, string> = {
  sm: 'w-9 h-5 after:top-[2px] after:start-[2px] after:h-4 after:w-4',
  md: 'w-11 h-6 after:top-[2px] after:start-[2px] after:h-5 after:w-5',
  lg: 'w-14 h-7 after:top-0.5 after:start-[4px] after:h-6 after:w-6',
}

export function Switch(props: SwitchProps) {
  const generatedID = createUniqueId()
  const [local, native] = splitProps(props, ['children', 'class', 'id', 'checked', 'defaultChecked', 'label', 'error', 'size', 'labelClass', 'labelProps', 'onChange', 'aria-describedby'])
  const [checked, setChecked] = createControllable(() => local.checked, local.defaultChecked ?? false)
  const id = () => local.id ?? generatedID
  const errorID = () => `${id()}-error`
  const onChange: JSX.EventHandler<HTMLInputElement, Event> = (event) => {
    setChecked(event.currentTarget.checked)
    callHandler(local.onChange, event)
  }
  return (
    <div>
      <label {...local.labelProps} for={id()} class={classes('form-control-label inline-flex items-center cursor-pointer gap-3', local.labelClass, local.labelProps?.class)}>
        <input {...native} id={id()} type="checkbox" class={classes('appearance-none h-0 absolute peer', local.class)} checked={checked()} onChange={onChange} role="switch" aria-invalid={local.error ? true : native['aria-invalid']} aria-describedby={[local['aria-describedby'], local.error ? errorID() : undefined].filter(Boolean).join(' ') || undefined} />
        <div class={classes(
          'relative bg-gray-200 rounded-full',
          'peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white peer-checked:bg-brand-600',
          'peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-brand-300',
          "after:content-[''] after:absolute after:bg-white after:border-default after:border after:rounded-full after:transition-all peer-disabled:after:border-disabled peer-disabled:opacity-50 peer-disabled:cursor-not-allowed",
          sizeClasses[local.size ?? 'md'],
        )} />
        <Show when={local.label !== undefined && local.label !== ''}><span class="text-sm">{local.label}</span></Show>
        {local.children}
      </label>
      <Show when={local.error}><small id={errorID()} class="text-xs text-red-500 mt-1" role="alert">{local.error}</small></Show>
    </div>
  )
}
