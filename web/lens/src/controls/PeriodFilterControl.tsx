import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import type { Filter, PeriodValue } from '../contract'
import { currentPeriodValue, useDashboard, useFilters, useTranslate } from '../runtime'
import { isVisualRegression } from '../visualRegression'
import { Calendar } from './Calendar'
import {
  dayLabel,
  formatISODate,
  parseISODate,
  type CalendarDate,
  type RangeDraft,
  type RangeSelection,
} from './model'

export interface PeriodFilterControlProps {
  filter: Filter
  /** Fixed "today" for deterministic stories and visual regression. */
  today?: CalendarDate
}

const popoverGap = 8
const viewportPadding = 12

interface PopoverPosition {
  left: number
  top: number
}

/** Below the trigger, right edges aligned, clamped into the viewport. */
export function positionPopover(
  anchor: { left: number; right: number; bottom: number },
  size: { width: number; height: number },
  viewport: { width: number; height: number },
): PopoverPosition {
  const left = Math.min(
    Math.max(viewportPadding, anchor.right - size.width),
    Math.max(viewportPadding, viewport.width - size.width - viewportPadding),
  )
  const top = Math.min(
    anchor.bottom + popoverGap,
    Math.max(viewportPadding, viewport.height - size.height - viewportPadding),
  )
  return { left, top }
}

function sameValue(left: PeriodValue, right: PeriodValue): boolean {
  return left.start === right.start && left.end === right.end
}

function draftFromValue(value: PeriodValue): RangeDraft {
  const start = parseISODate(value.start)
  const end = parseISODate(value.end)
  if (start && end) return { start, end }
  return {}
}

/**
 * The declared period control: preset chips plus a calendar popover. All
 * state it commits goes through the filters context, i.e. into the URL — the
 * control itself owns nothing but the open popover's in-progress range.
 */
export function PeriodFilterControl({ filter, today }: PeriodFilterControlProps) {
  const { values, setPeriod } = useFilters()
  const { document: dashboardDocument } = useDashboard()
  const translate = useTranslate()
  const locale = dashboardDocument.meta.locale
  const period = filter.period
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState<RangeDraft>({})
  const [container, setContainer] = useState<HTMLElement>()
  const [position, setPosition] = useState<PopoverPosition>({ left: 0, top: 0 })
  const triggerRef = useRef<HTMLButtonElement>(null)
  const dialogRef = useRef<HTMLDivElement>(null)
  const [animate] = useState(() => {
    if (isVisualRegression()) return false
    return !globalThis.window?.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  })

  const value = period ? currentPeriodValue(period, values) : { start: '', end: '' }

  const close = useCallback((restoreFocus = true) => {
    setOpen(false)
    if (restoreFocus) triggerRef.current?.focus()
  }, [])

  const openPopover = useCallback(() => {
    setDraft(draftFromValue(period ? currentPeriodValue(period, values) : { start: '', end: '' }))
    setOpen(true)
  }, [period, values])

  // The popover portals to the end of body inside a fresh Lens root so no
  // ancestor stacking context can bury it; the theme attribute is copied from
  // the root the trigger lives in.
  useEffect(() => {
    if (!open || typeof document === 'undefined') return undefined
    const element = document.createElement('div')
    const root = triggerRef.current?.closest<HTMLElement>('.lens-root')
    element.className = `lens-root lens-overlay-root${root?.classList.contains('dark') ? ' dark' : ''}`
    if (root?.dataset.theme) element.dataset.theme = root.dataset.theme
    document.body.appendChild(element)
    setContainer(element)
    return () => {
      element.remove()
      setContainer(undefined)
    }
  }, [open])

  const reposition = useCallback(() => {
    const dialog = dialogRef.current
    const trigger = triggerRef.current
    if (!dialog || !trigger) return
    const anchor = trigger.getBoundingClientRect()
    const rect = dialog.getBoundingClientRect()
    const next = positionPopover(
      anchor,
      { width: rect.width, height: rect.height },
      { width: globalThis.innerWidth || 1024, height: globalThis.innerHeight || 768 },
    )
    setPosition((current) => (current.left === next.left && current.top === next.top ? current : next))
  }, [])

  useLayoutEffect(() => {
    if (container) reposition()
  }, [container, reposition])

  useEffect(() => {
    if (!container) return undefined
    let frame = globalThis.requestAnimationFrame(() => {
      frame = globalThis.requestAnimationFrame(reposition)
    })
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(reposition)
    if (dialogRef.current) observer?.observe(dialogRef.current)
    globalThis.addEventListener('resize', reposition)
    const fonts = (globalThis.document as Document & { fonts?: FontFaceSet }).fonts
    void fonts?.ready.then(reposition)
    return () => {
      globalThis.cancelAnimationFrame(frame)
      observer?.disconnect()
      globalThis.removeEventListener('resize', reposition)
    }
  }, [container, reposition])

  useEffect(() => {
    if (container) dialogRef.current?.focus()
  }, [container])

  useEffect(() => {
    if (!open || typeof document === 'undefined') return undefined
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.stopPropagation()
      close()
    }
    document.addEventListener('keydown', onKeyDown, true)
    return () => document.removeEventListener('keydown', onKeyDown, true)
  }, [close, open])

  if (!period) return null

  const onPick = (selection: RangeSelection) => {
    setDraft(selection.draft)
    if (!selection.complete) return
    setPeriod(filter, {
      start: formatISODate(selection.complete.start),
      end: formatISODate(selection.complete.end),
    })
    close()
  }

  const applyPreset = (value: PeriodValue) => {
    setPeriod(filter, value)
    if (open) close(false)
  }

  const allTime = translate('filter.period.allTime', 'All time')
  const start = parseISODate(value.start)
  const end = parseISODate(value.end)
  const triggerLabel = value.start === '' && value.end === ''
    ? allTime
    : start && end
      ? `${dayLabel(locale, start)} – ${dayLabel(locale, end)}`
      : translate('filter.period.custom', 'Custom range')
  const min = period.min ? parseISODate(period.min) : undefined
  const max = period.max ? parseISODate(period.max) : undefined

  return (
    <div className="lens-filter" data-filter-id={filter.id}>
      {filter.label && <span className="lens-filter-name">{filter.label}</span>}
      {(period.presets ?? []).length > 0 && (
        <span className="lens-filter-presets">
          {(period.presets ?? []).map((preset) => (
            <button
              aria-pressed={sameValue(preset.value, value)}
              className="lens-filter-chip"
              key={preset.id}
              onClick={() => applyPreset(preset.value)}
              type="button"
            >
              {preset.label}
            </button>
          ))}
        </span>
      )}
      <button
        aria-expanded={open}
        aria-haspopup="dialog"
        aria-label={`${translate('filter.period.open', 'Change period')}: ${triggerLabel}`}
        className="lens-filter-trigger"
        onClick={() => (open ? close(false) : openPopover())}
        ref={triggerRef}
        type="button"
      >
        <span aria-hidden="true" className="lens-filter-trigger-icon">▦</span>
        {triggerLabel}
      </button>
      {open && container && createPortal(
        <>
          <div aria-hidden="true" className="lens-filter-scrim" onMouseDown={() => close(false)} />
          <div
            aria-label={filter.label || translate('calendar.label', 'Calendar')}
            aria-modal="false"
            className={`lens-filter-popover${animate ? ' lens-filter-popover-enter' : ''}`}
            ref={dialogRef}
            role="dialog"
            style={{ left: position.left, top: position.top }}
            tabIndex={-1}
          >
            <Calendar
              draft={draft}
              locale={locale}
              max={max}
              min={min}
              onPick={onPick}
              today={today}
              translate={translate}
            />
            <div className="lens-filter-popover-footer">
              {period.allowEmpty && (
                <button
                  className="lens-filter-chip"
                  onClick={() => applyPreset({ start: '', end: '' })}
                  type="button"
                >
                  {allTime}
                </button>
              )}
              <button
                className="lens-filter-chip lens-filter-close"
                onClick={() => close()}
                type="button"
              >
                {translate('filter.period.close', 'Close')}
              </button>
            </div>
          </div>
        </>,
        container,
      )}
    </div>
  )
}
