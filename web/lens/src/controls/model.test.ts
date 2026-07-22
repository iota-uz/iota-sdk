import { describe, expect, it } from 'vitest'
import {
  addDays,
  addMonths,
  canonicalCalendarLocale,
  clampDate,
  dayLabel,
  dayOfWeek,
  daysInMonth,
  firstDayOfWeek,
  formatISODate,
  keyboardTarget,
  monthGrid,
  monthLabel,
  parseISODate,
  previewRange,
  rangeDayState,
  resolvePreset,
  selectDay,
  weekdayLabels,
  type CalendarDate,
  type PeriodPresetId,
} from './model'

const date = (year: number, month: number, day: number): CalendarDate => ({ year, month, day })

describe('ISO date parsing', () => {
  it('round-trips wire dates', () => {
    for (const raw of ['2026-01-01', '2026-12-31', '2024-02-29', '1970-01-01']) {
      const parsed = parseISODate(raw)
      expect(parsed).toBeDefined()
      expect(formatISODate(parsed!)).toBe(raw)
    }
  })

  it('rejects malformed and impossible dates', () => {
    for (const raw of ['2026-2-1', '01.02.2026', '2026-13-01', '2026-00-10', '2025-02-29', '2026-04-31', '', 'now']) {
      expect(parseISODate(raw)).toBeUndefined()
    }
  })
})

describe('date arithmetic', () => {
  it('knows month lengths and leap years', () => {
    expect(daysInMonth(2024, 2)).toBe(29)
    expect(daysInMonth(2025, 2)).toBe(28)
    expect(daysInMonth(2026, 7)).toBe(31)
    expect(daysInMonth(2100, 2)).toBe(28)
    expect(daysInMonth(2000, 2)).toBe(29)
  })

  it('adds days across month and year boundaries', () => {
    expect(addDays(date(2026, 12, 31), 1)).toEqual(date(2027, 1, 1))
    expect(addDays(date(2026, 3, 1), -1)).toEqual(date(2026, 2, 28))
    expect(addDays(date(2024, 2, 28), 1)).toEqual(date(2024, 2, 29))
  })

  it('adds months and clamps the day', () => {
    expect(addMonths(date(2026, 1, 31), 1)).toEqual(date(2026, 2, 28))
    expect(addMonths(date(2026, 12, 15), 1)).toEqual(date(2027, 1, 15))
    expect(addMonths(date(2026, 1, 15), -1)).toEqual(date(2025, 12, 15))
    expect(addMonths(date(2024, 3, 31), -1)).toEqual(date(2024, 2, 29))
  })

  it('computes ISO day of week', () => {
    expect(dayOfWeek(date(2024, 1, 1))).toBe(1) // Monday
    expect(dayOfWeek(date(2026, 7, 19))).toBe(7) // Sunday
    expect(dayOfWeek(date(2026, 7, 22))).toBe(3) // Wednesday
  })

  it('clamps into bounds', () => {
    const min = date(2026, 1, 1)
    const max = date(2026, 12, 31)
    expect(clampDate(date(2025, 6, 1), min, max)).toEqual(min)
    expect(clampDate(date(2027, 6, 1), min, max)).toEqual(max)
    expect(clampDate(date(2026, 6, 1), min, max)).toEqual(date(2026, 6, 1))
  })
})

describe('month grid', () => {
  it('pads boundary days from adjacent months, Monday-first', () => {
    const weeks = monthGrid(2026, 7, 1) // July 2026 starts on Wednesday
    expect(weeks).toHaveLength(5)
    expect(weeks[0]![0]!.date).toEqual(date(2026, 6, 29))
    expect(weeks[0]![0]!.inMonth).toBe(false)
    expect(weeks[0]![2]!.date).toEqual(date(2026, 7, 1))
    expect(weeks[0]![2]!.inMonth).toBe(true)
    expect(weeks[4]![6]!.date).toEqual(date(2026, 8, 2))
    expect(weeks[4]![6]!.inMonth).toBe(false)
    for (const week of weeks) expect(week).toHaveLength(7)
  })

  it('respects a Sunday-first week', () => {
    const weeks = monthGrid(2026, 7, 7)
    expect(weeks[0]![0]!.date).toEqual(date(2026, 6, 28)) // Sunday
    expect(weeks[0]![3]!.date).toEqual(date(2026, 7, 1))
  })

  it('covers a month starting exactly on the first weekday', () => {
    // June 2026 starts on Monday.
    const weeks = monthGrid(2026, 6, 1)
    expect(weeks[0]![0]!.date).toEqual(date(2026, 6, 1))
    expect(weeks[0]![0]!.inMonth).toBe(true)
  })
})

describe('keyboard navigation', () => {
  const origin = date(2026, 7, 22)

  it('moves by day, week and month', () => {
    expect(keyboardTarget(origin, 'ArrowLeft', 1)).toEqual(date(2026, 7, 21))
    expect(keyboardTarget(origin, 'ArrowRight', 1)).toEqual(date(2026, 7, 23))
    expect(keyboardTarget(origin, 'ArrowUp', 1)).toEqual(date(2026, 7, 15))
    expect(keyboardTarget(origin, 'ArrowDown', 1)).toEqual(date(2026, 7, 29))
    expect(keyboardTarget(origin, 'PageUp', 1)).toEqual(date(2026, 6, 22))
    expect(keyboardTarget(origin, 'PageDown', 1)).toEqual(date(2026, 8, 22))
  })

  it('crosses month boundaries by arrow', () => {
    expect(keyboardTarget(date(2026, 7, 1), 'ArrowLeft', 1)).toEqual(date(2026, 6, 30))
    expect(keyboardTarget(date(2026, 7, 31), 'ArrowRight', 1)).toEqual(date(2026, 8, 1))
  })

  it('jumps to week bounds honoring first day of week', () => {
    // 2026-07-22 is Wednesday.
    expect(keyboardTarget(origin, 'Home', 1)).toEqual(date(2026, 7, 20)) // Monday
    expect(keyboardTarget(origin, 'End', 1)).toEqual(date(2026, 7, 26)) // Sunday
    expect(keyboardTarget(origin, 'Home', 7)).toEqual(date(2026, 7, 19)) // Sunday-first
    expect(keyboardTarget(origin, 'End', 7)).toEqual(date(2026, 7, 25)) // Saturday
  })
})

describe('range selection machine', () => {
  it('anchors, completes and restarts', () => {
    const first = selectDay({}, date(2026, 7, 3))
    expect(first.complete).toBeUndefined()
    expect(first.draft).toEqual({ start: date(2026, 7, 3) })

    const second = selectDay(first.draft, date(2026, 7, 10))
    expect(second.complete).toEqual({ start: date(2026, 7, 3), end: date(2026, 7, 10) })

    const restart = selectDay(second.draft, date(2026, 7, 5))
    expect(restart.complete).toBeUndefined()
    expect(restart.draft).toEqual({ start: date(2026, 7, 5) })
  })

  it('swaps when the second pick precedes the anchor', () => {
    const anchored = selectDay({}, date(2026, 7, 10))
    const completed = selectDay(anchored.draft, date(2026, 7, 3))
    expect(completed.complete).toEqual({ start: date(2026, 7, 3), end: date(2026, 7, 10) })
  })

  it('completes a single-day range', () => {
    const anchored = selectDay({}, date(2026, 7, 10))
    const completed = selectDay(anchored.draft, date(2026, 7, 10))
    expect(completed.complete).toEqual({ start: date(2026, 7, 10), end: date(2026, 7, 10) })
  })
})

describe('hover preview', () => {
  it('previews only while the second boundary is pending', () => {
    expect(previewRange({}, date(2026, 7, 5))).toBeUndefined()
    expect(previewRange({ start: date(2026, 7, 3), end: date(2026, 7, 9) }, date(2026, 7, 5))).toBeUndefined()
    expect(previewRange({ start: date(2026, 7, 3) }, date(2026, 7, 5)))
      .toEqual({ start: date(2026, 7, 3), end: date(2026, 7, 5) })
    expect(previewRange({ start: date(2026, 7, 3) }, date(2026, 7, 1)))
      .toEqual({ start: date(2026, 7, 1), end: date(2026, 7, 3) })
  })

  it('classifies day states for rendering', () => {
    const draft = { start: date(2026, 7, 3), end: date(2026, 7, 9) }
    expect(rangeDayState(date(2026, 7, 3), draft, undefined)).toBe('start')
    expect(rangeDayState(date(2026, 7, 9), draft, undefined)).toBe('end')
    expect(rangeDayState(date(2026, 7, 5), draft, undefined)).toBe('inRange')
    expect(rangeDayState(date(2026, 7, 10), draft, undefined)).toBe('none')

    const pending = { start: date(2026, 7, 3) }
    expect(rangeDayState(date(2026, 7, 4), pending, date(2026, 7, 6))).toBe('preview')
    expect(rangeDayState(date(2026, 7, 6), pending, date(2026, 7, 6))).toBe('previewEdge')
    expect(rangeDayState(date(2026, 7, 7), pending, date(2026, 7, 6))).toBe('none')
  })
})

describe('period presets', () => {
  // A Wednesday in Q3, mid-month, mid-year — exercises every branch.
  const today = date(2026, 7, 22)

  const bounds = (id: PeriodPresetId, from: CalendarDate) => resolvePreset(id, from)

  it('resolves today-relative presets to the legacy pickers\' inclusive bounds', () => {
    expect(bounds('today', today)).toEqual({ start: date(2026, 7, 22), end: date(2026, 7, 22) })
    expect(bounds('yesterday', today)).toEqual({ start: date(2026, 7, 21), end: date(2026, 7, 21) })
    // Monday-first week (legacy generic picker anchor).
    expect(bounds('thisWeek', today)).toEqual({ start: date(2026, 7, 20), end: date(2026, 7, 26) })
    expect(bounds('lastWeek', today)).toEqual({ start: date(2026, 7, 13), end: date(2026, 7, 19) })
    expect(bounds('thisMonth', today)).toEqual({ start: date(2026, 7, 1), end: date(2026, 7, 22) })
    expect(bounds('lastMonth', today)).toEqual({ start: date(2026, 6, 1), end: date(2026, 6, 30) })
    expect(bounds('last30days', today)).toEqual({ start: date(2026, 6, 23), end: date(2026, 7, 22) })
    expect(bounds('last12months', today)).toEqual({ start: date(2025, 7, 22), end: date(2026, 7, 22) })
    expect(bounds('thisQuarter', today)).toEqual({ start: date(2026, 7, 1), end: date(2026, 7, 22) })
    expect(bounds('yearToDate', today)).toEqual({ start: date(2026, 1, 1), end: date(2026, 7, 22) })
    expect(bounds('thisYear', today)).toEqual({ start: date(2026, 1, 1), end: date(2026, 12, 31) })
    expect(bounds('lastYear', today)).toEqual({ start: date(2025, 1, 1), end: date(2025, 12, 31) })
  })

  it('treats all-time as unbounded', () => {
    expect(bounds('allTime', today)).toBeUndefined()
  })

  it('crosses the year boundary for last month in January', () => {
    expect(bounds('lastMonth', date(2026, 1, 10))).toEqual({ start: date(2025, 12, 1), end: date(2025, 12, 31) })
  })

  it('anchors the quarter to its first month', () => {
    expect(bounds('thisQuarter', date(2026, 5, 15))).toEqual({ start: date(2026, 4, 1), end: date(2026, 5, 15) })
    expect(bounds('thisQuarter', date(2026, 1, 1))).toEqual({ start: date(2026, 1, 1), end: date(2026, 1, 1) })
    expect(bounds('thisQuarter', date(2026, 12, 31))).toEqual({ start: date(2026, 10, 1), end: date(2026, 12, 31) })
  })

  it('clamps last-12-months across a month-overflow origin', () => {
    // 2024 is a leap year: July→prior-July stays on the day, but a 31st origin
    // clamps rather than rolling forward.
    expect(bounds('last12months', date(2026, 3, 31))).toEqual({ start: date(2025, 3, 31), end: date(2026, 3, 31) })
  })
})

describe('locale data', () => {
  it('aliases Granite oz to Cyrillic Uzbek', () => {
    expect(canonicalCalendarLocale('oz')).toBe('uz-Cyrl')
    expect(canonicalCalendarLocale('uz-Cyrl')).toBe('uz-Cyrl')
    expect(canonicalCalendarLocale('ru')).toBe('ru')
    expect(canonicalCalendarLocale('')).toBe('en')
  })

  it('derives first day of week from locale data', () => {
    expect(firstDayOfWeek('ru')).toBe(1)
    expect(firstDayOfWeek('uz')).toBe(1)
    expect(firstDayOfWeek('uz-Cyrl')).toBe(1)
    expect(firstDayOfWeek('oz')).toBe(1)
    expect(firstDayOfWeek('en-US')).toBe(7)
  })

  it('labels months per locale', () => {
    expect(monthLabel('en', 2026, 7).toLowerCase()).toContain('july')
    expect(monthLabel('ru', 2026, 7).toLowerCase()).toContain('июл')
    expect(monthLabel('uz', 2026, 7).toLowerCase()).toContain('iyul')
    expect(monthLabel('oz', 2026, 7).toLowerCase()).toContain('июл')
  })

  it('rotates weekday labels to the first day of week', () => {
    const monday = weekdayLabels('en', 1)
    const sunday = weekdayLabels('en', 7)
    expect(monday).toHaveLength(7)
    expect(monday[0]).toBe('Mon')
    expect(sunday[0]).toBe('Sun')
    expect(sunday[1]).toBe('Mon')
    expect(weekdayLabels('ru', 1)[0]!.toLowerCase()).toContain('пн')
  })

  it('formats day labels for announcements', () => {
    expect(dayLabel('en', date(2026, 7, 22))).toContain('2026')
    expect(dayLabel('ru', date(2026, 7, 22))).toContain('июл')
  })
})
