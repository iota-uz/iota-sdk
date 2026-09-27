import { createUniqueId, splitProps, type JSX } from 'solid-js'
import { Button } from '../forms/Button'
import { Card } from '../display/Card'
import { classes } from '../internal/classes'

export interface FormActionsProps extends JSX.HTMLAttributes<HTMLDivElement> {
  saveLabel?: JSX.Element
  deleteLabel?: JSX.Element
  showDelete?: boolean
  submitting?: boolean
  deleting?: boolean
  onDelete?: (event: MouseEvent) => void
  saveButtonId?: string
  deleteButtonId?: string
}

export function FormActions(props: FormActionsProps) {
  const [local, native] = splitProps(props, ['saveLabel', 'deleteLabel', 'showDelete', 'submitting', 'deleting', 'onDelete', 'saveButtonId', 'deleteButtonId', 'class', 'children'])
  const instance = createUniqueId()
  return <div {...native} class={classes('h-16 md:h-20 shadow-t-lg border-t w-full flex items-center justify-end px-4 md:px-8 bg-surface-300 border-t-primary mt-auto gap-4', local.class)}>
    {local.showDelete && <Button id={local.deleteButtonId ?? `delete-user-btn-${instance}`} name="_action" value="delete" type="button" variant="danger" size="md" loading={local.deleting} disabled={local.deleting || local.submitting} onClick={(event) => local.onDelete?.(event)}>{local.deleteLabel ?? 'Delete'}</Button>}
    {local.children}
    <Button id={local.saveButtonId ?? `save-btn-${instance}`} name="_action" value="save" type="submit" variant="primary" size="md" loading={local.submitting} disabled={local.submitting || local.deleting}>{local.saveLabel ?? 'Save'}</Button>
  </div>
}

export interface FormContentProps extends JSX.FormHTMLAttributes<HTMLFormElement> {
  fields: JSX.Element
  actions?: JSX.Element
  contentClass?: string
}

export function FormContent(props: FormContentProps) {
  const [local, native] = splitProps(props, ['fields', 'actions', 'contentClass', 'class'])
  const instance = createUniqueId()
  return <form {...native} id={native.id ?? `save-form-${instance}`} method={native.method ?? 'post'} class={classes('flex flex-col flex-1', local.class)}>
    <div class="flex-1 overflow-y-auto p-4 md:p-6"><Card contentClass={classes('grid grid-cols-1 md:grid-cols-2 gap-4', local.contentClass)}>{local.fields}</Card></div>
    {local.actions ?? <FormActions />}
  </form>
}

export interface FormLayoutProps extends FormContentProps {
  error?: JSX.Element
  wrapperClass?: string
  wrapperId?: string
}

export function FormLayout(props: FormLayoutProps) {
  const [local, formProps] = splitProps(props, ['error', 'wrapperClass', 'wrapperId'])
  const instance = createUniqueId()
  return <div class={classes('flex flex-col justify-between h-[calc(100vh-4rem)]', local.wrapperClass)} id={local.wrapperId ?? `edit-content-${instance}`}>
    {local.error !== undefined && <small data-testid="field-error" class="text-red-500" role="alert">{local.error}</small>}
    <FormContent {...formProps} />
  </div>
}

export const ScaffoldForm = FormLayout
