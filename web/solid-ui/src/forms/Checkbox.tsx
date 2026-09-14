import { createEffect, createUniqueId, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { CheckIcon, MinusIcon } from '../internal/icons'

export interface CheckboxProps extends Omit<JSX.InputHTMLAttributes<HTMLInputElement>, 'type'> {
  defaultChecked?: boolean
  label?: JSX.Element
  error?: JSX.Element
  indeterminate?: boolean
  wrapperClass?: string
}

export function Checkbox(props: CheckboxProps) {
  const generatedID = createUniqueId()
  const [local, native] = splitProps(props, ['children', 'class', 'id', 'label', 'error', 'indeterminate', 'wrapperClass', 'ref', 'checked', 'defaultChecked', 'aria-describedby'])
  const id = () => local.id ?? generatedID
  const errorID = () => `${id()}-error`
  let input!: HTMLInputElement
  createEffect(() => { input.indeterminate = Boolean(local.indeterminate) })
  const setRef = (element: HTMLInputElement) => {
    input = element
    if (typeof local.ref === 'function') local.ref(element)
  }
  return (
    <div class={local.wrapperClass}>
      <label for={id()} class={classes('form-control-label flex items-center cursor-pointer gap-3', local.class)}>
        <input
          {...native}
          ref={setRef}
          type="checkbox"
          id={id()}
          checked={local.checked ?? local.defaultChecked}
          class="peer appearance-none absolute h-0"
          aria-invalid={local.error ? true : native['aria-invalid']}
          aria-describedby={[local['aria-describedby'], local.error ? errorID() : undefined].filter(Boolean).join(' ') || undefined}
        />
        <div class="w-5 h-5 rounded-[5px] border border-default duration-200 flex items-center justify-center hover:border-brand peer-disabled:border-disabled peer-indeterminate:bg-brand-500 peer-checked:border-brand peer-checked:bg-brand-500 peer-checked:text-white peer-indeterminate:text-white group">
          <CheckIcon size={16} class="scale-0 peer-indeterminate:group-[]:hidden peer-checked:group-[]:scale-100" />
          <MinusIcon size={16} class="scale-0 hidden peer-indeterminate:group-[]:inline peer-indeterminate:group-[]:scale-100" />
        </div>
        <Show when={local.label !== undefined && local.label !== ''}><span>{local.label}</span></Show>
        {local.children}
      </label>
      <Show when={local.error}>
        <small id={errorID()} class="text-xs text-red-500 mt-1" data-testid="field-error" data-field-id={id()} role="alert">{local.error}</small>
      </Show>
    </div>
  )
}
