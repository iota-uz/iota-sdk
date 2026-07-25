import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent,
} from 'react'
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

export function LensSelector({ perspectives, activeId, label, moreLabel, onSelect }: LensSelectorProps) {
  const inline = perspectives.length <= inlineLimit ? perspectives : perspectives.slice(0, inlineLimit - 1)
  const overflow = perspectives.length <= inlineLimit ? [] : perspectives.slice(inlineLimit - 1)
  const activeInOverflow = overflow.some(({ id }) => id === activeId)
  const groupRef = useRef<HTMLDivElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)
  const menuButtonRef = useRef<HTMLButtonElement>(null)
  const pillRefs = useRef<Record<string, HTMLButtonElement | null>>({})
  const [indicator, setIndicator] = useState<Indicator>()
  const [menuOpen, setMenuOpen] = useState(false)

  const measure = useCallback(() => {
    const pill = activeId ? pillRefs.current[activeId] : undefined
    if (!pill) {
      setIndicator(undefined)
      return
    }
    const next = { left: pill.offsetLeft, width: pill.offsetWidth }
    setIndicator((current) => (
      current && current.left === next.left && current.width === next.width ? current : next
    ))
  }, [activeId])

  useLayoutEffect(measure, [measure, perspectives])

  useEffect(() => {
    globalThis.addEventListener('resize', measure)
    // Web fonts landing after mount change pill widths; re-measure once ready.
    const fonts = (globalThis.document as Document & { fonts?: FontFaceSet }).fonts
    void fonts?.ready.then(measure)
    return () => globalThis.removeEventListener('resize', measure)
  }, [measure])

  useEffect(() => {
    if (!menuOpen) return undefined
    const onPress = (event: MouseEvent) => {
      const node = event.target as Node | null
      if (node && (menuRef.current?.contains(node) || menuButtonRef.current?.contains(node))) return
      setMenuOpen(false)
    }
    globalThis.document.addEventListener('mousedown', onPress)
    return () => globalThis.document.removeEventListener('mousedown', onPress)
  }, [menuOpen])

  const select = useCallback((id: string) => {
    setMenuOpen(false)
    if (id !== activeId) onSelect(id)
  }, [activeId, onSelect])

  // Roving focus across the inline radios and the menu button.
  const onGroupKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (!['ArrowRight', 'ArrowLeft', 'ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
    const group = groupRef.current
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

  const onMenuKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key !== 'Escape') return
    // The panel maps Escape to drill-back; closing the menu must consume it.
    event.preventDefault()
    event.stopPropagation()
    setMenuOpen(false)
    menuButtonRef.current?.focus()
  }

  // The roving tabIndex lands on the active pill; when the active perspective
  // hides in the overflow (or none is active) the first stop takes it.
  const tabStopId = activeId && !activeInOverflow && inline.some(({ id }) => id === activeId)
    ? activeId
    : activeInOverflow ? undefined : inline[0]?.id

  return (
    <div className="lens-focus-lens">
      <span className="lens-focus-lens-label">{label}</span>
      <div
        aria-label={label}
        className="lens-focus-lens-pills"
        onKeyDown={onGroupKeyDown}
        ref={groupRef}
        role="radiogroup"
      >
        {indicator && (
          // The resting position is an inline transform, so a VR screenshot is
          // deterministic; the CSS transition only choreographs the slide
          // between two resting states.
          <span
            aria-hidden="true"
            className="lens-focus-lens-indicator"
            style={{ transform: `translateX(${indicator.left}px)`, width: `${indicator.width}px` }}
          />
        )}
        {inline.map((perspective) => {
          const active = perspective.id === activeId
          return (
            <button
              aria-checked={active}
              className={`lens-focus-lens-pill${active ? ' lens-focus-lens-pill-active' : ''}`}
              data-lens-stop
              key={perspective.id}
              onClick={() => select(perspective.id)}
              ref={(element) => { pillRefs.current[perspective.id] = element }}
              role="radio"
              tabIndex={perspective.id === tabStopId ? 0 : -1}
              type="button"
            >
              {perspective.label}
            </button>
          )
        })}
        {overflow.length > 0 && (
          <div className="lens-focus-lens-more" onKeyDown={onMenuKeyDown}>
            <button
              aria-expanded={menuOpen}
              aria-haspopup="listbox"
              className={`lens-focus-lens-pill${activeInOverflow ? ' lens-focus-lens-pill-active' : ''}`}
              data-lens-stop
              onClick={() => setMenuOpen((open) => !open)}
              ref={menuButtonRef}
              tabIndex={activeInOverflow || tabStopId === undefined ? 0 : -1}
              type="button"
            >
              {activeInOverflow
                ? overflow.find(({ id }) => id === activeId)?.label ?? moreLabel
                : moreLabel}
              <CaretDown className="lens-focus-lens-more-caret" size={12} />
            </button>
            {menuOpen && (
              <div className="lens-focus-lens-menu" ref={menuRef} role="listbox" aria-label={moreLabel}>
                {overflow.map((perspective) => {
                  const active = perspective.id === activeId
                  return (
                    <button
                      aria-selected={active}
                      className="lens-focus-lens-option"
                      key={perspective.id}
                      onClick={() => select(perspective.id)}
                      role="option"
                      type="button"
                    >
                      <span className="lens-focus-lens-option-label">{perspective.label}</span>
                      {active && <Check size={14} />}
                    </button>
                  )
                })}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
