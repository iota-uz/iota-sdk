import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { CaretLeft, CaretRight } from '../icons'
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
  monthShortLabels,
  previewRange,
  rangeDayState,
  selectDay,
  sameDate,
  weekdayLabels,
  yearBlock,
  type CalendarDate,
  type CalendarKey,
  type MonthCell,
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
 * The viewer's wall-clock date. Only ever decides which months the calendar
 * opens on and the today marker — a display default, never a wire value.
 */
function localToday(): CalendarDate {
  const now = new Date()
  return { year: now.getFullYear(), month: now.getMonth() + 1, day: now.getDate() }
}

/** Matches the stylesheet's stacked-popover breakpoint: one pane below it. */
function useNarrow(): boolean {
  const [narrow, setNarrow] = useState(() => (
    globalThis.window?.matchMedia?.('(max-width: 540px)').matches ?? false
  ))
  useEffect(() => {
    const media = globalThis.window?.matchMedia?.('(max-width: 540px)')
    if (!media?.addEventListener) return undefined
    const onChange = (event: MediaQueryListEvent) => setNarrow(event.matches)
    media.addEventListener('change', onChange)
    return () => media.removeEventListener('change', onChange)
  }, [])
  return narrow
}

/** The day grid, or the month/year jump panel that temporarily replaces it. */
type CalendarView = 'days' | 'months' | 'years'

/**
 * A dual-pane range calendar on the Lens design tokens: two consecutive
 * months side by side (one pane below the stacked breakpoint), one navigation
 * window that steps by month. Deep jumps go through the month/year panel
 * behind the heading rather than a second pair of carets. The day cells form roving-tabindex
 * ARIA grids: arrows move by day and week, PageUp/PageDown by month, Home/End
 * to the locale's week bounds, Enter/Space picks. Out-of-month padding days
 * are decorative placeholders — never shaded, never interactive. Month
 * changes and selections are announced through a polite live region.
 */
export function Calendar({ locale, draft, min, max, today, onPick, translate }: CalendarProps) {
  const firstDay = firstDayOfWeek(locale)
  const narrow = useNarrow()
  const paneCount = narrow ? 1 : 2
  const resolvedToday = today ?? localToday()
  const initialFocus = clampDate(draft.start ?? resolvedToday, min, max)
  const [focused, setFocused] = useState<CalendarDate>(initialFocus)
  const [visibleMonth, setVisibleMonth] = useState<CalendarDate>(startOfMonth(initialFocus))
  const [hover, setHover] = useState<CalendarDate>()
  const [announcement, setAnnouncement] = useState('')
  const [view, setView] = useState<CalendarView>('days')
  const [panelYear, setPanelYear] = useState(initialFocus.year)
  const panesRef = useRef<HTMLDivElement>(null)
  const focusPending = useRef(false)

  const paneMonths = Array.from({ length: paneCount }, (_, index) => addMonths(visibleMonth, index))
  const inWindow = useCallback((date: CalendarDate) => (
    Array.from({ length: paneCount }, (_, index) => addMonths(visibleMonth, index))
      .some((month) => inMonth(date, month))
  ), [paneCount, visibleMonth])

  const disabled = useCallback((date: CalendarDate) => (
    (min !== undefined && compareDates(date, min) < 0) ||
    (max !== undefined && compareDates(date, max) > 0)
  ), [max, min])

  /** Shifts the window so `month` is visible, announcing the month shown. */
  const showMonth = useCallback((month: CalendarDate, announce = true) => {
    const target = startOfMonth(month)
    setVisibleMonth((current) => {
      if (compareDates(target, current) < 0) return target
      // Months after the window land in the last pane.
      return addMonths(target, -(paneCount - 1))
    })
    if (announce) setAnnouncement(monthLabel(locale, month.year, month.month))
  }, [locale, paneCount])

  const moveFocus = useCallback((date: CalendarDate) => {
    const target = clampDate(date, min, max)
    setFocused(target)
    focusPending.current = true
    if (!inWindow(target)) showMonth(target)
  }, [inWindow, max, min, showMonth])

  // Focus follows the roving cell after keyboard movement, once the cell for
  // the (possibly new) month exists in the DOM.
  useEffect(() => {
    if (!focusPending.current) return
    focusPending.current = false
    const cell = panesRef.current?.querySelector<HTMLElement>('[data-focused="true"]')
    cell?.focus()
  }, [focused, visibleMonth, paneCount])

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

  // The continuous range band. A committed draft owns the band; while a range
  // is in progress the hover preview does. Endpoint cells carry the side the
  // soft wash must extend toward so the band reads as one unbroken strip that
  // rounds off exactly at the outer edges of the endpoint pills. In-range
  // cells that touch a week-row edge (or an out-of-month gap) carry a cap so
  // the wash rounds off instead of bleeding to the grid border.
  const committed = draft.start && draft.end && compareDates(draft.start, draft.end) < 0
    ? { start: draft.start, end: draft.end }
    : undefined
  const band = committed ?? previewRange(draft, hover)
  const bandTone = committed ? '' : '-preview'
  const bandSide = (date: CalendarDate): string | undefined => {
    if (!band || sameDate(band.start, band.end)) return undefined
    if (sameDate(date, band.start)) return `right${bandTone}`
    if (sameDate(date, band.end)) return `left${bandTone}`
    return undefined
  }
  const washCap = (week: Array<MonthCell>, index: number): string | undefined => {
    const capLeft = index === 0 || !week[index - 1]!.inMonth
    const capRight = index === week.length - 1 || !week[index + 1]!.inMonth
    if (capLeft && capRight) return 'both'
    if (capLeft) return 'left'
    if (capRight) return 'right'
    return undefined
  }

  const weekdays = weekdayLabels(locale, firstDay)
  const focusWindow = inWindow(focused)

  const monthNav = useCallback((offset: number) => {
    const month = addMonths(visibleMonth, offset)
    setVisibleMonth(month)
    setAnnouncement(monthLabel(locale, month.year, month.month))
    setFocused((current) => clampDate(addMonths(current, offset), min, max))
  }, [locale, max, min, visibleMonth])

  const openMonthPanel = (year: number) => {
    setPanelYear(year)
    setView('months')
  }

  /** A whole month is unreachable when it falls entirely outside min/max. */
  const monthDisabled = (year: number, month: number) => (
    (min !== undefined && compareDates({ year, month, day: daysInMonth(year, month) }, min) < 0) ||
    (max !== undefined && compareDates({ year, month, day: 1 }, max) > 0)
  )

  const yearDisabled = (year: number) => (
    (min !== undefined && year < min.year) || (max !== undefined && year > max.year)
  )

  const pickMonth = (month: number) => {
    const target = { year: panelYear, month, day: 1 }
    setVisibleMonth(target)
    setFocused((current) => clampDate({ ...target, day: Math.min(current.day, daysInMonth(panelYear, month)) }, min, max))
    setAnnouncement(monthLabel(locale, panelYear, month))
    setView('days')
  }

  // The jump panel takes the day grid's place: months for one year, or the
  // twelve-year block that year sits in. It is the only way to travel far,
  // which is why the heading that opens it is a button in every pane.
  if (view !== 'days') {
    const years = yearBlock(panelYear)
    const monthsView = view === 'months'
    const step = monthsView ? 1 : 12
    const heading = monthsView ? String(panelYear) : `${years[0]} – ${years[years.length - 1]}`
    const monthNames = monthShortLabels(locale)
    return (
      <div className="lens-calendar" data-panes={paneCount} data-view={view}>
        <div className="lens-calendar-jump">
          <div className="lens-calendar-header">
            <span className="lens-calendar-nav-group">
              <button
                aria-label={translate('calendar.prevPage', 'Previous')}
                className="lens-calendar-nav"
                onClick={() => setPanelYear((current) => current - step)}
                type="button"
              >
                <CaretLeft size={12} />
              </button>
            </span>
            <button
              className="lens-calendar-month"
              onClick={() => setView(monthsView ? 'years' : 'months')}
              type="button"
            >
              {heading}
            </button>
            <span className="lens-calendar-nav-group">
              <button
                aria-label={translate('calendar.nextPage', 'Next')}
                className="lens-calendar-nav"
                onClick={() => setPanelYear((current) => current + step)}
                type="button"
              >
                <CaretRight size={12} />
              </button>
            </span>
          </div>
          <div className="lens-calendar-jump-grid">
            {monthsView
              ? monthNames.map((name, index) => (
                <button
                  aria-pressed={panelYear === visibleMonth.year && index + 1 === visibleMonth.month}
                  className="lens-calendar-jump-cell"
                  disabled={monthDisabled(panelYear, index + 1)}
                  key={name}
                  onClick={() => pickMonth(index + 1)}
                  type="button"
                >
                  {name}
                </button>
              ))
              : years.map((year) => (
                <button
                  aria-pressed={year === visibleMonth.year}
                  className="lens-calendar-jump-cell"
                  disabled={yearDisabled(year)}
                  key={year}
                  onClick={() => { setPanelYear(year); setView('months') }}
                  type="button"
                >
                  {year}
                </button>
              ))}
          </div>
        </div>
        <div aria-live="polite" className="lens-visually-hidden" role="status">{announcement}</div>
      </div>
    )
  }

  return (
    <div className="lens-calendar" data-panes={paneCount} data-view="days">
      <div
        className="lens-calendar-panes"
        onKeyDown={onKeyDown}
        onMouseLeave={() => setHover(undefined)}
        ref={panesRef}
      >
        {paneMonths.map((month, paneIndex) => {
          const weeks = monthGrid(month.year, month.month, firstDay)
          const heading = monthLabel(locale, month.year, month.month)
          const paneHasFocus = inMonth(focused, month)
          // The roving cell: the focused date when visible, otherwise the
          // first selectable day of the leading pane.
          const fallbackTab = !focusWindow && paneIndex === 0
            ? weeks.flat().find((cell) => cell.inMonth && !disabled(cell.date))?.date
            : undefined
          return (
            <div className="lens-calendar-pane" key={`${month.year}-${month.month}`}>
              <div className="lens-calendar-header">
                <span className="lens-calendar-nav-group">
                  {paneIndex === 0 && (
                    <button
                      aria-label={translate('calendar.prevMonth', 'Previous month')}
                      className="lens-calendar-nav"
                      onClick={() => monthNav(-1)}
                      type="button"
                    >
                      <CaretLeft size={12} />
                    </button>
                  )}
                </span>
                <button
                  aria-label={`${translate('calendar.chooseMonth', 'Choose month')}: ${heading}`}
                  className="lens-calendar-month"
                  onClick={() => openMonthPanel(month.year)}
                  type="button"
                >
                  {heading}
                </button>
                <span className="lens-calendar-nav-group">
                  {paneIndex === paneMonths.length - 1 && (
                    <button
                      aria-label={translate('calendar.nextMonth', 'Next month')}
                      className="lens-calendar-nav"
                      onClick={() => monthNav(1)}
                      type="button"
                    >
                      <CaretRight size={12} />
                    </button>
                  )}
                </span>
              </div>
              <div
                aria-label={`${translate('calendar.label', 'Calendar')}, ${heading}`}
                className="lens-calendar-grid"
                role="grid"
              >
                <div className="lens-calendar-weekdays" role="row">
                  {weekdays.map((label) => (
                    <span className="lens-calendar-weekday" key={label} role="columnheader">{label}</span>
                  ))}
                </div>
                {weeks.map((week) => (
                  <div className="lens-calendar-week" key={formatISODate(week[0]!.date)} role="row">
                    {week.map((cell, cellIndex) => {
                      if (!cell.inMonth) {
                        // Padding day of an adjacent month: decorative only.
                        // It is not a gridcell, takes no band wash, and (in
                        // the dual-pane window) would otherwise duplicate the
                        // accessible name of the neighbour pane's real cell.
                        return (
                          <span
                            aria-hidden="true"
                            className="lens-calendar-day"
                            data-outside="true"
                            key={formatISODate(cell.date)}
                          >
                            <span className="lens-calendar-day-label">{cell.date.day}</span>
                          </span>
                        )
                      }
                      const state = rangeDayState(cell.date, draft, hover)
                      const washed = state === 'inRange' || state === 'preview'
                      const isFocusCell = paneHasFocus
                        ? sameDate(cell.date, focused)
                        : sameDate(cell.date, fallbackTab)
                      const cellDisabled = disabled(cell.date)
                      return (
                        <button
                          aria-disabled={cellDisabled || undefined}
                          aria-label={dayLabel(locale, cell.date)}
                          aria-selected={state === 'start' || state === 'end' || state === 'inRange'}
                          className="lens-calendar-day"
                          data-band={bandSide(cell.date)}
                          data-cap={washed ? washCap(week, cellIndex) : undefined}
                          data-focused={isFocusCell || undefined}
                          data-state={state === 'none' ? undefined : state}
                          data-today={sameDate(cell.date, resolvedToday) ? true : undefined}
                          disabled={cellDisabled}
                          key={formatISODate(cell.date)}
                          onClick={() => pick(cell.date)}
                          onFocus={() => setFocused(cell.date)}
                          onMouseEnter={() => setHover(cellDisabled ? undefined : cell.date)}
                          role="gridcell"
                          tabIndex={isFocusCell ? 0 : -1}
                          type="button"
                        >
                          <span className="lens-calendar-day-label">{cell.date.day}</span>
                        </button>
                      )
                    })}
                  </div>
                ))}
              </div>
            </div>
          )
        })}
      </div>
      <div aria-live="polite" className="lens-visually-hidden" role="status">{announcement}</div>
    </div>
  )
}
