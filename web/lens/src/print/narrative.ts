import type { Frame, Panel } from '../contract'
import { formatFieldValue } from '../runtime'
import { indexOf, numeric, text } from './values'

/**
 * A sentence the report can print under a figure, expressed as a translation
 * key with its variables so a catalogue keeps word order.
 *
 * Every fact is a restatement of numbers printed on the same page: the largest
 * share, the biggest deduction, the change between the first and last point.
 * Nothing here explains, attributes or projects — a printed report must not say
 * more than its own table does.
 */
export interface NarrativeFact {
  labelKey: string
  fallback: string
  vars: Record<string, string | number>
}

interface Reading {
  /** Position in the frame, so a fact can align a reading with other columns. */
  row: number
  /** The series this reading belongs to, when the frame carries more than one. */
  group: string
  label: string
  value: number
  formatted: string
}

function readings(panel: Panel, frame: Frame, locale: string): Array<Reading> {
  const labelIndex = indexOf(frame, panel.encoding.label ?? panel.encoding.category ?? panel.encoding.id)
  const valueIndex = indexOf(frame, panel.encoding.value)
  const seriesIndex = indexOf(frame, panel.encoding.series)
  if (valueIndex < 0) return []
  const format = panel.encoding.value ? panel.format[panel.encoding.value] : undefined
  const result: Array<Reading> = []
  frame.rows.forEach((row, index) => {
    const value = numeric(row[valueIndex])
    if (value === undefined) return
    result.push({
      row: index,
      group: seriesIndex >= 0 ? text(row[seriesIndex]) : '',
      label: labelIndex >= 0 ? text(row[labelIndex]) : text(row[valueIndex]),
      value,
      formatted: formatFieldValue(row[valueIndex], format, locale),
    })
  })
  return result
}

function percent(locale: string, ratio: number): string {
  return new Intl.NumberFormat(locale, {
    style: 'percent',
    minimumFractionDigits: ratio >= 0.1 ? 0 : 1,
    maximumFractionDigits: 1,
  }).format(ratio)
}

function partitionFact(panel: Panel, frame: Frame, locale: string): NarrativeFact | undefined {
  const all = readings(panel, frame, locale).filter(({ value }) => value > 0)
  if (all.length < 2) return undefined
  // A frame can hold more than one partition — the two rings of a donut, say.
  // A share only means anything inside one of them, so the sentence weighs the
  // largest reading against its own ring rather than against the sum of both.
  // Which ring that is is left to the table beneath, which names every row.
  const biggest = [...all].sort((left, right) => right.value - left.value)[0]!
  const positive = all.filter(({ group }) => group === biggest.group)
  if (positive.length < 2) return undefined
  const total = positive.reduce((sum, { value }) => sum + value, 0)
  if (total <= 0) return undefined
  const ranked = [...positive].sort((left, right) => right.value - left.value)
  const leader = ranked[0]!
  if (ranked.length === 2) {
    return {
      labelKey: 'print.factPair',
      fallback: '{label} carries {share} of the total; the remainder is {rest}.',
      vars: {
        label: leader.label,
        share: percent(locale, leader.value / total),
        rest: ranked[1]!.label,
      },
    }
  }
  const topThree = ranked.slice(0, 3).reduce((sum, { value }) => sum + value, 0)
  // With exactly three readings the three largest are all of them: saying they
  // are «ahead of 0 smaller categories» invents a tail that is not there.
  if (ranked.length === 3) {
    return {
      labelKey: 'print.factPartitionAll',
      fallback: '{label} is the largest of the three at {share}; together they make the whole.',
      vars: {
        label: leader.label,
        share: percent(locale, leader.value / total),
      },
    }
  }
  return {
    labelKey: 'print.factPartition',
    fallback: '{label} is the largest share at {share}; the three largest together make {top}, ahead of {rest} smaller categories.',
    vars: {
      label: leader.label,
      share: percent(locale, leader.value / total),
      top: percent(locale, topThree / total),
      rest: ranked.length - 3,
    },
  }
}

function progressFact(panel: Panel, frame: Frame, locale: string): NarrativeFact | undefined {
  const max = panel.radial?.max
  if (!max || max <= 0) return undefined
  const rows = readings(panel, frame, locale)
  const reached = rows.reduce((sum, { value }) => sum + Math.max(0, value), 0)
  if (reached <= 0) return undefined
  const valueFormat = panel.encoding.value ? panel.format[panel.encoding.value] : undefined
  return {
    labelKey: 'print.factProgress',
    fallback: '{value} of {max} — {share} covered.',
    vars: {
      value: formatFieldValue(reached, valueFormat, locale),
      max: formatFieldValue(max, valueFormat, locale),
      share: percent(locale, Math.min(reached / max, 1)),
    },
  }
}

function bridgeFact(panel: Panel, frame: Frame, locale: string): NarrativeFact | undefined {
  const rows = readings(panel, frame, locale)
  if (rows.length < 2) return undefined
  const finalIndex = indexOf(frame, panel.encoding.final)
  const opening = rows[0]!
  // A bridge marks its closing total explicitly; without the marker the last
  // stage is the outcome the columns walk towards.
  const result = rows.find(({ row }) => finalIndex >= 0 && frame.rows[row]?.[finalIndex] === true)
    ?? rows[rows.length - 1]!
  const largest = rows
    .filter((reading) => reading !== opening && reading !== result && reading.value < 0)
    .sort((left, right) => left.value - right.value)[0]
  if (!largest) {
    return {
      labelKey: 'print.factBridgeEnds',
      fallback: '{start} becomes {end}.',
      vars: { start: opening.formatted, end: result.formatted },
    }
  }
  const vars = {
    start: opening.formatted,
    end: result.formatted,
    label: largest.label,
    amount: largest.formatted,
  }
  // Without a positive opening amount there is nothing to take a share of, so
  // the sentence keeps the deduction and drops the proportion.
  if (opening.value <= 0) {
    return {
      labelKey: 'print.factBridgeBare',
      fallback: '{start} becomes {end}; the largest deduction is {label} at {amount}.',
      vars,
    }
  }
  return {
    labelKey: 'print.factBridge',
    fallback: '{start} becomes {end}; the largest deduction is {label} at {amount} — {share} of the opening amount.',
    vars: { ...vars, share: percent(locale, Math.abs(largest.value) / opening.value) },
  }
}

function seriesFact(panel: Panel, frame: Frame, locale: string): NarrativeFact | undefined {
  const rows = readings(panel, frame, locale)
  if (rows.length < 2) return undefined
  const first = rows[0]!
  const last = rows[rows.length - 1]!
  const peak = [...rows].sort((left, right) => right.value - left.value)[0]!
  const vars = {
    first: first.formatted,
    firstLabel: first.label,
    last: last.formatted,
    lastLabel: last.label,
    peak: peak.formatted,
    peakLabel: peak.label,
  }
  // A series that starts at zero has no percentage change to state; the two
  // endpoints still tell the reader where it went.
  if (first.value === 0) {
    return {
      labelKey: 'print.factSeriesBare',
      fallback: '{first} at {firstLabel}, {last} at {lastLabel}; the peak is {peak} at {peakLabel}.',
      vars,
    }
  }
  const change = (last.value - first.value) / Math.abs(first.value)
  return {
    labelKey: 'print.factSeries',
    fallback: '{first} at {firstLabel} to {last} at {lastLabel} ({change}); the peak is {peak} at {peakLabel}.',
    vars: { ...vars, change: `${change >= 0 ? '+' : '−'}${percent(locale, Math.abs(change))}` },
  }
}

function tableFact(panel: Panel, frame: Frame, locale: string): NarrativeFact | undefined {
  if (frame.rows.length < 2) return undefined
  const rows = readings(panel, frame, locale)
  const leader = [...rows].sort((left, right) => right.value - left.value)[0]
  if (!leader) {
    return {
      labelKey: 'print.factRows',
      fallback: '{rows} rows.',
      vars: { rows: frame.rows.length },
    }
  }
  return {
    labelKey: 'print.factTable',
    fallback: '{rows} rows; the largest is {label} at {value}.',
    vars: { rows: frame.rows.length, label: leader.label, value: leader.formatted },
  }
}

/** A chapter's own headline: one reading per figure, in printing order. */
export interface HeadlineReading {
  id: string
  label: string
  value: string
}

/**
 * What a chapter says in one line, before its figures.
 *
 * A reader arriving at «Формирование результата» should not have to assemble
 * the chapter's outcome from six figures. Each figure contributes exactly one
 * number — the value it states when it states only one, or the total it is
 * taken against — so the opener is a restatement of the pages below it and
 * never an interpretation of them.
 */
export function headlineReadings(
  figures: Array<{ id: string; panel: Panel; frame?: Frame }>,
  locale: string,
  limit = 4,
): Array<HeadlineReading> {
  const readings: Array<HeadlineReading> = []
  for (const figure of figures) {
    if (readings.length >= limit) break
    const { panel, frame } = figure
    const valueField = panel.encoding.value
    const format = valueField ? panel.format[valueField] : undefined
    const valueIndex = frame ? indexOf(frame, valueField) : -1
    const single = frame?.rows.length === 1 && valueIndex >= 0
      ? frame.rows[0]?.[valueIndex]
      : undefined
    const value = single !== undefined
      ? formatFieldValue(single, format, locale)
      : panel.total !== undefined
        ? formatFieldValue(panel.total, format, locale)
        : undefined
    if (!value) continue
    readings.push({ id: figure.id, label: panel.title, value })
  }
  // One reading is the figure itself, restated. A line is worth its ink from
  // two upwards.
  return readings.length >= 2 ? readings : []
}

/**
 * The one sentence worth printing under this figure, or nothing when the frame
 * carries no reading that a sentence would add to.
 */
export function narrativeFact(panel: Panel, frame: Frame | undefined, locale: string): NarrativeFact | undefined {
  if (!frame || frame.rows.length === 0) return undefined
  if (panel.radial?.mode === 'progress') return progressFact(panel, frame, locale)
  if (panel.semantics === 'partition') return partitionFact(panel, frame, locale)
  if (panel.semantics === 'reconciliation') return bridgeFact(panel, frame, locale)
  // "From the first point to the last" is a statement about a progression. A
  // bar chart of products ordered by size is not one: its first and last
  // columns are its largest and smallest, and reading them as a change would
  // invent a trend the chart never claimed.
  if (panel.semantics === 'series' && (panel.kind === 'line' || panel.kind === 'area')) {
    return seriesFact(panel, frame, locale)
  }
  return tableFact(panel, frame, locale)
}
