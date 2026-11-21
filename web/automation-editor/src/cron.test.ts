import { describe, expect, it } from 'vitest'
import {
  describeSchedule, detectPreset, durationMs, formatDuration, isDuration, isValidCron, nextRuns, parseCron, presetCron, presets, timeZones,
} from './cron'

const iso = (dates: Date[]) => dates.map((d) => d.toISOString())

describe('parseCron', () => {
  it('parses lists, ranges, steps and names', () => {
    const r = parseCron('*/15 9-17 1,15 JAN-MAR mon-fri')
    expect(r.ok).toBe(true)
    if (!r.ok) return
    expect(r.fields.minute).toEqual([0, 15, 30, 45])
    expect(r.fields.hour).toEqual([9, 10, 11, 12, 13, 14, 15, 16, 17])
    expect(r.fields.day).toEqual([1, 15])
    expect(r.fields.month).toEqual([1, 2, 3])
    expect(r.fields.weekday).toEqual([1, 2, 3, 4, 5])
    expect(r.fields.dayStar).toBe(false)
  })
  it('folds weekday 7 into Sunday', () => {
    const r = parseCron('0 0 * * 7')
    expect(r.ok && r.fields.weekday).toEqual([0])
  })
  it('reports errors with field and code', () => {
    expect(parseCron('0 0 * *')).toEqual({ ok: false, error: { code: 'fields', count: 4 } })
    expect(parseCron('60 0 * * *')).toMatchObject({ ok: false, error: { code: 'range', field: 'minute', value: '60' } })
    expect(parseCron('0 x * * *')).toMatchObject({ ok: false, error: { code: 'value', field: 'hour' } })
    expect(parseCron('0 0 */0 * *')).toMatchObject({ ok: false, error: { code: 'value', field: 'day' } })
    expect(parseCron('0 0 5-1 * *')).toMatchObject({ ok: false, error: { code: 'value', field: 'day' } })
    expect(isValidCron('0 9 * * 1-5')).toBe(true)
  })
})

describe('presets', () => {
  it('round-trip through detectPreset', () => {
    expect(detectPreset(presets.daily({ hour: 20, minute: 0 }))).toEqual({ kind: 'daily', hour: 20, minute: 0 })
    expect(detectPreset(presets.weekdays({ hour: 8, minute: 30 }))).toEqual({ kind: 'weekdays', hour: 8, minute: 30 })
    expect(detectPreset(presets.weekly({ hour: 9, minute: 5, weekday: 0 }))).toEqual({ kind: 'weekly', hour: 9, minute: 5, weekday: 0 })
    expect(detectPreset(presets.monthly({ hour: 7, minute: 0, day: 31 }))).toEqual({ kind: 'monthly', hour: 7, minute: 0, day: 31 })
    expect(detectPreset('*/5 * * * *').kind).toBe('custom')
    expect(detectPreset('0 9 * 1 *').kind).toBe('custom')
    expect(presetCron({ kind: 'custom', hour: 0, minute: 0 })).toBeNull()
  })
})

describe('nextRuns', () => {
  const from = new Date('2026-11-21T10:30:00Z') // a Saturday

  it('daily in UTC', () => {
    expect(iso(nextRuns('0 20 * * *', 'UTC', from))).toEqual([
      '2026-11-21T20:00:00.000Z', '2026-11-22T20:00:00.000Z', '2026-11-23T20:00:00.000Z',
      '2026-11-24T20:00:00.000Z', '2026-11-25T20:00:00.000Z',
    ])
  })

  it('weekdays in Asia/Tashkent (UTC+5)', () => {
    expect(iso(nextRuns('0 9 * * 1-5', 'Asia/Tashkent', from))).toEqual([
      '2026-11-23T04:00:00.000Z', '2026-11-24T04:00:00.000Z', '2026-11-25T04:00:00.000Z',
      '2026-11-26T04:00:00.000Z', '2026-11-27T04:00:00.000Z',
    ])
  })

  it('follows DST in Europe/Berlin and skips the gap', () => {
    const march = new Date('2026-03-27T00:00:00Z')
    expect(iso(nextRuns('0 9 * * *', 'Europe/Berlin', march, 3))).toEqual([
      '2026-03-27T08:00:00.000Z', '2026-03-28T08:00:00.000Z', '2026-03-29T07:00:00.000Z',
    ])
    // 02:30 does not exist on 29 March 2026 in Berlin.
    const gap = iso(nextRuns('30 2 * * *', 'Europe/Berlin', march, 3))
    expect(gap).toEqual(['2026-03-27T01:30:00.000Z', '2026-03-28T01:30:00.000Z', '2026-03-30T00:30:00.000Z'])
  })

  it('applies the Vixie OR rule for day and weekday', () => {
    const runs = iso(nextRuns('0 0 1 * 1', 'UTC', new Date('2026-11-25T00:00:00Z'), 3))
    expect(runs).toEqual(['2026-11-30T00:00:00.000Z', '2026-12-01T00:00:00.000Z', '2026-12-07T00:00:00.000Z'])
  })

  it('terminates on impossible dates', () => {
    expect(nextRuns('0 0 30 2 *', 'UTC', from)).toEqual([])
    expect(nextRuns('bad', 'UTC', from)).toEqual([])
  })
})

describe('describeSchedule', () => {
  it('names schedules by meaning', () => {
    expect(describeSchedule('0 20 * * *', 'en')).toBe('Every day at 20:00')
    expect(describeSchedule('0 20 * * *', 'ru')).toBe('Каждый день в 20:00')
    expect(describeSchedule('30 8 * * 1-5', 'ru')).toBe('По будням в 08:30')
    expect(describeSchedule('0 9 * * 3', 'ru')).toBe('Каждую среду в 09:00')
    expect(describeSchedule('0 9 * * 1', 'en')).toBe('Every Monday at 09:00')
    expect(describeSchedule('0 9 5 * *', 'en')).toBe('Monthly on day 5 at 09:00')
    expect(describeSchedule('*/5 * * * *', 'en')).toBe('On schedule */5 * * * *')
    expect(describeSchedule('nope', 'uz')).toBe('Jadval')
  })
})

describe('durations', () => {
  it('reads the format of the graph', () => {
    expect(durationMs('30s')).toBe(30_000)
    expect(durationMs('2h')).toBe(7_200_000)
    expect(durationMs('30d')).toBe(30 * 86_400_000)
    expect(durationMs('31d')).toBeNull()
    expect(isDuration('0s')).toBe(false)
    expect(isDuration('1.5h')).toBe(false)
    expect(formatDuration(7_200_000)).toBe('2h')
    expect(formatDuration(90_000)).toBe('90s')
  })
})

it('lists time zones with UTC first', () => {
  const zones = timeZones()
  expect(zones[0]).toBe('UTC')
  expect(zones).toContain('Asia/Tashkent')
})
