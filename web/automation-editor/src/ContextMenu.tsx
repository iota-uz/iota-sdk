// Context menu of a step or of the canvas (right click, long press, the menu key / Shift+F10). Inside a
// multi selection it acts on the whole selection ("Delete 3 steps"). Shortcuts are shown only for a
// mouse (T3).
import { createSignal, For, onCleanup, onMount, Show, type JSX } from 'solid-js'
import { isCoarse, useDismiss } from './layer'
import { isMac } from './locale'
import { keyLabel, strings } from './strings'
import type { EditorLocale } from './types'

export type MenuItem = {
  key: string
  label: string
  shortcut?: string
  danger?: boolean
  checked?: boolean
  disabled?: boolean
  run(): void
}

export interface ContextMenuProps {
  x: number
  y: number
  bounds: { width: number; height: number }
  items: MenuItem[]
  locale: EditorLocale
  onClose(): void
}

export function ContextMenu(props: ContextMenuProps): JSX.Element {
  const t = () => strings(props.locale)
  const coarse = isCoarse()
  const mac = isMac()
  let root: HTMLDivElement | undefined
  const [active, setActive] = createSignal(0)
  const enabled = () => props.items.filter((i) => !i.disabled)
  useDismiss(() => root, () => props.onClose())

  onMount(() => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
    root?.querySelector<HTMLElement>('[role="menuitem"]:not([aria-disabled="true"]), [role="menuitemcheckbox"]')?.focus({ preventScroll: true })
    onCleanup(() => {
      if (opener?.isConnected) opener.focus({ preventScroll: true })
    })
  })

  const focusAt = (index: number) => {
    const items = [...(root?.querySelectorAll<HTMLElement>('[data-menu-item]:not([aria-disabled="true"])') ?? [])]
    const count = items.length
    if (count === 0) return
    const next = (index + count) % count
    setActive(next)
    items[next]?.focus({ preventScroll: true })
  }

  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      focusAt(active() + 1)
    } else if (event.key === 'ArrowUp') {
      event.preventDefault()
      focusAt(active() - 1)
    } else if (event.key === 'Home') {
      event.preventDefault()
      focusAt(0)
    } else if (event.key === 'End') {
      event.preventDefault()
      focusAt(enabled().length - 1)
    } else if (event.key === 'Escape' || event.key === 'Tab') {
      event.preventDefault()
      event.stopPropagation()
      props.onClose()
    }
  }

  const style = (): JSX.CSSProperties => {
    const width = 260
    const height = Math.min(48 * props.items.length + 16, 480)
    const left = Math.max(8, Math.min(props.x, props.bounds.width - width - 8))
    const top = Math.max(8, Math.min(props.y, props.bounds.height - height - 8))
    return { left: `${left}px`, top: `${top}px` }
  }

  return (
    <div ref={root} class="ae-menu nopan nowheel" style={style()} role="menu" aria-label={t().menu.label} tabindex="-1" onKeyDown={onKeyDown}>
      <For each={props.items}>
        {(item) => (
          <button
            type="button"
            data-menu-item
            class="ae-menu__item"
            classList={{ 'ae-menu__item--danger': Boolean(item.danger) }}
            role={item.checked === undefined ? 'menuitem' : 'menuitemcheckbox'}
            aria-checked={item.checked === undefined ? undefined : item.checked}
            aria-disabled={item.disabled ? 'true' : undefined}
            data-key={item.key}
            tabindex="-1"
            onClick={() => {
              if (item.disabled) return
              props.onClose()
              item.run()
            }}
          >
            <span class="ae-menu__label">{item.label}</span>
            <Show when={item.checked !== undefined}>
              <span class="ae-menu__check" aria-hidden="true">{item.checked ? '✓' : ''}</span>
            </Show>
            <Show when={item.shortcut && !coarse}>
              <kbd class="ae-kbd">{keyLabel(item.shortcut!, mac)}</kbd>
            </Show>
          </button>
        )}
      </For>
    </div>
  )
}
