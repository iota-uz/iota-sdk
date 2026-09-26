/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { createEffect, createSignal, createUniqueId, on, onCleanup, untrack, For, Show } from 'solid-js'
import { Portal } from 'solid-js/web'
import type { CompareMode, Filter } from '../contract'
import { CaretDown, Check } from '../icons'
import { useFilters, useTranslate } from '../runtime'
import { useFocusTrap } from '../panels/focusTrap'
import { useOverlayContainer } from '../panels/overlayContainer'
import { formatDisplayDate, maskDisplayInput, parseDisplayDate, positionPopover } from './PeriodFilterControl'
import { formatISODate, parseISODate } from './model'

/** How long a type-ahead buffer keeps collecting before it starts a new word. */
const typeAheadResetMs = 500

/**
 * A comparison boundary, typed in the runtime's own masked `dd.mm.yyyy` field.
 *
 * It used to be `<input type="date">`: the one native widget left inside a Lens
 * popover, bringing the OS date picker (its own calendar, its own week start,
 * its own locale format, its own focus ring) into a surface that already has a
 * calendar of its own three controls away. The value on the wire is unchanged —
 * ISO, or empty while the text does not parse.
 */
function CompareDateField(props: { label: string; value: string; onChange: (iso: string) => void }) {
  const translate = useTranslate()
  const parsed = parseISODate(props.value)
  const [text, setText] = createSignal(parsed ? formatDisplayDate(parsed) : '')
  const [invalid, setInvalid] = createSignal(false)
  createEffect(() => {
    const next = parseISODate(props.value)
    setText(next ? formatDisplayDate(next) : '')
    setInvalid(false)
  })
  const commit = () => {
    const trimmed = text().trim()
    if (trimmed === '') {
      setInvalid(false)
      props.onChange('')
      return
    }
    const date = parseDisplayDate(trimmed)
    if (!date) {
      setInvalid(true)
      return
    }
    setInvalid(false)
    props.onChange(formatISODate(date))
  }
  return (
    <span class="lens-compare-date" data-invalid={invalid() || undefined}>
      <input
        aria-label={props.label}
        inputmode="numeric"
        onBlur={commit}
        onChange={(event) => {
          setText(maskDisplayInput(event.currentTarget.value))
          setInvalid(false)
        }}
        onKeyDown={(event) => {
          if (event.key !== 'Enter') return
          event.preventDefault()
          commit()
        }}
        placeholder={translate('filter.period.dateFormat', 'dd.mm.yyyy')}
        type="text"
        value={text()}
      />
    </span>
  )
}

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
export function CompareFilterControl(props: { filter: Filter }) {
  const comparison = props.filter.compare
  const { setCompare } = useFilters()
  const translate = useTranslate()
  const serverMode = comparison?.value.mode ?? 'off'
  const popoverID = createUniqueId()
  let triggerRef: HTMLButtonElement | undefined
  let popoverRef: HTMLDivElement | undefined
  const optionRefs = new Map<CompareMode, HTMLDivElement | null>()
  let focusedOption: HTMLDivElement | null = null
  const typeAhead = { text: '', at: 0 }
  const [open, setOpen] = createSignal(false)
  // `mode` is what the control would apply; `focused` is only where the roving
  // tabindex currently sits. Keeping them apart is what lets a keyboard user
  // walk the list and leave with Escape without having changed anything — the
  // selection does not follow focus, because a comparison mode is a slice of the
  // whole dashboard and browsing it must not restate it.
  const [mode, setMode] = createSignal<CompareMode>(serverMode)
  const [focused, setFocused] = createSignal<CompareMode>(serverMode)
  const [start, setStart] = createSignal(comparison?.value.start ?? '')
  const [end, setEnd] = createSignal(comparison?.value.end ?? '')
  const [position, setPosition] = createSignal({ left: 0, top: 0 })
  const closePopover = () => setOpen(false)
  const container = useOverlayContainer(open, () => triggerRef)
  useFocusTrap(() => popoverRef, open() && Boolean(untrack(container)), closePopover, () => focusedOption, () => triggerRef)

  const serverStart = comparison?.value.start ?? ''
  const serverEnd = comparison?.value.end ?? ''
  createEffect(() => setMode(serverMode))
  createEffect(() => setStart(serverStart))
  createEffect(() => setEnd(serverEnd))
  // Closing without applying drops the staged state, so the trigger can never
  // name a comparison the document is not actually showing.
  createEffect(() => {
    if (open()) {
      setFocused(serverMode)
      return
    }
    setMode(serverMode)
    setStart(serverStart)
    setEnd(serverEnd)
  })

  const reposition = () => {
    const trigger = triggerRef
    const popover = popoverRef
    if (!trigger || !popover) return
    const anchor = trigger.getBoundingClientRect()
    const size = popover.getBoundingClientRect()
    setPosition(positionPopover(
      anchor,
      { width: size.width, height: size.height },
      { width: globalThis.innerWidth || 1024, height: globalThis.innerHeight || 768 },
    ))
  }

  createEffect(on(container, (current) => {
    if (current) reposition()
  }))

  createEffect(() => {
    if (!container()) return
    const frame = globalThis.requestAnimationFrame(reposition)
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(reposition)
    if (triggerRef) observer?.observe(triggerRef)
    if (popoverRef) observer?.observe(popoverRef)
    globalThis.addEventListener('resize', reposition)
    globalThis.addEventListener('scroll', reposition, true)
    onCleanup(() => {
      globalThis.cancelAnimationFrame(frame)
      observer?.disconnect()
      globalThis.removeEventListener('resize', reposition)
      globalThis.removeEventListener('scroll', reposition, true)
    })
  })

  createEffect(() => {
    if (!open()) return
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target
      if (!(target instanceof Node)) return
      const root = triggerRef?.closest('.lens-compare-filter')
      if (!root?.contains(target) && !popoverRef?.contains(target)) setOpen(false)
    }
    globalThis.document.addEventListener('pointerdown', onPointerDown)
    onCleanup(() => globalThis.document.removeEventListener('pointerdown', onPointerDown))
  })

  const options: Array<CompareOption> = [
    { value: 'off', label: translate('filter.compare.off', 'Comparison off') },
    { value: 'previous_period', label: translate('filter.compare.previous', 'Previous period') },
    { value: 'year_ago', label: translate('filter.compare.yearAgo', 'Year ago') },
    { value: 'custom', label: translate('filter.compare.custom', 'Custom interval') },
  ]
  const activeLabel = () => options.find((option) => option.value === mode())?.label ?? options[0]!.label

  // A relative mode is a single unambiguous choice, so it commits on selection.
  // "custom" only stages: it has two more values to collect, and the Apply
  // button below the fields is the one place that commits them.
  const select = (next: CompareMode) => {
    setMode(next)
    setFocused(next)
    if (next === 'custom') {
      optionRefs.get(next)?.focus()
      return
    }
    setCompare(props.filter, { mode: next })
    setOpen(false)
  }

  const focusOption = (next: CompareMode) => {
    setFocused(next)
    optionRefs.get(next)?.focus()
  }

  const onOptionKeyDown = (event: KeyboardEvent) => {
    const index = options.findIndex((option) => option.value === focused())
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
        return select(focused())
      }
      default: break
    }
    // Type-ahead: printable characters jump to the first option that starts
    // with what has been typed so far, the way a native listbox does.
    if (event.key.length !== 1 || event.altKey || event.ctrlKey || event.metaKey) return
    const now = Date.now()
    const text = (now - typeAhead.at > typeAheadResetMs ? '' : typeAhead.text) + event.key.toLowerCase()
    typeAhead.text = text
    typeAhead.at = now
    const match = options.find((option) => option.label.toLowerCase().startsWith(text))
    if (match) {
      event.preventDefault()
      focusOption(match.value)
    }
  }

  const onTriggerKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
    event.preventDefault()
    setOpen(true)
  }

  const rangeInvalid = () => !start() || !end() || end() < start()

  return (
    <div class="lens-compare-filter">
      <button
        aria-controls={popoverID}
        aria-expanded={open()}
        aria-haspopup="listbox"
        class="lens-compare-trigger"
        onClick={() => setOpen((value) => !value)}
        onKeyDown={onTriggerKeyDown}
        ref={(el) => { triggerRef = el }}
        type="button"
      >
        <span class="lens-compare-trigger-label">{props.filter.label}</span>
        <span class="lens-compare-trigger-value">{activeLabel()}</span>
        <CaretDown aria-hidden="true" />
      </button>
      <Show when={open() && container()}>
        <Portal mount={container()}>
          <div
            class="lens-compare-popover"
            ref={(el) => { popoverRef = el }}
            style={{ left: `${position().left}px`, top: `${position().top}px` }}
          >
            <div aria-label={props.filter.label} class="lens-compare-options" id={popoverID} role="listbox">
              <For each={options}>
                {(option) => (
                  <div
                    aria-selected={option.value === mode()}
                    class="lens-facet-option lens-compare-option"
                    onClick={() => select(option.value)}
                    onKeyDown={onOptionKeyDown}
                    ref={(element) => {
                      optionRefs.set(option.value, element)
                      if (option.value === focused()) focusedOption = element
                    }}
                    role="option"
                    tabIndex={option.value === focused() ? 0 : -1}
                  >
                    <span class="lens-compare-option-mark">
                      {option.value === mode() && <Check aria-hidden="true" />}
                    </span>
                    <span class="lens-facet-option-label">{option.label}</span>
                  </div>
                )}
              </For>
            </div>
            <Show when={mode() === 'custom'}>
              <div class="lens-compare-custom">
                <CompareDateField
                  label={translate('filter.compare.start', 'Comparison start')}
                  onChange={setStart}
                  value={start()}
                />
                <CompareDateField
                  label={translate('filter.compare.end', 'Comparison end')}
                  onChange={setEnd}
                  value={end()}
                />
                <button
                  class="lens-compare-apply"
                  disabled={rangeInvalid()}
                  onClick={() => {
                    setCompare(props.filter, { mode: 'custom', start: start(), end: end() })
                    setOpen(false)
                  }}
                  type="button"
                >
                  {translate('filter.period.apply', 'Apply')}
                </button>
              </div>
            </Show>
          </div>
        </Portal>
      </Show>
    </div>
  )
}
