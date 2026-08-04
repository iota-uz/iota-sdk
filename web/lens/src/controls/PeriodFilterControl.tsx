import { useCallback, useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import type { Filter, PeriodValue } from '../contract'
import { CalendarBlank, CaretDown, CaretLeft, CaretRight } from '../icons'
import { currentPeriodValue, useDashboard, useFilters, useTranslate } from '../runtime'
import { useOverlayContainer } from '../runtime/overlayContainer'
import { isVisualRegression } from '../visualRegression'
import { Calendar } from './Calendar'
import {
  compactRangeLabel,
  compareDates,
  daysInMonth,
  defaultPeriodPresets,
  formatISODate,
  parseISODate,
  rangeHint,
  resolvePreset,
  shiftPeriodRange,
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

/**
 * Below the trigger, left edges aligned, falling back to right-aligned and then
 * clamped into the viewport.
 *
 * It aligned right edges while the trigger was the last segment of a ~600px
 * preset tray, where that kept a 640px card on screen. The trigger is ~150px
 * now and starts the scope row, so right-alignment threw the card 490px to the
 * left of it — over the host's sidebar, anchored to nothing a reader had
 * clicked. Left-aligned it hangs off the control it belongs to, and only a
 * trigger close to the right edge falls back.
 */
export function positionPopover(
  anchor: { left: number; right: number; bottom: number },
  size: { width: number; height: number },
  viewport: { width: number; height: number },
): PopoverPosition {
  const rightEdge = viewport.width - viewportPadding
  const preferred = anchor.left + size.width <= rightEdge ? anchor.left : anchor.right - size.width
  const left = Math.min(
    Math.max(viewportPadding, preferred),
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
 * The document's own declared presets — the profitability year chips, i.e. the
 * scopes the producer decided this dashboard is read by.
 *
 * They used to be a chip row in the header, beside the trigger. Six of them
 * spent roughly 600px of the page's most contested strip restating the period
 * the trigger next to them was already printing, and the row still could not
 * hold them: at 1440px it clipped mid-word behind a gradient on an already-grey
 * tray. They render in the popover's rail now, ahead of the built-in catalogue,
 * where a list has room to be a list — and the relationship the year chips
 * really encoded, "the period before this one", is a pair of arrows on the
 * trigger itself.
 */
function declaredPresets(period: NonNullable<Filter['period']>): Array<RenderablePreset> {
  if (!period.presets || period.presets.length === 0) return []
  return period.presets
    // An open-ended declared preset is this control's own "All time", which it
    // already pins to the foot of the rail as the clear-like action it is.
    // Declared and built-in used to live in different surfaces — a header chip
    // and a rail entry — so the duplicate was invisible; in one list it is the
    // same word twice, and neither copy says which one a press would apply.
    .filter((preset) => !(period.allowEmpty && preset.value.start === '' && preset.value.end === ''))
    .map((preset) => ({ id: preset.id, label: preset.label, value: preset.value }))
}

/**
 * The relative presets surfaced inside the popover: the built-in catalog, i.e.
 * the legacy HTMX picker's quick ranges verbatim, minus anything the producer
 * already declared.
 *
 * A catalog entry can resolve to the same range as a declared one — "current
 * fiscal year" and the 2026 chip, "last fiscal year" and the 2025 chip. While
 * the two lists lived in different surfaces both simply reported the pressed
 * state, which was the legacy behaviour. In one rail that is two highlighted
 * rows for one applied period, and a reader cannot tell what distinguishes
 * them. The producer's own list wins: it named the period for this dashboard,
 * and the catalog fills in what it did not cover.
 */
function popoverPresets(
  period: NonNullable<Filter['period']>,
  today: CalendarDate,
  translate: (key: string, fallback: string) => string,
  declared: Array<RenderablePreset>,
): Array<RenderablePreset> {
  return catalogPresets(period, today, translate)
    .filter((preset) => !declared.some((other) => sameValue(other.value, preset.value)))
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
  const [position, setPosition] = useState<PopoverPosition>({ left: 0, top: 0 })
  const triggerRef = useRef<HTMLButtonElement>(null)
  const dialogRef = useRef<HTMLDivElement>(null)
  const container = useOverlayContainer(open, triggerRef)
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

  // Calendar picks build the draft and nothing else: the popover has exactly
  // one commit path (Apply), so a mis-clicked start date costs a correction
  // rather than an applied period and a document refetch. The rail's presets
  // are the deliberate exception — a single unambiguous click.
  const onPick = (selection: RangeSelection) => {
    setDraft(selection.draft)
  }

  const cancel = () => {
    const applied = draftFromValue(period ? currentPeriodValue(period, values) : { start: '', end: '' })
    setDraft(applied)
    setFields({ start: fieldFromDate(applied.start), end: fieldFromDate(applied.end) })
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
  const presets = declaredPresets(period)
  const relativePresets = popoverPresets(period, resolvedToday, translate, presets)
  const toDatePresets = relativePresets.filter((preset) => !preset.past)
  const pastPresets = relativePresets.filter((preset) => preset.past)
  const draftComplete = Boolean(draft.start && draft.end && compareDates(draft.start, draft.end) <= 0)

  const allTime = translate('filter.period.allTime', 'All time')
  const start = parseISODate(value.start)
  const end = parseISODate(value.end)
  const triggerLabel = value.start === '' && value.end === ''
    ? allTime
    : start && end
      ? compactRangeLabel(start, end)
      : translate('filter.period.custom', 'Custom range')
  const min = period.min ? parseISODate(period.min) : undefined
  const max = period.max ? parseISODate(period.max) : undefined

  // A step is offered only for a resolved range: "all time" has no length to
  // step by, and a step landing wholly outside the document's own bounds is not
  // a period this dashboard can show.
  const step = (direction: 1 | -1) => {
    if (!start || !end) return undefined
    const next = shiftPeriodRange(start, end, direction, resolvedToday)
    if (min && compareDates(next.end, min) < 0) return undefined
    if (max && compareDates(next.start, max) > 0) return undefined
    return next
  }
  const stepBack = step(-1)
  const stepForward = step(1)
  const stepLabel = (target: { start: CalendarDate; end: CalendarDate } | undefined, key: string, fallback: string) =>
    target ? `${translate(key, fallback)}: ${compactRangeLabel(target.start, target.end)}` : translate(key, fallback)

  return (
    <div
      aria-label={filter.label || translate('filter.bar.label', 'Dashboard filters')}
      className="lens-period-filter"
      data-filter-id={filter.id}
      role="group"
    >
      {/* One control that states the period and two that move it. The arrows
          carry what six year chips used to: the period before this one, in
          ~210px instead of ~600, and they keep working on a range a reader drew
          by hand or on a dashboard that declares no presets at all. */}
      <button
        aria-label={stepLabel(stepBack, 'filter.period.previous', 'Previous period')}
        className="lens-filter-step"
        disabled={!stepBack}
        onClick={() => stepBack && applyValue({ start: formatISODate(stepBack.start), end: formatISODate(stepBack.end) })}
        type="button"
      >
        <CaretLeft size={12} />
      </button>
      <button
        aria-expanded={open}
        aria-haspopup="dialog"
        aria-label={`${translate('filter.period.open', 'Change period')}: ${triggerLabel}`}
        className="lens-filter-trigger"
        onClick={() => (open ? close(false) : openPopover())}
        ref={triggerRef}
        type="button"
      >
        <CalendarBlank className="lens-filter-trigger-icon" size={14} />
        {/* The resolved range is always printed. It used to appear only for a
            custom range, so with a preset applied the one fact a report reader
            checks before quoting a number — which days these are — lived in the
            trigger's aria-label. «30 дней» is not a period; 03.07.2026 –
            01.08.2026 is. */}
        <span className="lens-filter-trigger-label">{triggerLabel}</span>
        <CaretDown className="lens-filter-trigger-caret" size={11} />
      </button>
      <button
        aria-label={stepLabel(stepForward, 'filter.period.next', 'Next period')}
        className="lens-filter-step"
        disabled={!stepForward}
        onClick={() => stepForward && applyValue({ start: formatISODate(stepForward.start), end: formatISODate(stepForward.end) })}
        type="button"
      >
        <CaretRight size={12} />
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
            {(presets.length > 0 || relativePresets.length > 0 || period.allowEmpty) && (
              // The rail leads with the producer's own periods, unheaded because
              // a list of years names itself, then two headed groups of relative
              // ones — still running, then completed — with All time pinned to
              // the foot as the clear-like action it is, not a third preset.
              <div className="lens-filter-popover-side">
                {presets.map((preset) => (
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
                {toDatePresets.length > 0 && (
                  <span className="lens-filter-preset-heading">
                    {translate('filter.period.quickSelect', 'Quick select')}
                  </span>
                )}
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
                {pastPresets.length > 0 && (
                  <span className="lens-filter-preset-heading">
                    {translate('filter.period.completed', 'Completed')}
                  </span>
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
                {period.allowEmpty && (
                  <button
                    aria-pressed={value.start === '' && value.end === ''}
                    className="lens-filter-preset lens-filter-preset-clear"
                    onClick={() => applyValue({ start: '', end: '' })}
                    type="button"
                  >
                    {allTime}
                  </button>
                )}
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
              {/* The footer states what the draft currently is — the day count
                  once it is a range, the next step while it is not — so the
                  summary has a home instead of dangling under the grid. */}
              <div className="lens-filter-popover-footer">
                <span className="lens-filter-summary" data-complete={draftComplete || undefined}>
                  {rangeHint(draft, translate)}
                </span>
                <button
                  className="lens-filter-chip lens-filter-cancel"
                  onClick={cancel}
                  type="button"
                >
                  {translate('filter.period.cancel', 'Cancel')}
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
