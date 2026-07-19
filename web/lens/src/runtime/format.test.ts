import { describe, expect, it } from 'vitest'
import type { FieldFormat } from '../contract'
import { formatFieldValue } from './format'

describe('formatFieldValue', () => {
  it.each<{ name: string; value: unknown; field?: FieldFormat; expected: string }>([
    { name: 'default number', value: 1234.5, expected: '1,234.5' },
    { name: 'precision', value: 12.345, field: { kind: 'number', minorUnits: false, precision: 2 }, expected: '12.35' },
    { name: 'major-unit money', value: 123.45, field: { kind: 'money', currency: 'USD', minorUnits: false, precision: 2 }, expected: '$123.45' },
    { name: 'minor-unit money', value: 12345, field: { kind: 'money', currency: 'USD', minorUnits: true, precision: 2 }, expected: '$123.45' },
    { name: 'minor-unit scale ignores display precision', value: 12345, field: { kind: 'money', currency: 'USD', minorUnits: true, precision: 0 }, expected: '$123' },
    { name: 'percent is already percentage points', value: 7.5, field: { kind: 'percent', minorUnits: false, precision: 1 }, expected: '7.5%' },
    { name: 'Go date layout', value: '2026-07-19T09:30:00Z', field: { kind: 'date', minorUnits: false, layout: '02 Jan 2006 15:04' }, expected: '19 Jul 2026 09:30' },
    { name: 'string', value: 'ready', field: { kind: 'string', minorUnits: false }, expected: 'ready' },
    { name: 'null', value: null, field: { kind: 'string', minorUnits: false }, expected: '—' },
  ])('$name', ({ value, field, expected }) => {
    expect(formatFieldValue(value, field, 'en-US')).toBe(expected)
  })
})
