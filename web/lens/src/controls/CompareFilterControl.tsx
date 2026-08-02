import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import type { CompareMode, Filter } from '../contract'
import { CaretDown, Check } from '../icons'
import { useFilters, useTranslate } from '../runtime'
import { useFocusTrap } from '../runtime/focusTrap'
import { useOverlayContainer } from '../runtime/overlayContainer'
import { positionPopover } from './PeriodFilterControl'

/** How long a type-ahead buffer keeps collecting before it starts a new word. */
const typeAheadResetMs = 500

interface CompareOption {
  value: CompareMode
  label: string
}

/**
 * The comparison control: a trigger and a listbox, i.e. the same popover
 * primitive the facet filter is built from.
 *
 * It used to be a bare `<select>`. Everything around it in the header — period
 * chips, facet triggers, export buttons — is a tokenised Lens control, so the
 * one native widget in the row rendered at the platform's size, with the
 * platform's radius and the platform's focus ring (`rgb(0,95,204) auto 1px` on
 * this OS), and no amount of border and background tokens could reach the parts
 * a UA owns. It also had no way to hold the custom-interval fields, which is why
 * they hung outside it in the toolbar until the user picked "Custom".
 *
 * ARIA listbox semantics, roving tabindex: Arrow keys and Home/End move the
 * selection, Enter/Space commit it, Escape closes and returns focus to the
 * trigger, printable characters jump to the first matching option. Selecting a
 * relative mode applies and closes; selecting "custom" opens the two date fields
 * inside the popover, which is where the one commit path (Apply) lives.
 */
export function CompareFilterControl({ filter }: { filter: Filter }) {
  const comparison = filter.compare
  const { setCompare } = useFilters()
  const translate = useTranslate()
  const serverMode = comparison?.value.mode ?? 'off'
  const popoverID = useId()
  const triggerRef = useRef<HTMLButtonElement>(null)
  const popoverRef = useRef<HTMLDivElement>(null)
  const optionRefs = useRef(new Map<CompareMode, HTMLDivElement | null>())
  const focusedOptionRef = useRef<HTMLDivElement | null>(null)
  const typeAhead = useRef({ text: '', at: 0 })
  const [open, setOpen] = useState(false)
  // `mode` is what the control would apply; `focused` is only where the roving
  // tabindex currently sits. Keeping them apart is what lets a keyboard user
  // walk the list and leave with Escape without having changed anything — the
  // selection does not follow focus, because a comparison mode is a slice of the
  // whole dashboard and browsing it must not restate it.
  const [mode, setMode] = useState<CompareMode>(serverMode)
  const [focused, setFocused] = useState<CompareMode>(serverMode)
  const [start, setStart] = useState(comparison?.value.start ?? '')
  const [end, setEnd] = useState(comparison?.value.end ?? '')
  const [position, setPosition] = useState({ left: 0, top: 0 })
  const closePopover = useCallback(() => setOpen(false), [])
  const container = useOverlayContainer(open, triggerRef)
  useFocusTrap(popoverRef, open && Boolean(container), closePopover, focusedOptionRef, triggerRef)

  const serverStart = comparison?.value.start ?? ''
  const serverEnd = comparison?.value.end ?? ''
  useEffect(() => setMode(serverMode), [serverMode])
  useEffect(() => setStart(serverStart), [serverStart])
  useEffect(() => setEnd(serverEnd), [serverEnd])
  // Closing without applying drops the staged state, so the trigger can never
  // name a comparison the document is not actually showing.
  useEffect(() => {
    if (open) {
      setFocused(serverMode)
      return
    }
    setMode(serverMode)
    setStart(serverStart)
    setEnd(serverEnd)
  }, [open, serverEnd, serverMode, serverStart])

  const reposition = useCallback(() => {
    const trigger = triggerRef.current
    const popover = popoverRef.current
    if (!trigger || !popover) return
    const anchor = trigger.getBoundingClientRect()
    const size = popover.getBoundingClientRect()
    setPosition(positionPopover(
      anchor,
      { width: size.width, height: size.height },
      { width: globalThis.innerWidth || 1024, height: globalThis.innerHeight || 768 },
    ))
  }, [])

  useLayoutEffect(() => {
    if (container) reposition()
  }, [container, reposition])

  useEffect(() => {
    if (!container) return undefined
    const frame = globalThis.requestAnimationFrame(reposition)
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(reposition)
    if (popoverRef.current) observer?.observe(popoverRef.current)
    globalThis.addEventListener('resize', reposition)
    globalThis.addEventListener('scroll', reposition, true)
    return () => {
      globalThis.cancelAnimationFrame(frame)
      observer?.disconnect()
      globalThis.removeEventListener('resize', reposition)
      globalThis.removeEventListener('scroll', reposition, true)
    }
  }, [container, reposition])

  useEffect(() => {
    if (!open) return undefined
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target
      if (!(target instanceof Node)) return
      const root = triggerRef.current?.closest('.lens-compare-filter')
      if (!root?.contains(target) && !popoverRef.current?.contains(target)) setOpen(false)
    }
    globalThis.document.addEventListener('pointerdown', onPointerDown)
    return () => globalThis.document.removeEventListener('pointerdown', onPointerDown)
  }, [open])

  if (!comparison) return null

  const options: Array<CompareOption> = [
    { value: 'off', label: translate('filter.compare.off', 'Comparison off') },
    { value: 'previous_period', label: translate('filter.compare.previous', 'Previous period') },
    { value: 'year_ago', label: translate('filter.compare.yearAgo', 'Year ago') },
    { value: 'custom', label: translate('filter.compare.custom', 'Custom interval') },
  ]
  const activeLabel = options.find((option) => option.value === mode)?.label ?? options[0]!.label

  // A relative mode is a single unambiguous choice, so it commits on selection.
  // "custom" only stages: it has two more values to collect, and the Apply
  // button below the fields is the one place that commits them.
  const select = (next: CompareMode) => {
    setMode(next)
    setFocused(next)
    if (next === 'custom') {
      optionRefs.current.get(next)?.focus()
      return
    }
    setCompare(filter, { mode: next })
    setOpen(false)
  }

  const focusOption = (next: CompareMode) => {
    setFocused(next)
    optionRefs.current.get(next)?.focus()
  }

  const onOptionKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const index = options.findIndex((option) => option.value === focused)
    const step = (delta: number) => {
      event.preventDefault()
      const size = options.length
      focusOption(options[(index + delta + size) % size]!.value)
    }
    switch (event.key) {
      case 'ArrowDown': return step(1)
      case 'ArrowUp': return step(-1)
      case 'Home': {
        event.preventDefault()
        return focusOption(options[0]!.value)
      }
      case 'End': {
        event.preventDefault()
        return focusOption(options[options.length - 1]!.value)
      }
      case 'Enter':
      case ' ': {
        event.preventDefault()
        return select(focused)
      }
      default: break
    }
    // Type-ahead: printable characters jump to the first option that starts
    // with what has been typed so far, the way a native listbox does.
    if (event.key.length !== 1 || event.altKey || event.ctrlKey || event.metaKey) return
    const now = Date.now()
    const text = (now - typeAhead.current.at > typeAheadResetMs ? '' : typeAhead.current.text) + event.key.toLowerCase()
    typeAhead.current = { text, at: now }
    const match = options.find((option) => option.label.toLowerCase().startsWith(text))
    if (match) {
      event.preventDefault()
      focusOption(match.value)
    }
  }

  const onTriggerKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
    event.preventDefault()
    setOpen(true)
  }

  const rangeInvalid = !start || !end || end < start

  return (
    <div className="lens-compare-filter">
      <button
        aria-controls={popoverID}
        aria-expanded={open}
        aria-haspopup="listbox"
        className="lens-compare-trigger"
        onClick={() => setOpen((value) => !value)}
        onKeyDown={onTriggerKeyDown}
        ref={triggerRef}
        type="button"
      >
        <span className="lens-compare-trigger-label">{filter.label}</span>
        <span className="lens-compare-trigger-value">{activeLabel}</span>
        <CaretDown aria-hidden="true" />
      </button>
      {open && container && createPortal(
        <div
          className="lens-compare-popover"
          ref={popoverRef}
          style={{ left: position.left, top: position.top }}
        >
          <div aria-label={filter.label} className="lens-compare-options" id={popoverID} role="listbox">
            {options.map((option) => (
              <div
                aria-selected={option.value === mode}
                className="lens-facet-option lens-compare-option"
                key={option.value}
                onClick={() => select(option.value)}
                onKeyDown={onOptionKeyDown}
                ref={(element) => {
                  optionRefs.current.set(option.value, element)
                  if (option.value === focused) focusedOptionRef.current = element
                }}
                role="option"
                tabIndex={option.value === focused ? 0 : -1}
              >
                <span className="lens-compare-option-mark">
                  {option.value === mode && <Check aria-hidden="true" />}
                </span>
                <span className="lens-facet-option-label">{option.label}</span>
              </div>
            ))}
          </div>
          {mode === 'custom' && (
            <div className="lens-compare-custom">
              <input
                aria-label={translate('filter.compare.start', 'Comparison start')}
                onChange={(event) => setStart(event.currentTarget.value)}
                type="date"
                value={start}
              />
              <input
                aria-label={translate('filter.compare.end', 'Comparison end')}
                onChange={(event) => setEnd(event.currentTarget.value)}
                type="date"
                value={end}
              />
              <button
                className="lens-compare-apply"
                disabled={rangeInvalid}
                onClick={() => {
                  setCompare(filter, { mode: 'custom', start, end })
                  setOpen(false)
                }}
                type="button"
              >
                {translate('filter.period.apply', 'Apply')}
              </button>
            </div>
          )}
        </div>,
        container,
      )}
    </div>
  )
}
