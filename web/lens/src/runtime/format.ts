import type { FieldFormat } from '../contract'

function numeric(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

function fallback(value: unknown): string {
  if (value === null || value === undefined) return '—'
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return String(value)
  return '—'
}

function precisionOptions(precision: number | undefined): Intl.NumberFormatOptions {
  if (precision === undefined) return {}
  return { minimumFractionDigits: precision, maximumFractionDigits: precision }
}

function formatMoney(value: number, field: FieldFormat, locale: string): string {
  const currency = field.currency ?? 'USD'
  const base = new Intl.NumberFormat(locale, { style: 'currency', currency, ...precisionOptions(field.precision) })
  if (!field.minorUnits) return base.format(value)
  const fractionDigits = new Intl.NumberFormat(locale, { style: 'currency', currency })
    .resolvedOptions().maximumFractionDigits ?? 0
  return base.format(value / 10 ** fractionDigits)
}

const goDateTokens = [
  'January', 'Monday', 'Z07:00', '-07:00', '-0700', '2006', 'Jan', 'Mon', 'MST', 'PM', 'pm',
  '15', '03', '04', '05', '06', '01', '02', '1', '2', '3',
] as const

function datePart(date: Date, token: (typeof goDateTokens)[number], locale: string): string {
  const number = (value: number, width = 2) => String(value).padStart(width, '0')
  switch (token) {
    case '2006': return number(date.getUTCFullYear(), 4)
    case '06': return number(date.getUTCFullYear() % 100)
    case '01': return number(date.getUTCMonth() + 1)
    case '1': return String(date.getUTCMonth() + 1)
    case '02': return number(date.getUTCDate())
    case '2': return String(date.getUTCDate())
    case '15': return number(date.getUTCHours())
    case '03': return number(date.getUTCHours() % 12 || 12)
    case '3': return String(date.getUTCHours() % 12 || 12)
    case '04': return number(date.getUTCMinutes())
    case '05': return number(date.getUTCSeconds())
    case 'PM': return date.getUTCHours() < 12 ? 'AM' : 'PM'
    case 'pm': return date.getUTCHours() < 12 ? 'am' : 'pm'
    case 'January': return new Intl.DateTimeFormat(locale, { month: 'long', timeZone: 'UTC' }).format(date)
    case 'Jan': return new Intl.DateTimeFormat(locale, { month: 'short', timeZone: 'UTC' }).format(date)
    case 'Monday': return new Intl.DateTimeFormat(locale, { weekday: 'long', timeZone: 'UTC' }).format(date)
    case 'Mon': return new Intl.DateTimeFormat(locale, { weekday: 'short', timeZone: 'UTC' }).format(date)
    case 'MST': return 'UTC'
    case 'Z07:00':
    case '-07:00': return 'Z'
    case '-0700': return '+0000'
  }
}

function formatDate(value: unknown, layout: string | undefined, locale: string): string | undefined {
  const date = value instanceof Date ? value : new Date(typeof value === 'number' || typeof value === 'string' ? value : '')
  if (Number.isNaN(date.getTime())) return undefined
  const template = layout || '2006-01-02'
  let output = ''
  for (let index = 0; index < template.length;) {
    const token = goDateTokens.find((candidate) => template.startsWith(candidate, index))
    if (!token) {
      output += template[index]
      index += 1
      continue
    }
    output += datePart(date, token, locale)
    index += token.length
  }
  return output
}

export function formatFieldValue(value: unknown, field: FieldFormat | undefined, locale: string): string {
  if (!field) {
    const number = numeric(value)
    return number === undefined ? fallback(value) : new Intl.NumberFormat(locale).format(number)
  }
  if (field.kind === 'string') return fallback(value)
  if (field.kind === 'date') return formatDate(value, field.layout, locale) ?? fallback(value)
  const number = numeric(value)
  if (number === undefined) return fallback(value)
  if (field.kind === 'money') return formatMoney(number, field, locale)
  if (field.kind === 'percent') {
    return new Intl.NumberFormat(locale, {
      style: 'unit', unit: 'percent', unitDisplay: 'narrow', ...precisionOptions(field.precision),
    }).format(number)
  }
  return new Intl.NumberFormat(locale, precisionOptions(field.precision)).format(number)
}
