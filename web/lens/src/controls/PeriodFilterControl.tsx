import { useCallback, useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import type { Filter, PeriodValue } from '../contract'
import { CalendarBlank, CaretDown } from '../icons'
import { currentPeriodValue, useDashboard, useFilters, useTranslate } from '../runtime'
import { isVisualRegression } from '../visualRegression'
import { Calendar } from './Calendar'
import {
  compareDates,
  dayLabel,
  daysInMonth,
  defaultPeriodPresets,
  formatISODate,
  parseISODate,
  resolvePreset,
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

/** The viewer's wall-clock date, used only to resolve today-relative presets. */
function localToday(): CalendarDate {
  const now = new Date()
  return { year: now.getFullYear(), month: now.getMonth() + 1, day: now.getDate() }
}

interface RenderablePreset {
  id: string
  label: string
  value: PeriodValue
  /** Completed past period — rendered after the rail's divider. */
  past?: boolean
}

/**
 * The built-in relative catalog resolved against `today`. Entries whose bounds
 * fall outside the filter's min/max are dropped so a click can never produce a
 * value the declaration rejects. `allTime` is intentionally absent — the
 * control surfaces it through its own footer chip.
 */
function catalogPresets(
  period: NonNullable<Filter['period']>,
  today: CalendarDate,
  translate: (key: string, fallback: string) => string,
): Array<RenderablePreset> {
  const presets: Array<RenderablePreset> = []
  for (const def of defaultPeriodPresets) {
    const bounds = resolvePreset(def.id, today)
    if (!bounds) continue
    const value = { start: formatISODate(bounds.start), end: formatISODate(bounds.end) }
    if (period.min && value.start < period.min) continue
    if (period.max && value.end > period.max) continue
    presets.push({ id: def.id, label: translate(def.labelKey, def.fallback), value, past: def.past })
  }
  return presets
}

/**
 * The top-row chips: the document's server-declared presets when it has them
 * (server authority — e.g. the profitability year chips), otherwise the
 * built-in relative catalog.
 */
function topRowPresets(
  period: NonNullable<Filter['period']>,
  today: CalendarDate,
  translate: (key: string, fallback: string) => string,
): Array<RenderablePreset> {
  if (period.presets && period.presets.length > 0) {
    return period.presets.map((preset) => ({ id: preset.id, label: preset.label, value: preset.value }))
  }
  return catalogPresets(period, today, translate)
}

/**
 * The relative presets surfaced inside the popover: always the full built-in
 * catalog, i.e. the legacy HTMX picker's quick ranges verbatim. A catalog
 * entry may resolve to the same range as a server year-chip (last fiscal year
 * vs. the previous year's chip); both then simply report the pressed state,
 * which is the legacy behaviour too.
 */
function popoverPresets(
  period: NonNullable<Filter['period']>,
  today: CalendarDate,
  translate: (key: string, fallback: string) => string,
): Array<RenderablePreset> {
  return catalogPresets(period, today, translate)
}

function draftFromValue(value: PeriodValue): RangeDraft {
  const start = parseISODate(value.start)
  const end = parseISODate(value.end)
  if (start && end) return { start, end }
  return {}
}

const displayDatePattern = /^(\d{2})\.(\d{2})\.(\d{4})$/

/** Parses the typed `dd.mm.yyyy` display format, rejecting invalid dates. */
export function parseDisplayDate(raw: string): CalendarDate | undefined {
  const match = displayDatePattern.exec(raw.trim())
  if (!match) return undefined
  const day = Number(match[1])
  const month = Number(match[2])
  const year = Number(match[3])
  if (month < 1 || month > 12) return undefined
  if (day < 1 || day > daysInMonth(year, month)) return undefined
  return { year, month, day }
}

export function formatDisplayDate(date: CalendarDate): string {
  const day = String(date.day).padStart(2, '0')
  const month = String(date.month).padStart(2, '0')
  return `${day}.${month}.${String(date.year).padStart(4, '0')}`
}

/**
 * Input mask for the date fields: keeps digits only and re-inserts the two
 * dots of `dd.mm.yyyy` as the user types, so separators never have to be
 * typed and stray characters cannot enter the field.
 */
export function maskDisplayInput(raw: string): string {
  const digits = raw.replace(/\D/g, '').slice(0, 8)
  if (digits.length <= 2) return digits
  if (digits.length <= 4) return `${digits.slice(0, 2)}.${digits.slice(2)}`
  return `${digits.slice(0, 2)}.${digits.slice(2, 4)}.${digits.slice(4)}`
}

interface DateFieldState {
  text: string
  invalid: boolean
}

function fieldFromDate(date: CalendarDate | undefined): DateFieldState {
  return { text: date ? formatDisplayDate(date) : '', invalid: false }
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
  const [fields, setFields] = useState<{ start: DateFieldState; end: DateFieldState }>({
    start: fieldFromDate(undefined),
    end: fieldFromDate(undefined),
  })
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
    const next = draftFromValue(period ? currentPeriodValue(period, values) : { start: '', end: '' })
    setDraft(next)
    // Reset explicitly: the draft may be unchanged since the last open, which
    // would leave a previously typed (possibly invalid) text in place.
    setFields({ start: fieldFromDate(next.start), end: fieldFromDate(next.end) })
    setOpen(true)
  }, [period, values])

  // The typed fields mirror the draft: any draft change (calendar pick, a
  // fresh open, a preset) rewrites both texts and clears the invalid marks.
  // While the user is typing the draft does not move, so nothing clobbers the
  // in-progress text — only a successful blur/Enter commit does.
  const draftStartISO = draft.start ? formatISODate(draft.start) : ''
  const draftEndISO = draft.end ? formatISODate(draft.end) : ''
  useEffect(() => {
    setFields({
      start: fieldFromDate(parseISODate(draftStartISO)),
      end: fieldFromDate(parseISODate(draftEndISO)),
    })
  }, [draftStartISO, draftEndISO])

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

  const applyValue = (value: PeriodValue) => {
    setPeriod(filter, value)
    if (open) close(false)
  }

  // Typed entry updates the in-progress draft only; the calendar commits on
  // its second click, typed edits commit through the explicit Apply button —
  // mirroring the legacy picker's From/To inputs plus Apply. A field's text
  // parses into the draft on blur or Enter; text that does not parse marks
  // the field invalid and leaves the draft (the last valid range) untouched.
  const onFieldChange = (edge: 'start' | 'end', raw: string) => {
    const text = maskDisplayInput(raw)
    setFields((current) => ({ ...current, [edge]: { text, invalid: false } }))
  }

  const commitField = (edge: 'start' | 'end') => {
    const text = fields[edge].text.trim()
    if (text === '') {
      setDraft((current) => (edge === 'start' ? { end: current.end } : { start: current.start }))
      setFields((current) => ({ ...current, [edge]: { text: '', invalid: false } }))
      return
    }
    const parsed = parseDisplayDate(text)
    if (!parsed) {
      setFields((current) => ({ ...current, [edge]: { text, invalid: true } }))
      return
    }
    setDraft((current) => (edge === 'start' ? { start: parsed, end: current.end } : { start: current.start, end: parsed }))
    setFields((current) => ({ ...current, [edge]: fieldFromDate(parsed) }))
  }

  const onFieldKeyDown = (edge: 'start' | 'end') => (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== 'Enter') return
    event.preventDefault()
    commitField(edge)
  }

  const applyDraft = () => {
    // An invalid typed field reverts to the last valid draft value instead of
    // silently applying something other than what the field shows.
    if (fields.start.invalid || fields.end.invalid) {
      setFields({ start: fieldFromDate(draft.start), end: fieldFromDate(draft.end) })
      return
    }
    if (draft.start && draft.end && compareDates(draft.start, draft.end) <= 0) {
      applyValue({ start: formatISODate(draft.start), end: formatISODate(draft.end) })
    }
  }

  const resolvedToday = today ?? localToday()
  const presets = topRowPresets(period, resolvedToday, translate)
  const relativePresets = popoverPresets(period, resolvedToday, translate)
  const toDatePresets = relativePresets.filter((preset) => !preset.past)
  const pastPresets = relativePresets.filter((preset) => preset.past)
  const draftComplete = Boolean(draft.start && draft.end && compareDates(draft.start, draft.end) <= 0)

  const allTime = translate('filter.period.allTime', 'All time')
  // The active period source: a chip when the applied range matches one, the
  // trigger (a custom range) when none does. The trigger carries a persistent
  // active cue in that case, in the same visual language as a pressed chip.
  const customActive = !presets.some((preset) => sameValue(preset.value, value))
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
      {presets.length > 0 && (
        <span className="lens-filter-presets">
          {presets.map((preset) => (
            <button
              aria-pressed={sameValue(preset.value, value)}
              className="lens-filter-chip"
              key={preset.id}
              onClick={() => applyValue(preset.value)}
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
        data-active={customActive || undefined}
        onClick={() => (open ? close(false) : openPopover())}
        ref={triggerRef}
        type="button"
      >
        <CalendarBlank className="lens-filter-trigger-icon" size={14} />
        <span className="lens-filter-trigger-label">{triggerLabel}</span>
        <CaretDown className="lens-filter-trigger-caret" size={11} />
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
            {(relativePresets.length > 0 || period.allowEmpty) && (
              <div className="lens-filter-popover-side">
                <span className="lens-filter-preset-heading">
                  {translate('filter.period.quickSelect', 'Quick select')}
                </span>
                {toDatePresets.map((preset) => (
                  <button
                    aria-pressed={sameValue(preset.value, value)}
                    className="lens-filter-preset"
                    key={preset.id}
                    onClick={() => applyValue(preset.value)}
                    type="button"
                  >
                    {preset.label}
                  </button>
                ))}
                {period.allowEmpty && (
                  <button
                    aria-pressed={value.start === '' && value.end === ''}
                    className="lens-filter-preset"
                    onClick={() => applyValue({ start: '', end: '' })}
                    type="button"
                  >
                    {allTime}
                  </button>
                )}
                {pastPresets.length > 0 && (toDatePresets.length > 0 || period.allowEmpty) && (
                  <div aria-hidden="true" className="lens-filter-preset-divider" />
                )}
                {pastPresets.map((preset) => (
                  <button
                    aria-pressed={sameValue(preset.value, value)}
                    className="lens-filter-preset"
                    key={preset.id}
                    onClick={() => applyValue(preset.value)}
                    type="button"
                  >
                    {preset.label}
                  </button>
                ))}
              </div>
            )}
            <div className="lens-filter-popover-main">
              <Calendar
                draft={draft}
                locale={locale}
                max={max}
                min={min}
                onPick={onPick}
                today={today}
                translate={translate}
              />
              <div className="lens-filter-range">
                <label className="lens-filter-range-field">
                  <span className="lens-filter-range-caption">{translate('filter.period.from', 'From')}</span>
                  <span className="lens-filter-range-input" data-invalid={fields.start.invalid || undefined}>
                    <CalendarBlank className="lens-filter-range-icon" size={12} />
                    <input
                      className="lens-filter-input"
                      inputMode="numeric"
                      onBlur={() => commitField('start')}
                      onChange={(event) => onFieldChange('start', event.target.value)}
                      onKeyDown={onFieldKeyDown('start')}
                      placeholder={translate('filter.period.dateFormat', 'dd.mm.yyyy')}
                      type="text"
                      value={fields.start.text}
                    />
                  </span>
                </label>
                <span aria-hidden="true" className="lens-filter-range-sep">—</span>
                <label className="lens-filter-range-field">
                  <span className="lens-filter-range-caption">{translate('filter.period.to', 'To')}</span>
                  <span className="lens-filter-range-input" data-invalid={fields.end.invalid || undefined}>
                    <CalendarBlank className="lens-filter-range-icon" size={12} />
                    <input
                      className="lens-filter-input"
                      inputMode="numeric"
                      onBlur={() => commitField('end')}
                      onChange={(event) => onFieldChange('end', event.target.value)}
                      onKeyDown={onFieldKeyDown('end')}
                      placeholder={translate('filter.period.dateFormat', 'dd.mm.yyyy')}
                      type="text"
                      value={fields.end.text}
                    />
                  </span>
                </label>
              </div>
              <div className="lens-filter-popover-footer">
                <button
                  className="lens-filter-chip lens-filter-close"
                  onClick={() => close()}
                  type="button"
                >
                  {translate('filter.period.close', 'Close')}
                </button>
                <button
                  className="lens-filter-chip lens-filter-apply"
                  disabled={!draftComplete}
                  onClick={applyDraft}
                  type="button"
                >
                  {translate('filter.period.apply', 'Apply')}
                </button>
              </div>
            </div>
          </div>
        </>,
        container,
      )}
    </div>
  )
}
