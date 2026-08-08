import { FailureReasonSchema, type FailureReason } from '../contract'

/**
 * The kind of failure behind a panel's error, when the server named one.
 *
 * The reason is read off the error object rather than matched against an error
 * class: an error crosses a module boundary, a mocked runtime, and a bundle
 * split on its way here, and any of those breaks `instanceof` while leaving the
 * value intact. The contract schema is what decides a reason is real, so a
 * vocabulary this build does not know reads as no reason at all — which is the
 * generic case, and correct.
 */
export function failureReason(error: unknown): FailureReason | undefined {
  if (!error || typeof error !== 'object') return undefined
  const parsed = FailureReasonSchema.safeParse((error as { reason?: unknown }).reason)
  return parsed.success ? parsed.data : undefined
}

export interface PanelFailureCopy {
  /** The sentence to show. Always populated. */
  message: string
  /**
   * Whether this is a fault worth announcing. A cancelled load is not: the
   * reader stopped it themselves by leaving or asking something else, and a
   * screen reader interrupting them to report their own action is noise.
   */
  alert: boolean
}

type Translate = (key: string, fallback: string) => string

const genericDefault = 'This panel could not be rendered.'
const timeoutDefault = 'There is too much data in the selected period to load it in time. Pick a shorter period, or try again.'
const canceledDefault = 'Loading stopped before it finished. Try again when you are ready.'
// A host that explicitly translates panel.error to the English default text
// is indistinguishable, by value, from a host that never translated it at
// all — genericDefault is a legitimate translation, not only a fallback. Ask
// with a fallback nothing would ever choose on purpose, so getting it back
// means the key is absent, independently of what a real translation says.
// Each translate() call below still takes its key as a literal string, so
// the Go-side i18n call-site scanner (pkg/lens/document/i18n_test.go) still
// finds it.
const missingTranslationSentinel = ' lens-missing-translation'

/**
 * The reader-facing sentence for a failed panel.
 *
 * An absent or unrecognized reason — an older server, a newer vocabulary — is
 * the generic case, which is why the switch has a default.
 *
 * What a missing reason-specific translation falls back to depends on whether
 * the host has a catalogue at all. A host that translated `panel.error` and
 * nothing newer gets its own sentence back: one untranslated key is not worth
 * dropping a reader into another language mid-board. A host with no catalogue
 * is already reading the runtime's English, so it gets the English that
 * actually says something.
 */
export function panelFailureCopy(error: unknown, translate: Translate): PanelFailureCopy {
  const rawGeneric = translate('panel.error', missingTranslationSentinel)
  const localizedGeneric = rawGeneric === missingTranslationSentinel ? undefined : rawGeneric
  const generic = localizedGeneric ?? genericDefault
  switch (failureReason(error)) {
    case 'timeout': {
      const raw = translate('panel.error.timeout', missingTranslationSentinel)
      return { message: raw === missingTranslationSentinel ? (localizedGeneric ?? timeoutDefault) : raw, alert: true }
    }
    case 'canceled': {
      const raw = translate('panel.error.canceled', missingTranslationSentinel)
      return { message: raw === missingTranslationSentinel ? (localizedGeneric ?? canceledDefault) : raw, alert: false }
    }
    default:
      return { message: generic, alert: true }
  }
}
