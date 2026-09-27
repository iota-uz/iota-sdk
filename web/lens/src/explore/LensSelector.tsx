import { createEffect, createSignal, For, onCleanup, onMount, Show, type JSX } from 'solid-js'
import type { Perspective } from '../contract'
import { CaretDown, Check } from '../icons'

/** How many perspectives render as inline pills before overflowing to a menu. */
const inlineLimit = 4

/**
 * The standalone perspective switcher of a focus canvas. Radiogroup semantics
 * with roving focus: arrows move focus, Enter/Space (or click) selects, so
 * scanning the options never fires a navigation per keystroke. Up to four
 * perspectives render inline under a sliding active indicator; more than four
 * keep the first three inline and fold the rest behind a menu.
 */
export interface LensSelectorProps {
  perspectives: Array<Perspective>
  activeId?: string
  label: string
  moreLabel: string
  onSelect: (id: string) => void
}

interface Indicator {
  left: number
  width: number
}

export function LensSelector(props: LensSelectorProps): JSX.Element {
  const perspectives = () => props.perspectives
  const inline = () => perspectives().length <= inlineLimit ? perspectives() : perspectives().slice(0, inlineLimit - 1)
  const overflow = () => perspectives().length <= inlineLimit ? [] : perspectives().slice(inlineLimit - 1)
  const activeInOverflow = () => overflow().some(({ id }) => id === props.activeId)
  let groupRef: HTMLDivElement | undefined
  let menuRef: HTMLDivElement | undefined
  let menuButtonRef: HTMLButtonElement | undefined
  const pillRefs: Record<string, HTMLButtonElement | undefined> = {}
  const [indicator, setIndicator] = createSignal<Indicator>()
  const [menuOpen, setMenuOpen] = createSignal(false)

  const measure = () => {
    const pill = props.activeId ? pillRefs[props.activeId] : undefined
    if (!pill) {
      setIndicator(undefined)
      return
    }
    const next = { left: pill.offsetLeft, width: pill.offsetWidth }
    setIndicator((current) => (
      current && current.left === next.left && current.width === next.width ? current : next
    ))
  }

  onMount(() => {
    // The resting indicator must exist before first paint of the next frame,
    // like the layout effect this replaces.
    measure()
  })
  createEffect(() => {
    void props.activeId
    void perspectives()
    measure()
  })

  createEffect(() => {
    const listener = measure
    globalThis.addEventListener('resize', listener)
    // Web fonts landing after mount change pill widths; re-measure once ready.
    const fonts = (globalThis.document as Document & { fonts?: FontFaceSet }).fonts
    void fonts?.ready.then(listener)
    onCleanup(() => globalThis.removeEventListener('resize', listener))
  })

  createEffect(() => {
    if (!menuOpen()) return
    const onPress = (event: MouseEvent) => {
      const node = event.target as Node | null
      if (node && (menuRef?.contains(node) || menuButtonRef?.contains(node))) return
      setMenuOpen(false)
    }
    globalThis.document.addEventListener('mousedown', onPress)
    onCleanup(() => globalThis.document.removeEventListener('mousedown', onPress))
  })

  const select = (id: string) => {
    setMenuOpen(false)
    if (id !== props.activeId) props.onSelect(id)
  }

  // Roving focus across the inline radios and the menu button.
  const onGroupKeyDown = (event: KeyboardEvent) => {
    if (!['ArrowRight', 'ArrowLeft', 'ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
    const group = groupRef
    if (!group) return
    event.preventDefault()
    event.stopPropagation()
    const stops = Array.from(group.querySelectorAll<HTMLElement>('[data-lens-stop]'))
    if (stops.length === 0) return
    const active = globalThis.document.activeElement as HTMLElement | null
    const index = active ? stops.indexOf(active) : -1
    let next = index
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown') next = index < 0 ? 0 : (index + 1) % stops.length
    if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') next = index < 0 ? stops.length - 1 : (index - 1 + stops.length) % stops.length
    if (event.key === 'Home') next = 0
    if (event.key === 'End') next = stops.length - 1
    stops[next]?.focus()
  }

  const onMenuKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'Escape') return
    // The panel maps Escape to drill-back; closing the menu must consume it.
    event.preventDefault()
    event.stopPropagation()
    setMenuOpen(false)
    menuButtonRef?.focus()
  }

  // The roving tabIndex lands on the active pill; when the active perspective
  // hides in the overflow (or none is active) the first stop takes it.
  const tabStopId = () => props.activeId && !activeInOverflow() && inline().some(({ id }) => id === props.activeId)
    ? props.activeId
    : activeInOverflow() ? undefined : inline()[0]?.id

  return (
    <div class="lens-focus-lens">
      <span class="lens-focus-lens-label">{props.label}</span>
      <div
        aria-label={props.label}
        class="lens-focus-lens-pills"
        onKeyDown={onGroupKeyDown}
        ref={groupRef}
        role="radiogroup"
        tabIndex={-1}
      >
        <Show when={indicator()}>
          {(value) => (
            // The resting position is an inline transform, so a VR screenshot is
            // deterministic; the CSS transition only choreographs the slide
            // between two resting states.
            <span
              aria-hidden="true"
              class="lens-focus-lens-indicator"
              style={{ transform: `translateX(${value().left}px)`, width: `${value().width}px` }}
            />
          )}
        </Show>
        <For each={inline()}>
          {(perspective) => {
            const active = () => perspective.id === props.activeId
            return (
              <button
                aria-checked={active()}
                class={`lens-focus-lens-pill${active() ? ' lens-focus-lens-pill-active' : ''}`}
                data-lens-stop
                onClick={() => select(perspective.id)}
                ref={(element) => { pillRefs[perspective.id] = element }}
                role="radio"
                tabIndex={perspective.id === tabStopId() ? 0 : -1}
                type="button"
              >
                {perspective.label}
              </button>
            )
          }}
        </For>
        <Show when={overflow().length > 0}>
          {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions -- delegate from the button/listbox options without making this layout wrapper a focus stop. */}
          <div
            class="lens-focus-lens-more"
            onKeyDown={onMenuKeyDown}
          >
            <button
              aria-expanded={menuOpen()}
              aria-haspopup="listbox"
              class={`lens-focus-lens-pill${activeInOverflow() ? ' lens-focus-lens-pill-active' : ''}`}
              data-lens-stop
              onClick={() => setMenuOpen((open) => !open)}
              ref={menuButtonRef}
              tabIndex={activeInOverflow() || tabStopId() === undefined ? 0 : -1}
              type="button"
            >
              {activeInOverflow()
                ? overflow().find(({ id }) => id === props.activeId)?.label ?? props.moreLabel
                : props.moreLabel}
              <CaretDown className="lens-focus-lens-more-caret" size={12} />
            </button>
            <Show when={menuOpen()}>
              <div class="lens-focus-lens-menu" ref={menuRef} role="listbox" aria-label={props.moreLabel}>
                <For each={overflow()}>
                  {(perspective) => {
                    const active = () => perspective.id === props.activeId
                    return (
                      <button
                        aria-selected={active()}
                        class="lens-focus-lens-option"
                        onClick={() => select(perspective.id)}
                        role="option"
                        type="button"
                      >
                        <span class="lens-focus-lens-option-label">{perspective.label}</span>
                        <Show when={active()}>
                          <Check size={14} />
                        </Show>
                      </button>
                    )
                  }}
                </For>
              </div>
            </Show>
          </div>
        </Show>
      </div>
    </div>
  )
}
