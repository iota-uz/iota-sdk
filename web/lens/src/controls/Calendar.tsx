import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from 'react'
import type { TranslationVars } from '../runtime'
import {
  addMonths,
  clampDate,
  compareDates,
  dayLabel,
  daysInMonth,
  firstDayOfWeek,
  formatISODate,
  keyboardTarget,
  monthGrid,
  monthLabel,
  rangeDayState,
  selectDay,
  sameDate,
  weekdayLabels,
  type CalendarDate,
  type CalendarKey,
  type RangeDraft,
  type RangeSelection,
} from './model'

export interface CalendarProps {
  locale: string
  draft: RangeDraft
  min?: CalendarDate
  max?: CalendarDate
  /** Fixed "today" for deterministic stories and visual regression. */
  today?: CalendarDate
  onPick: (selection: RangeSelection) => void
  translate: (key: string, fallback: string, vars?: TranslationVars) => string
}

const navigationKeys: ReadonlyArray<CalendarKey> = [
  'ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'PageUp', 'PageDown', 'Home', 'End',
]

function startOfMonth(date: CalendarDate): CalendarDate {
  return { year: date.year, month: date.month, day: 1 }
}

function inMonth(date: CalendarDate, month: CalendarDate): boolean {
  return date.year === month.year && date.month === month.month
}

/**
 * A single-month range calendar on the Lens design tokens. The grid is a
 * roving-tabindex ARIA grid: arrows move by day and week, PageUp/PageDown by
 * month, Home/End to the locale's week bounds, Enter/Space picks. Month
 * changes and selections are announced through a polite live region.
 */
/**
 * The viewer's wall-clock date. Only ever decides which month the calendar
 * opens on and the today marker — a display default, never a wire value.
 */
function localToday(): CalendarDate {
  const now = new Date()
  return { year: now.getFullYear(), month: now.getMonth() + 1, day: now.getDate() }
}

export function Calendar({ locale, draft, min, max, today, onPick, translate }: CalendarProps) {
  const firstDay = firstDayOfWeek(locale)
  const initialFocus = clampDate(draft.start ?? today ?? localToday(), min, max)
  const [focused, setFocused] = useState<CalendarDate>(initialFocus)
  const [visibleMonth, setVisibleMonth] = useState<CalendarDate>(startOfMonth(initialFocus))
  const [hover, setHover] = useState<CalendarDate>()
  const [announcement, setAnnouncement] = useState('')
  const gridRef = useRef<HTMLDivElement>(null)
  const focusPending = useRef(false)

  const disabled = useCallback((date: CalendarDate) => (
    (min !== undefined && compareDates(date, min) < 0) ||
    (max !== undefined && compareDates(date, max) > 0)
  ), [max, min])

  const showMonth = useCallback((month: CalendarDate, announce = true) => {
    setVisibleMonth(startOfMonth(month))
    if (announce) setAnnouncement(monthLabel(locale, month.year, month.month))
  }, [locale])

  const moveFocus = useCallback((date: CalendarDate) => {
    const target = clampDate(date, min, max)
    setFocused(target)
    focusPending.current = true
    if (!inMonth(target, visibleMonth)) showMonth(target)
  }, [max, min, showMonth, visibleMonth])

  // Focus follows the roving cell after keyboard movement, once the cell for
  // the (possibly new) month exists in the DOM.
  useEffect(() => {
    if (!focusPending.current) return
    focusPending.current = false
    const cell = gridRef.current?.querySelector<HTMLElement>('[data-focused="true"]')
    cell?.focus()
  }, [focused, visibleMonth])

  const pick = useCallback((date: CalendarDate) => {
    if (disabled(date)) return
    const selection = selectDay(draft, date)
    setHover(undefined)
    setFocused(date)
    if (selection.complete) {
      setAnnouncement(translate('calendar.announceRange', 'Selected {start} to {end}', {
        start: dayLabel(locale, selection.complete.start),
        end: dayLabel(locale, selection.complete.end),
      }))
    } else if (selection.draft.start) {
      setAnnouncement(translate('calendar.announceStart', '{date} chosen as range start', {
        date: dayLabel(locale, selection.draft.start),
      }))
    }
    onPick(selection)
  }, [disabled, draft, locale, onPick, translate])

  const onKeyDown = useCallback((event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      pick(focused)
      return
    }
    if (!(navigationKeys as ReadonlyArray<string>).includes(event.key)) return
    event.preventDefault()
    moveFocus(keyboardTarget(focused, event.key as CalendarKey, firstDay))
  }, [firstDay, focused, moveFocus, pick])

  const weeks = monthGrid(visibleMonth.year, visibleMonth.month, firstDay)
  const weekdays = weekdayLabels(locale, firstDay)
  const heading = monthLabel(locale, visibleMonth.year, visibleMonth.month)
  const focusInMonth = inMonth(focused, visibleMonth)
  // The roving cell: the focused date when visible, otherwise the first
  // selectable day of the month.
  const fallbackTab = weeks.flat().find((cell) => cell.inMonth && !disabled(cell.date))?.date
  const hint = !draft.start || draft.end
    ? translate('calendar.hintStart', 'Select a start date')
    : translate('calendar.hintEnd', 'Select an end date')

  const monthNav = useCallback((offset: number) => {
    const month = addMonths(visibleMonth, offset)
    showMonth(month)
    setFocused((current) => {
      const day = Math.min(current.day, daysInMonth(month.year, month.month))
      return clampDate({ year: month.year, month: month.month, day }, min, max)
    })
  }, [max, min, showMonth, visibleMonth])

  return (
    <div className="lens-calendar">
      <div className="lens-calendar-header">
        <button
          aria-label={translate('calendar.prevMonth', 'Previous month')}
          className="lens-calendar-nav"
          onClick={() => monthNav(-1)}
          type="button"
        >
          ‹
        </button>
        <span aria-hidden="true" className="lens-calendar-month">{heading}</span>
        <button
          aria-label={translate('calendar.nextMonth', 'Next month')}
          className="lens-calendar-nav"
          onClick={() => monthNav(1)}
          type="button"
        >
          ›
        </button>
      </div>
      <div
        aria-label={`${translate('calendar.label', 'Calendar')}, ${heading}`}
        className="lens-calendar-grid"
        onKeyDown={onKeyDown}
        onMouseLeave={() => setHover(undefined)}
        ref={gridRef}
        role="grid"
      >
        <div className="lens-calendar-weekdays" role="row">
          {weekdays.map((label) => (
            <span className="lens-calendar-weekday" key={label} role="columnheader">{label}</span>
          ))}
        </div>
        {weeks.map((week) => (
          <div className="lens-calendar-week" key={formatISODate(week[0]!.date)} role="row">
            {week.map((cell) => {
              const state = rangeDayState(cell.date, draft, hover)
              const isFocusCell = focusInMonth
                ? sameDate(cell.date, focused)
                : cell.inMonth && sameDate(cell.date, fallbackTab)
              const cellDisabled = disabled(cell.date)
              return (
                <button
                  aria-disabled={cellDisabled || undefined}
                  aria-label={dayLabel(locale, cell.date)}
                  aria-selected={state === 'start' || state === 'end' || state === 'inRange'}
                  className="lens-calendar-day"
                  data-focused={isFocusCell || undefined}
                  data-outside={!cell.inMonth || undefined}
                  data-state={state === 'none' ? undefined : state}
                  data-today={today && sameDate(cell.date, today) ? true : undefined}
                  disabled={cellDisabled}
                  key={formatISODate(cell.date)}
                  onClick={() => pick(cell.date)}
                  onFocus={() => setFocused(cell.date)}
                  onMouseEnter={() => setHover(cellDisabled ? undefined : cell.date)}
                  role="gridcell"
                  tabIndex={isFocusCell ? 0 : -1}
                  type="button"
                >
                  {cell.date.day}
                </button>
              )
            })}
          </div>
        ))}
      </div>
      <p className="lens-calendar-hint">{hint}</p>
      <div aria-live="polite" className="lens-visually-hidden" role="status">{announcement}</div>
    </div>
  )
}
