import { createUniqueId, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { HelpHint } from '../utilities/Help'

export interface LabelProps extends JSX.LabelHTMLAttributes<HTMLLabelElement> {
  required?: boolean
}

export function Label(props: LabelProps) {
  const [local, native] = splitProps(props, ['children', 'class', 'required'])
  return (
    <label {...native} class={classes('form-control-label mb-2', local.class)}>
      {local.children}
      <Show when={local.required}><span aria-hidden="true"> *</span></Show>
    </label>
  )
}

export interface FieldProps extends JSX.HTMLAttributes<HTMLDivElement> {
  label?: JSX.Element
  description?: JSX.Element
  helpLabel?: string
  labelFor?: string
  error?: JSX.Element
  errorId?: string
  required?: boolean
}

export function Field(props: FieldProps) {
  const [local, native] = splitProps(props, ['children', 'class', 'label', 'description', 'helpLabel', 'labelFor', 'error', 'errorId', 'required'])
  const generatedID = createUniqueId()
  const errorID = () => local.errorId ?? `${local.labelFor ?? generatedID}-error`
  return (
    <div {...native} class={classes('flex flex-col w-full', local.class)}>
      <Show when={local.label !== undefined && local.label !== ''}>
        <div class="mb-2 flex items-center gap-1.5">
          <Label class="mb-0!" for={local.labelFor} required={local.required}>{local.label}</Label>
          <Show when={local.description !== undefined && local.description !== ''}>
            <HelpHint
              data-field-help
              title={local.label}
              description={local.description}
              label={local.helpLabel ?? (typeof local.label === 'string' ? `More information about ${local.label}` : 'More information')}
            />
          </Show>
        </div>
      </Show>
      {local.children}
      <Show when={local.error}>
        <small id={errorID()} class="text-xs text-red-500 mt-1" data-testid="field-error" data-field-id={local.labelFor} role="alert">
          {local.error}
        </small>
      </Show>
    </div>
  )
}
