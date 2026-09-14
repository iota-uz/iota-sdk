import { createUniqueId, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { Field } from './Field'

export interface TextareaProps extends JSX.TextareaHTMLAttributes<HTMLTextAreaElement> {
  defaultValue?: string
  label?: JSX.Element
  error?: JSX.Element
  wrapperClass?: string
}

export function Textarea(props: TextareaProps) {
  const generatedID = createUniqueId()
  const [local, native] = splitProps(props, ['label', 'error', 'wrapperClass', 'class', 'id', 'value', 'defaultValue', 'aria-describedby'])
  const id = () => local.id ?? generatedID
  const errorID = () => `${id()}-error`
  return (
    <Field class={local.wrapperClass} label={local.label} labelFor={id()} error={local.error} errorId={errorID()} required={native.required}>
      <textarea
        {...native}
        id={id()}
        value={local.value ?? local.defaultValue}
        class={classes('form-control form-control-input w-full', local.class)}
        aria-invalid={local.error ? true : native['aria-invalid']}
        aria-describedby={[local['aria-describedby'], local.error ? errorID() : undefined].filter(Boolean).join(' ') || undefined}
      />
    </Field>
  )
}
