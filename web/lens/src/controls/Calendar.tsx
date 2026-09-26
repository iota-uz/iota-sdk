/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { createEffect, createSignal, createMemo, For, onCleanup, Show } from 'solid-js'
import { stackedCalendarMediaQuery } from '../breakpoints'
import { CaretLeft, CaretRight } from '../icons'
import type { TranslationVars } from '../runtime'
import {
  addMonths,
  clampDate,
  compareDates,
  dayLabel,
  daysInMonth,
  firstDayOfWeek,
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

function useNarrow(): () => boolean {
  const [narrow, setNarrow] = createSignal(
    globalThis.window?.matchMedia?.(stackedCalendarMediaQuery).matches ?? false,
  )
  createEffect(() => {
    const media = globalThis.window?.matchMedia?.(stackedCalendarMediaQuery)
    if (!media?.addEventListener) return
    const onChange = (event: MediaQueryListEvent) => setNarrow(event.matches)
    media.addEventListener('change', onChange)
    onCleanup(() => media.removeEventListener('change', onChange))
  })
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
export function Calendar(props: CalendarProps) {
  const firstDay = firstDayOfWeek(props.locale)
  const narrow = useNarrow()
  const paneCount = () => narrow() ? 1 : 2
  const resolvedToday = () => props.today ?? localToday()
  const initialFocus = clampDate(props.draft.start ?? resolvedToday(), props.min, props.max)
  const [focused, setFocused] = createSignal<CalendarDate>(initialFocus)
  const [visibleMonth, setVisibleMonth] = createSignal<CalendarDate>(startOfMonth(initialFocus))
  const [hover, setHover] = createSignal<CalendarDate>()
  const [announcement, setAnnouncement] = createSignal('')
  const [view, setView] = createSignal<CalendarView>('days')
  const [panelYear, setPanelYear] = createSignal(initialFocus.year)
  let panesRef: HTMLDivElement | undefined
  let focusPending = false

  const paneMonths = () => Array.from({ length: paneCount() }, (_, index) => addMonths(visibleMonth(), index))
  const inWindow = (date: CalendarDate) => (
    Array.from({ length: paneCount() }, (_, index) => addMonths(visibleMonth(), index))
      .some((month) => inMonth(date, month))
  )

  const disabled = (date: CalendarDate) => (
    (props.min !== undefined && compareDates(date, props.min) < 0) ||
    (props.max !== undefined && compareDates(date, props.max) > 0)
  )

  /** Shifts the window so `month` is visible, announcing the month shown. */
  const showMonth = (month: CalendarDate, announce = true) => {
    const target = startOfMonth(month)
    setVisibleMonth((current) => {
      if (compareDates(target, current) < 0) return target
      // Months after the window land in the last pane.
      return addMonths(target, -(paneCount() - 1))
    })
    if (announce) setAnnouncement(monthLabel(props.locale, month.year, month.month))
  }

  const moveFocus = (date: CalendarDate) => {
    const target = clampDate(date, props.min, props.max)
    setFocused(target)
    focusPending = true
    if (!inWindow(target)) showMonth(target)
  }

  // Focus follows the roving cell after keyboard movement, once the cell for
  // the (possibly new) month exists in the DOM.
  createEffect(() => {
    void focused()
    void visibleMonth()
    void paneCount()
    if (!focusPending) return
    focusPending = false
    const cell = panesRef?.querySelector<HTMLElement>('[data-focused="true"]')
    cell?.focus()
  })

  const pick = (date: CalendarDate) => {
    if (disabled(date)) return
    const selection = selectDay(props.draft, date)
    setHover(undefined)
    setFocused(date)
    if (selection.complete) {
      setAnnouncement(props.translate('calendar.announceRange', 'Selected {start} to {end}', {
        start: dayLabel(props.locale, selection.complete.start),
        end: dayLabel(props.locale, selection.complete.end),
      }))
    } else if (selection.draft.start) {
      setAnnouncement(props.translate('calendar.announceStart', '{date} chosen as range start', {
        date: dayLabel(props.locale, selection.draft.start),
      }))
    }
    props.onPick(selection)
  }

  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      pick(focused())
      return
    }
    if (!(navigationKeys as ReadonlyArray<string>).includes(event.key)) return
    event.preventDefault()
    moveFocus(keyboardTarget(focused(), event.key as CalendarKey, firstDay))
  }

  // The continuous range band. A committed draft owns the band; while a range
  // is in progress the hover preview does. Endpoint cells carry the side the
  // soft wash must extend toward so the band reads as one unbroken strip that
  // rounds off exactly at the outer edges of the endpoint pills. In-range
  // cells that touch a week-row edge (or an out-of-month gap) carry a cap so
  // the wash rounds off instead of bleeding to the grid border.
  const committed = () => props.draft.start && props.draft.end && compareDates(props.draft.start, props.draft.end) < 0
    ? { start: props.draft.start, end: props.draft.end }
    : undefined
  const band = () => committed() ?? previewRange(props.draft, hover())
  const bandTone = () => committed() ? '' : '-preview'
  const bandSide = (date: CalendarDate): string | undefined => {
    const current = band()
    if (!current || sameDate(current.start, current.end)) return undefined
    if (sameDate(date, current.start)) return `right${bandTone()}`
    if (sameDate(date, current.end)) return `left${bandTone()}`
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

  const weekdays = weekdayLabels(props.locale, firstDay)
  const focusWindow = () => inWindow(focused())

  const monthNav = (offset: number) => {
    const month = addMonths(visibleMonth(), offset)
    setVisibleMonth(month)
    setAnnouncement(monthLabel(props.locale, month.year, month.month))
    setFocused((current) => clampDate(addMonths(current, offset), props.min, props.max))
  }

  const openMonthPanel = (year: number) => {
    setPanelYear(year)
    setView('months')
  }

  /** A whole month is unreachable when it falls entirely outside min/max. */
  const monthDisabled = (year: number, month: number) => (
    (props.min !== undefined && compareDates({ year, month, day: daysInMonth(year, month) }, props.min) < 0) ||
    (props.max !== undefined && compareDates({ year, month, day: 1 }, props.max) > 0)
  )

  const yearDisabled = (year: number) => (
    (props.min !== undefined && year < props.min.year) || (props.max !== undefined && year > props.max.year)
  )

  const pickMonth = (month: number) => {
    const target = { year: panelYear(), month, day: 1 }
    setVisibleMonth(target)
    setFocused((current) => clampDate({ ...target, day: Math.min(current.day, daysInMonth(panelYear(), month)) }, props.min, props.max))
    setAnnouncement(monthLabel(props.locale, panelYear(), month))
    setView('days')
  }

  const jumpHeading = createMemo(() => {
    void view()
    const years = yearBlock(panelYear())
    const monthsView = view() === 'months'
    return monthsView ? String(panelYear()) : `${years[0]} – ${years[years.length - 1]}`
  })
  const jumpStep = createMemo(() => view() === 'months' ? 1 : 12)
  const monthNames = monthShortLabels(props.locale)

  // The jump panel takes the day grid's place: months for one year, or the
  // twelve-year block that year sits in. It is the only way to travel far,
  // which is why the heading that opens it is a button in every pane.
  return (
    <Show
      when={view() === 'days'}
      fallback={
        <div class="lens-calendar" data-panes={paneCount()} data-view={view()}>
          <div class="lens-calendar-jump">
            <div class="lens-calendar-header">
              <span class="lens-calendar-nav-group">
                <button
                  aria-label={props.translate('calendar.prevPage', 'Previous')}
                  class="lens-calendar-nav"
                  onClick={() => setPanelYear((current) => current - jumpStep())}
                  type="button"
                >
                  <CaretLeft size={12} />
                </button>
              </span>
              <button
                class="lens-calendar-month"
                onClick={() => setView(view() === 'months' ? 'years' : 'months')}
                type="button"
              >
                {jumpHeading()}
              </button>
              <span class="lens-calendar-nav-group">
                <button
                  aria-label={props.translate('calendar.nextPage', 'Next')}
                  class="lens-calendar-nav"
                  onClick={() => setPanelYear((current) => current + jumpStep())}
                  type="button"
                >
                  <CaretRight size={12} />
                </button>
              </span>
            </div>
            <div class="lens-calendar-jump-grid">
              <Show
                when={view() === 'months'}
                fallback={
                  <For each={yearBlock(panelYear())}>
                    {(year) => (
                      <button
                        aria-pressed={year === visibleMonth().year}
                        class="lens-calendar-jump-cell"
                        disabled={yearDisabled(year)}
                        onClick={() => { setPanelYear(year); setView('months') }}
                        type="button"
                      >
                        {year}
                      </button>
                    )}
                  </For>
                }
              >
                <For each={monthNames}>
                  {(name, index) => (
                    <button
                      aria-pressed={panelYear() === visibleMonth().year && index() + 1 === visibleMonth().month}
                      class="lens-calendar-jump-cell"
                      disabled={monthDisabled(panelYear(), index() + 1)}
                      onClick={() => pickMonth(index() + 1)}
                      type="button"
                    >
                      {name}
                    </button>
                  )}
                </For>
              </Show>
            </div>
          </div>
          <div aria-live="polite" class="lens-visually-hidden" role="status">{announcement()}</div>
        </div>
      }
    >
      <div class="lens-calendar" data-panes={paneCount()} data-view="days">
        {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions -- keyboard navigation is delegated from focusable grid cells; this wrapper must not become another tab stop. */}
        <div
          class="lens-calendar-panes"
          onKeyDown={onKeyDown}
          onMouseLeave={() => setHover(undefined)}
          ref={(el) => { panesRef = el }}
        >
          <For each={paneMonths()}>
            {(month, paneIndex) => {
              const weeks = monthGrid(month.year, month.month, firstDay)
              const heading = monthLabel(props.locale, month.year, month.month)
              const paneHasFocus = () => inMonth(focused(), month)
              // The roving cell: the focused date when visible, otherwise the
              // first selectable day of the leading pane.
              const fallbackTab = () => !focusWindow() && paneIndex() === 0
                ? weeks.flat().find((cell) => cell.inMonth && !disabled(cell.date))?.date
                : undefined
              return (
                <div class="lens-calendar-pane">
                  <div class="lens-calendar-header">
                    <span class="lens-calendar-nav-group">
                      {paneIndex() === 0 && (
                        <button
                          aria-label={props.translate('calendar.prevMonth', 'Previous month')}
                          class="lens-calendar-nav"
                          onClick={() => monthNav(-1)}
                          type="button"
                        >
                          <CaretLeft size={12} />
                        </button>
                      )}
                    </span>
                    <button
                      aria-label={`${props.translate('calendar.chooseMonth', 'Choose month')}: ${heading}`}
                      class="lens-calendar-month"
                      onClick={() => openMonthPanel(month.year)}
                      type="button"
                    >
                      {heading}
                    </button>
                    <span class="lens-calendar-nav-group">
                      {paneIndex() === paneMonths().length - 1 && (
                        <button
                          aria-label={props.translate('calendar.nextMonth', 'Next month')}
                          class="lens-calendar-nav"
                          onClick={() => monthNav(1)}
                          type="button"
                        >
                          <CaretRight size={12} />
                        </button>
                      )}
                    </span>
                  </div>
                  <div
                    aria-label={`${props.translate('calendar.label', 'Calendar')}, ${heading}`}
                    class="lens-calendar-grid"
                    role="grid"
                  >
                    <div class="lens-calendar-weekdays" role="row">
                      <For each={weekdays}>
                        {(label) => <span class="lens-calendar-weekday" role="columnheader">{label}</span>}
                      </For>
                    </div>
                    <For each={weeks}>
                      {(week) => (
                        <div class="lens-calendar-week" role="row">
                          <For each={week}>
                            {(cell, cellIndex) => {
                              if (!cell.inMonth) {
                                // Padding day of an adjacent month: decorative only.
                                // It is not a gridcell, takes no band wash, and (in
                                // the dual-pane window) would otherwise duplicate the
                                // accessible name of the neighbour pane's real cell.
                                return (
                                  <span
                                    aria-hidden="true"
                                    class="lens-calendar-day"
                                    data-outside="true"
                                  >
                                    <span class="lens-calendar-day-label">{cell.date.day}</span>
                                  </span>
                                )
                              }
                              const state = () => rangeDayState(cell.date, props.draft, hover())
                              const washed = () => state() === 'inRange' || state() === 'preview'
                              const isFocusCell = () => paneHasFocus()
                                ? sameDate(cell.date, focused())
                                : sameDate(cell.date, fallbackTab())
                              const cellDisabled = disabled(cell.date)
                              return (
                                <button
                                  aria-disabled={cellDisabled || undefined}
                                  aria-label={dayLabel(props.locale, cell.date)}
                                  aria-selected={state() === 'start' || state() === 'end' || state() === 'inRange'}
                                  class="lens-calendar-day"
                                  data-band={bandSide(cell.date)}
                                  data-cap={washed() ? washCap(week, cellIndex()) : undefined}
                                  data-focused={isFocusCell() || undefined}
                                  data-state={state() === 'none' ? undefined : state()}
                                  data-today={sameDate(cell.date, resolvedToday()) ? true : undefined}
                                  disabled={cellDisabled}
                                  onClick={() => pick(cell.date)}
                                  onFocus={() => setFocused(cell.date)}
                                  onMouseEnter={() => setHover(cellDisabled ? undefined : cell.date)}
                                  role="gridcell"
                                  tabIndex={isFocusCell() ? 0 : -1}
                                  type="button"
                                >
                                  <span class="lens-calendar-day-label">{cell.date.day}</span>
                                </button>
                              )
                            }}
                          </For>
                        </div>
                      )}
                    </For>
                  </div>
                </div>
              )
            }}
          </For>
        </div>
        <div aria-live="polite" class="lens-visually-hidden" role="status">{announcement()}</div>
      </div>
    </Show>
  )
}
