import type { Availability, Confidence } from '../contract'
import { resolveQuality, type QualityInput } from '../panels/QualityChip'
import { useTranslate } from '../runtime'

/* eslint-disable react-refresh/only-export-components */

/**
 * Data quality on paper.
 *
 * On screen every number carries a chip saying how it was obtained — verified,
 * calculated, a proxy, or not available at all — and the chip's tooltip repeats
 * the word. The printed report dropped both, so a management estimate and a
 * ledger figure sat side by side looking equally settled. The chip prints; what
 * the chip means prints once per chapter, as a footnote.
 */

export type Translate = (key: string, fallback: string, vars?: Record<string, string | number>) => string

/** The chip's own word: the same catalogue the dashboard uses, no icon. */
export function qualityLabel(input: QualityInput, translate: Translate): string | undefined {
  const resolved = resolveQuality(input)
  return resolved ? translate(resolved.meta.labelKey, resolved.meta.fallback) : undefined
}

/**
 * What the chip means, for a reader who cannot hover it. Only qualities that
 * change how a figure may be used are explained: "verified" and "calculated"
 * are the ordinary case and would turn every page into boilerplate.
 */
export function qualityFootnote(input: QualityInput, translate: Translate): string | undefined {
  const resolved = resolveQuality(input)
  if (!resolved) return undefined
  if (resolved.axis === 'availability') {
    return translate(
      'print.noteUnavailable',
      'Not available: the source is not calculated or not configured, so the figure is shown as a dash rather than as a zero.',
    )
  }
  if (resolved.value === 'proxy') {
    return translate(
      'print.noteProxy',
      'Proxy: a management estimate derived from adjacent data, not a figure taken from the accounting system.',
    )
  }
  if (resolved.value === 'requires_reconciliation') {
    return translate(
      'print.noteReconciliation',
      'Requires reconciliation: a known difference against the accounting system has not been resolved.',
    )
  }
  return undefined
}

/**
 * What each word of the quality vocabulary means, for the closing appendix. The
 * chip beside a number tells a reader which case applies; only this list tells
 * them what the case is. Unlike the per-page footnote, the ordinary qualities
 * are defined too — a report that says «Проверено» owes the reader the
 * definition it is holding itself to.
 */
const DEFINITIONS: Record<string, { labelKey: string; fallback: string }> = {
  verified: {
    labelKey: 'print.defVerified',
    fallback: 'Verified — the figure is taken from the accounting system as recorded, without derivation.',
  },
  calculated: {
    labelKey: 'print.defCalculated',
    fallback: 'Calculated — derived from recorded data by a stated rule; the rule, not the figure, is what to check.',
  },
  proxy: {
    labelKey: 'print.defProxy',
    fallback: 'Proxy — a management estimate from adjacent data, used where the direct source does not exist.',
  },
  requires_reconciliation: {
    labelKey: 'print.defReconciliation',
    fallback: 'Requires reconciliation — a known difference against the accounting system is still open.',
  },
  config_required: {
    labelKey: 'print.defConfigRequired',
    fallback: 'Configuration required — the calculation exists but its parameters have not been set for this period.',
  },
  empty_source: {
    labelKey: 'print.defEmptySource',
    fallback: 'No source data — the source holds no rows for this period, which is not the same as a zero.',
  },
  unavailable: {
    labelKey: 'print.defUnavailable',
    fallback: 'Unavailable — the figure could not be produced, and is shown as a dash rather than as a zero.',
  },
}

const AVAILABILITY_VALUES = new Set(['config_required', 'empty_source', 'unavailable'])

export interface QualityDefinition {
  value: string
  label: string
  definition: string
}

/** The quality vocabulary this document actually used, in a fixed reading order. */
export function qualityDefinitions(inputs: Array<QualityInput>, translate: Translate): Array<QualityDefinition> {
  const seen = new Set<string>()
  const order = Object.keys(DEFINITIONS)
  for (const input of inputs) {
    const resolved = resolveQuality(input)
    if (resolved) seen.add(resolved.value)
  }
  return order
    .filter((value) => seen.has(value))
    .map((value) => {
      const meta = DEFINITIONS[value]!
      const axis: QualityInput = AVAILABILITY_VALUES.has(value)
        ? { availability: value as Availability }
        : { confidence: value as Confidence }
      return {
        value,
        label: qualityLabel(axis, translate) ?? value,
        definition: translate(meta.labelKey, meta.fallback),
      }
    })
}

export function PrintQualityChip({ confidence, availability }: {
  confidence?: Confidence
  availability?: Availability
}) {
  const translate = useTranslate()
  const resolved = resolveQuality({ confidence, availability })
  if (!resolved) return null
  return (
    <span className="lens-print-chip" data-quality={resolved.value}>
      {translate(resolved.meta.labelKey, resolved.meta.fallback)}
    </span>
  )
}
