/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import type { JSX } from 'solid-js'
import { Show } from 'solid-js'
import type { Availability, Confidence } from '../contract'
import { useTranslate } from '../runtime'
import {
  Approximate,
  Brackets,
  type IconProps,
  SealCheck,
  SlashCircle,
  Sliders,
  Tray,
  WarningTriangle,
} from '../icons'

/**
 * The two data-quality axes are shown as a single chip with a strict
 * precedence: an unavailable-ish availability (anything other than `available`)
 * wins over a confidence, because a number that cannot exist is a more urgent
 * fact than how a shown number was produced. When availability is `available`
 * (or unset) the confidence chip describes the shown number's provenance.
 *
 * Tones are quality-neutral, never business-positive: a `verified` loss is not
 * "good", so `verified` reads as a strong-neutral rather than a green success.
 */

/* eslint-disable react-refresh/only-export-components */
interface QualityMeta {
  className: string
  icon: (props: IconProps) => JSX.Element
  /** Named labelKey so the Go i18n-parity scanner picks these up as catalog keys. */
  labelKey: string
  fallback: string
}

const CONFIDENCE_META: Record<Confidence, QualityMeta> = {
  verified: { className: 'lens-quality-verified', icon: SealCheck, labelKey: 'confidence.verified', fallback: 'Verified' },
  calculated: { className: 'lens-quality-calculated', icon: Brackets, labelKey: 'confidence.calculated', fallback: 'Calculated' },
  proxy: { className: 'lens-quality-proxy', icon: Approximate, labelKey: 'confidence.proxy', fallback: 'Proxy' },
  requires_reconciliation: {
    className: 'lens-quality-requires-reconciliation',
    icon: WarningTriangle,
    labelKey: 'confidence.requires_reconciliation',
    fallback: 'Requires reconciliation',
  },
}

const AVAILABILITY_META: Record<Exclude<Availability, 'available'>, QualityMeta> = {
  config_required: {
    className: 'lens-quality-config-required',
    icon: Sliders,
    labelKey: 'availability.config_required',
    fallback: 'Configuration required',
  },
  empty_source: { className: 'lens-quality-empty-source', icon: Tray, labelKey: 'availability.empty_source', fallback: 'No source data' },
  unavailable: { className: 'lens-quality-unavailable', icon: SlashCircle, labelKey: 'availability.unavailable', fallback: 'Unavailable' },
}

export interface QualityInput {
  confidence?: Confidence
  availability?: Availability
}

type ResolvedQuality =
  | { axis: 'confidence'; value: Confidence; meta: QualityMeta }
  | { axis: 'availability'; value: Exclude<Availability, 'available'>; meta: QualityMeta }

/** Applies the availability-first precedence; returns nothing when neither axis says anything. */
export function resolveQuality({ confidence, availability }: QualityInput): ResolvedQuality | undefined {
  if (availability && availability !== 'available') {
    return { axis: 'availability', value: availability, meta: AVAILABILITY_META[availability] }
  }
  if (confidence) return { axis: 'confidence', value: confidence, meta: CONFIDENCE_META[confidence] }
  return undefined
}

/**
 * The translated label of the chosen quality, for folding into an element's
 * aria-label so a screen reader hears the status inline rather than as a
 * separate chip. Returns undefined when no chip would render.
 */
export function useQualityLabel(): (input: QualityInput) => string | undefined {
  const translate = useTranslate()
  return (input) => {
    const resolved = resolveQuality(input)
    return resolved ? translate(resolved.meta.labelKey, resolved.meta.fallback) : undefined
  }
}

export interface QualityChipProps {
  confidence?: Confidence
  availability?: Availability
  className?: string
  /** Solid-style class override; treated the same as `className`. */
  class?: string
}

/**
 * A single data-quality chip. Icon and translated label are always present, so
 * color is never the sole carrier of meaning.
 */
export function QualityChip(props: QualityChipProps) {
  const translate = useTranslate()
  const resolved = () => resolveQuality({ confidence: props.confidence, availability: props.availability })
  return (
    <Show when={resolved()}>
      {(value) => {
        const label = translate(value().meta.labelKey, value().meta.fallback)
        const Icon = value().meta.icon
        return (
          <span
            class={['lens-status-chip', 'lens-quality-chip', value().meta.className, props.className, props.class].filter(Boolean).join(' ')}
            title={label}
          >
            <Icon className="lens-quality-chip-icon" />
            <span class="lens-quality-chip-label">{label}</span>
          </span>
        )
      }}
    </Show>
  )
}
