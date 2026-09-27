import { useEffect, useRef, useState } from 'react'
import { isVisualRegression } from '../visualRegression'

/** Duration of the count-up when a stat value changes. */
const tickerDurationMs = 300

interface ParsedValue {
  prefix: string
  suffix: string
  value: number
  decimals: number
  groupSep: string
  decimalSep: string
}

// Space-like grouping separators: ASCII space, non-breaking and narrow no-break.
const groupSpaces = /[\s\u00A0\u202F]/

function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined'
    && typeof window.matchMedia === 'function'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/**
 * Split a pre-formatted locale string ("1 250 000 UZS", "42,5%", "-3.4%") into
 * a prefix, a numeric value and a suffix, recovering the grouping and decimal
 * separators so an intermediate frame can be re-rendered in the same shape.
 * Returns null when there is no numeric core to animate.
 */
export function parseFormattedValue(text: string): ParsedValue | null {
  const firstDigit = text.search(/\d/)
  if (firstDigit === -1) return null
  let lastDigit = -1
  for (let index = text.length - 1; index >= 0; index -= 1) {
    if (text[index]! >= '0' && text[index]! <= '9') { lastDigit = index; break }
  }
  const core = text.slice(firstDigit, lastDigit + 1)
  const prefix = text.slice(0, firstDigit)
  const suffix = text.slice(lastDigit + 1)
  const negative = /[-−]\s*$/.test(prefix)

  const separators = [...core].filter((character) => character < '0' || character > '9')
  let decimalSep = ''
  let decimals = 0
  let groupSep = ''
  if (separators.length > 0) {
    const distinct = [...new Set(separators)]
    const lastSep = separators[separators.length - 1]!
    const trailing = core.length - 1 - core.lastIndexOf(lastSep)
    // A space is always a group separator; otherwise the last-occurring
    // separator is a decimal point when it appears once with 1-2 trailing
    // digits, or when a distinct second separator character exists.
    const spaceLast = groupSpaces.test(lastSep)
    const occurrences = separators.filter((character) => character === lastSep).length
    const isDecimal = !spaceLast && (distinct.length > 1 || (occurrences === 1 && trailing <= 2))
    if (isDecimal) {
      decimalSep = lastSep
      decimals = trailing
      groupSep = distinct.find((character) => character !== lastSep) ?? ''
    } else {
      groupSep = lastSep
    }
  }

  const digitsOnly = decimalSep
    ? core.slice(0, core.lastIndexOf(decimalSep)).replace(/\D/g, '') + '.' + core.slice(core.lastIndexOf(decimalSep) + 1).replace(/\D/g, '')
    : core.replace(/\D/g, '')
  const magnitude = Number(digitsOnly)
  if (!Number.isFinite(magnitude)) return null
  return { prefix, suffix, value: negative ? -magnitude : magnitude, decimals, groupSep, decimalSep }
}

function formatCore(value: number, parsed: ParsedValue): string {
  const fixed = Math.abs(value).toFixed(parsed.decimals)
  const [integer, fraction] = fixed.split('.')
  const grouped = parsed.groupSep
    ? integer!.replace(/\B(?=(\d{3})+(?!\d))/g, parsed.groupSep)
    : integer!
  const body = fraction ? `${grouped}${parsed.decimalSep}${fraction}` : grouped
  return `${parsed.prefix}${body}${parsed.suffix}`
}

/**
 * Renders a stat value, counting from the previous value to the next over a
 * short ease-out when the value changes. The resting output is always the exact
 * formatted string it was given, so parity with the server renderer is byte for
 * byte. First mount, unparseable values, visual-regression runs and reduced
 * motion all render the final value with no animation.
 */
export function StatValueTicker({ text }: { text: string }) {
  const [display, setDisplay] = useState(text)
  const previous = useRef(text)
  const mounted = useRef(false)
  const raf = useRef<number>()

  useEffect(() => {
    const from = parseFormattedValue(previous.current)
    const to = parseFormattedValue(text)
    const before = previous.current
    previous.current = text

    if (!mounted.current) {
      mounted.current = true
      setDisplay(text)
      return
    }
    const reduce = isVisualRegression()
      || prefersReducedMotion()
      || typeof requestAnimationFrame === 'undefined'
    if (reduce || before === text || !from || !to || from.decimals !== to.decimals) {
      setDisplay(text)
      return
    }

    const start = performance.now()
    const step = (now: number) => {
      const progress = Math.min(1, (now - start) / tickerDurationMs)
      const eased = 1 - Math.pow(1 - progress, 3)
      if (progress >= 1) {
        raf.current = undefined
        setDisplay(text)
        return
      }
      setDisplay(formatCore(from.value + (to.value - from.value) * eased, to))
      raf.current = requestAnimationFrame(step)
    }
    raf.current = requestAnimationFrame(step)
    return () => {
      if (raf.current !== undefined) cancelAnimationFrame(raf.current)
      raf.current = undefined
    }
  }, [text])

  return <>{display}</>
}
