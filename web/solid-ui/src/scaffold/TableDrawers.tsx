import { createSignal, For, splitProps, type JSX } from 'solid-js'
import { Drawer } from '../overlays/Drawer'
import { classes } from '../internal/classes'
import { RowActions, type RowActionsProps } from './Actions'

function CloseIcon() { return <svg aria-hidden="true" width="20" height="20" viewBox="0 0 256 256" fill="none" stroke="currentColor" stroke-width="16"><circle cx="128" cy="128" r="96" /><path d="m96 96 64 64m0-64-64 64" stroke-linecap="round" /></svg> }

export interface DefaultTableDrawerProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'title'> {
  title: JSX.Element
  open?: boolean
  defaultOpen?: boolean
  onOpenChange?: (open: boolean) => void
  onAfterClose?: () => void
  closeLabel?: string
}

export function DefaultTableDrawer(props: DefaultTableDrawerProps) {
  const [local, native] = splitProps(props, ['title', 'open', 'defaultOpen', 'onOpenChange', 'onAfterClose', 'closeLabel', 'children'])
  const [internal, setInternal] = createSignal(local.defaultOpen ?? true)
  const expanded = () => local.open ?? internal()
  const change = (open: boolean) => { if (local.open === undefined) setInternal(open); local.onOpenChange?.(open); if (!open) local.onAfterClose?.() }
  return <Drawer direction="rtl" open={expanded()} onOpenChange={change} class="flex items-stretch"><div {...native} class={classes('bg-white dark:bg-gray-900 w-full md:w-2/3 ml-auto', native.class)}><div class="flex flex-col h-full"><div class="flex justify-between px-4 py-3 border-b border-subtle"><h3 class="text-lg font-medium">{local.title}</h3><div><button type="button" class="cursor-pointer" aria-label={local.closeLabel ?? 'Close'} onClick={() => change(false)}><CloseIcon /></button></div></div><div class="flex-1 min-h-0 overflow-y-auto">{local.children}</div></div></div></Drawer>
}

export type DetailField =
  | { name: string; label: JSX.Element; type?: 'text' | 'html' | 'date' | 'time' | 'datetime'; value?: JSX.Element }
  | { name: string; label: JSX.Element; type: 'boolean'; value?: boolean }
  | { name: string; label: JSX.Element; type: 'badge'; value?: JSX.Element }

export interface DetailAction {
  label: JSX.Element
  href?: string
  method?: 'get' | 'delete' | string
  class?: string
  confirm?: string
  disabled?: boolean
  onSelect?: (action: DetailAction, event: MouseEvent) => void
}

function DetailValue(props: { field: DetailField }) {
  if (props.field.value === undefined || props.field.value === '') return <span class="text-gray-400 dark:text-gray-300">-</span>
  if (props.field.type === 'boolean') return props.field.value
    ? <span class="inline-flex items-center rounded-md bg-green-50 dark:bg-green-900/20 px-2 py-1 text-xs font-medium text-green-700 dark:text-green-400 ring-1 ring-inset ring-green-600/20 dark:ring-green-500/30">True</span>
    : <span class="inline-flex items-center rounded-md bg-red-50 dark:bg-red-900/20 px-2 py-1 text-xs font-medium text-red-700 dark:text-red-400 ring-1 ring-inset ring-red-600/20 dark:ring-red-500/30">False</span>
  if (props.field.type === 'badge') return <span class="inline-flex items-center rounded-md bg-blue-50 dark:bg-blue-900/20 px-2 py-1 text-xs font-medium text-blue-700 dark:text-blue-400 ring-1 ring-inset ring-blue-600/20 dark:ring-blue-500/30">{props.field.value}</span>
  return <>{props.field.value}</>
}

export interface DetailsDrawerProps extends Omit<DefaultTableDrawerProps, 'children'> { fields: readonly DetailField[]; actions?: readonly DetailAction[] }

export function DetailsDrawer(props: DetailsDrawerProps) {
  const [local, drawerProps] = splitProps(props, ['fields', 'actions'])
  return <DefaultTableDrawer {...drawerProps}><div class="p-6 space-y-4"><dl class="divide-y divide-subtle"><For each={local.fields}>{(field) => <div class="px-4 py-6 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-0"><dt class="text-sm font-medium leading-6 text-gray-900 dark:text-gray-100">{field.label}</dt><dd class="mt-1 text-sm leading-6 text-gray-700 dark:text-gray-300 sm:col-span-2 sm:mt-0"><DetailValue field={field} /></dd></div>}</For></dl>{local.actions && local.actions.length > 0 && <div class="mt-6 flex gap-3 justify-end"><For each={local.actions}>{(action) => action.method?.toLowerCase() === 'delete' || !action.href ? <button type="button" disabled={action.disabled} class={classes('btn', action.class)} onClick={(event) => { if (!action.confirm || window.confirm(action.confirm)) action.onSelect?.(action, event) }}>{action.label}</button> : <a href={action.href} aria-disabled={action.disabled || undefined} class={classes('btn', action.class)} onClick={(event) => { if (action.disabled) event.preventDefault(); else action.onSelect?.(action, event) }}>{action.label}</a>}</For></div>}</div></DefaultTableDrawer>
}

export function TableRowActions(props: RowActionsProps) { return <RowActions {...props} /> }
