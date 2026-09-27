import { describe, expect, it } from 'vitest'
import type { FieldFormat } from '../contract'
import { columnUnit } from './units'

/** Intl separates groups and magnitudes with non-breaking spaces. */
const plain = (value: string) => value.replace(/[\u00A0\u202F]/g, ' ')

const money: FieldFormat = { kind: 'money', currency: 'UZS', minorUnits: false, compact: true, precision: 2 }

describe('columnUnit', () => {
  it('states one magnitude for a column whose rows are comparable', () => {
    const unit = columnUnit([4.0e9, 3.12e8, 1.11e8], money, 'ru')

    expect(unit.note).toBe('млрд UZS')
    expect(plain(unit.format(4.0e9))).toBe('4,00')
    expect(plain(unit.format(3.12e8))).toBe('0,31')
  })

  it('keeps per-row magnitudes when the column spans a long tail', () => {
    // 4.21 млрд next to 3 116: a shared scale would print the tail as 0,00.
    const unit = columnUnit([4.21e9, 3116], money, 'ru')

    expect(unit.note).toBe('UZS')
    expect(plain(unit.format(4.21e9))).toBe('4,21 млрд')
    expect(plain(unit.format(3116))).toBe('3 116')
  })

  it('hoists the currency even when nothing can be normalised', () => {
    const unit = columnUnit([4.21e9, 3116], money, 'ru')

    expect(unit.format(4.21e9)).not.toContain('UZS')
  })

  it('reads minor units before deciding the magnitude', () => {
    const tiyin: FieldFormat = { ...money, minorUnits: true }
    const unit = columnUnit([4.0e11, 3.0e11], tiyin, 'ru')

    expect(unit.note).toBe('млрд UZS')
    expect(plain(unit.format(4.0e11))).toBe('4,00')
  })

  it('leaves a percentage column alone', () => {
    const percent: FieldFormat = { kind: 'percent', minorUnits: false, precision: 1 }
    const unit = columnUnit([95.1, 0.1], percent, 'ru')

    expect(unit.note).toBeUndefined()
    expect(plain(unit.format(95.1))).toContain('95,1')
  })

  it('leaves a column of counts alone', () => {
    const count: FieldFormat = { kind: 'number', minorUnits: false }
    const unit = columnUnit([435, 23, 1], count, 'ru')

    expect(unit.note).toBeUndefined()
    expect(unit.format(435)).toBe('435')
  })

  it('ignores zeroes when judging the spread', () => {
    const unit = columnUnit([4.0e9, 0, 3.12e8], money, 'ru')

    expect(unit.note).toBe('млрд UZS')
  })

  it('formats a non-numeric cell the way the field would', () => {
    const unit = columnUnit([4.0e9, 3.12e8], money, 'ru')

    expect(unit.format(null)).toBe('—')
  })
})
