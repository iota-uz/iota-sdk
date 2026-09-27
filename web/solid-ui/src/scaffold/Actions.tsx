import { For, splitProps, type JSX } from 'solid-js'
import { Button, type ButtonSize, type ButtonVariant } from '../forms/Button'
import { Dropdown, DropdownItem } from '../navigation/Dropdown'

export type ActionType = 'create' | 'export' | 'import' | 'custom'

export interface ScaffoldAction {
  id?: string
  type?: ActionType
  label: JSX.Element
  href?: string
  icon?: JSX.Element
  variant?: ButtonVariant
  size?: ButtonSize
  disabled?: boolean
  class?: string
  ariaLabel?: string
  onSelect?: (event: Event) => void
}

export type ActionProps = ScaffoldAction

export function Action(props: ActionProps) {
  const variant = () => local.variant ?? (local.type === 'create' ? 'primary' : 'secondary')
  const local = props
  return local.href
    ? <Button id={local.id} href={local.href} icon={local.icon} variant={variant()} size={local.size ?? 'normal'} class={[local.disabled && 'btn-disabled', local.class].filter(Boolean).join(' ')} aria-label={local.ariaLabel} aria-disabled={local.disabled || undefined} onClick={(event) => { if (local.disabled) event.preventDefault(); else local.onSelect?.(event) }}>{local.label}</Button>
    : <Button id={local.id} icon={local.icon} variant={variant()} size={local.size ?? 'normal'} class={local.class} aria-label={local.ariaLabel} disabled={local.disabled} onClick={(event) => local.onSelect?.(event)}>{local.label}</Button>
}

export interface ActionsProps { actions?: readonly ScaffoldAction[]; children?: JSX.Element }

export function Actions(props: ActionsProps) {
  return <><For each={props.actions}>{(action) => <Action {...action} />}</For>{props.children}</>
}

export interface RowActionsProps extends ActionsProps { class?: string }

export function RowActions(props: RowActionsProps) {
  return <div class={['flex gap-2', props.class].filter(Boolean).join(' ')}><Actions actions={props.actions}>{props.children}</Actions></div>
}

export interface ActionMenuProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onChange'> {
  label: string
  trigger: JSX.Element
  actions: readonly ScaffoldAction[]
  open?: boolean
  defaultOpen?: boolean
  onOpenChange?: (open: boolean) => void
}

export function ActionMenu(props: ActionMenuProps) {
  const [local, native] = splitProps(props, ['label', 'trigger', 'actions', 'open', 'defaultOpen', 'onOpenChange'])
  return <Dropdown {...native} label={local.label} trigger={local.trigger} open={local.open} defaultOpen={local.defaultOpen} onOpenChange={local.onOpenChange}>
    <For each={local.actions}>{(action) => action.href
      ? <DropdownItem href={action.href} aria-disabled={action.disabled || undefined} onClick={(event) => { if (action.disabled) event.preventDefault(); else action.onSelect?.(event) }}>{action.icon}{action.label}</DropdownItem>
      : <DropdownItem disabled={action.disabled} onClick={(event) => action.onSelect?.(event)}>{action.icon}{action.label}</DropdownItem>}
    </For>
  </Dropdown>
}
