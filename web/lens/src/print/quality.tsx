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
