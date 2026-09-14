import { createEffect, createSignal, onCleanup, onMount, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export interface DropdownProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onChange'> {
  trigger: JSX.Element
  open?: boolean
  defaultOpen?: boolean
  onOpenChange?: (open: boolean) => void
  menuClass?: string
  label?: string
}

export function Dropdown(props: DropdownProps) {
  const [local, native] = splitProps(props, ['trigger', 'open', 'defaultOpen', 'onOpenChange', 'menuClass', 'label', 'class', 'children'])
  const [internal, setInternal] = createSignal(local.defaultOpen ?? false)
  const expanded = () => local.open ?? internal()
  let details!: HTMLDetailsElement
  let summary!: HTMLElement
  const setOpen = (open: boolean) => {
    if (local.open === undefined) setInternal(open)
    local.onOpenChange?.(open)
  }
  createEffect(() => { if (details) details.open = expanded() })
  const onDocumentPointerDown = (event: PointerEvent) => {
    if (expanded() && !details.contains(event.target as Node)) setOpen(false)
  }
  const onKeyDown = (event: KeyboardEvent) => {
    const items = Array.from(details.querySelectorAll<HTMLElement>('[role="menuitem"]:not([disabled])'))
    const current = items.indexOf(event.target as HTMLElement)
    if (event.key === 'Escape') {
      event.preventDefault()
      setOpen(false)
      summary.focus()
    } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const next = current < 0 ? 0 : (current + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length
      items[next]?.focus()
    } else if (event.key === 'Home' || event.key === 'End') {
      event.preventDefault()
      items[event.key === 'Home' ? 0 : items.length - 1]?.focus()
    }
  }
  onMount(() => details.ownerDocument.addEventListener('pointerdown', onDocumentPointerDown))
  onCleanup(() => details?.ownerDocument.removeEventListener('pointerdown', onDocumentPointerDown))
  return (
    <div {...native} class={classes('relative', local.class)}>
      <details
        ref={details}
        class="relative z-10 peer"
        name="details-dropdown"
        onToggle={() => { if (details.open !== expanded()) setOpen(details.open) }}
        onKeyDown={onKeyDown}
      >
        <summary ref={summary} aria-haspopup="menu" aria-expanded={expanded()} aria-label={local.label}>{local.trigger}</summary>
        <ul role="menu" class={classes('flex flex-col gap-1 mt-1 absolute bg-surface-300 right-0 text-sm rounded-md w-44 overflow-hidden shadow-sm border border-secondary p-1', local.menuClass)}>{local.children}</ul>
      </details>
      <details aria-hidden="true" class="hidden peer-open:block" name="details-dropdown"><summary class="fixed w-full h-full left-0 top-0" /></details>
    </div>
  )
}

export interface DropdownItemProps extends JSX.ButtonHTMLAttributes<HTMLButtonElement> { href?: undefined }
export interface DropdownLinkProps extends JSX.AnchorHTMLAttributes<HTMLAnchorElement> { href: string }

export function DropdownItem(props: DropdownItemProps | DropdownLinkProps) {
  const [local, native] = splitProps(props, ['href', 'class', 'children'])
  return (
    <li>
      {local.href
        ? <a {...native as JSX.AnchorHTMLAttributes<HTMLAnchorElement>} role="menuitem" href={local.href} class={classes('block p-2 duration-200 hover:bg-surface-400 rounded-md', local.class)}>{local.children}</a>
        : <button {...native as JSX.ButtonHTMLAttributes<HTMLButtonElement>} role="menuitem" type={(native as JSX.ButtonHTMLAttributes<HTMLButtonElement>).type ?? 'button'} class={classes('block p-2 duration-200 hover:bg-surface-400 rounded-md', local.class)}>{local.children}</button>}
    </li>
  )
}

export interface DropdownFormItemProps extends JSX.FormHTMLAttributes<HTMLFormElement> {
  fields?: Readonly<Record<string, string>>
  buttonClass?: string
}

export function DropdownFormItem(props: DropdownFormItemProps) {
  const [local, native] = splitProps(props, ['fields', 'buttonClass', 'children'])
  return (
    <li><form {...native} method={native.method ?? 'post'}>
      {Object.entries(local.fields ?? {}).map(([name, value]) => <input type="hidden" name={name} value={value} />)}
      <button role="menuitem" type="submit" class={classes('block w-full p-2 text-left duration-200 hover:bg-surface-400 rounded-md', local.buttonClass)}>{local.children}</button>
    </form></li>
  )
}
