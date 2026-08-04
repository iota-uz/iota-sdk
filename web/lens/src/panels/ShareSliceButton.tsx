import { useEffect, useState } from 'react'
import { Check, Copy } from '../icons'
import { useTranslate } from '../runtime'

/**
 * Long enough to be read. At 1.8s a confirmation was gone before a reader who
 * had just moved the pointer looked back at the button — and the copy is the
 * payoff of the action, the one moment that says the link is in the clipboard.
 */
const copiedStatusDurationMs = 3000

/** Copies the canonical browser URL, which is the Lens slice state contract. */
export function ShareSliceButton() {
  const translate = useTranslate()
  const [status, setStatus] = useState<'idle' | 'copied' | 'error'>('idle')
  const label = translate('share.copy', 'Copy slice link')
  const copiedLabel = translate('share.copied', 'Link copied')

  useEffect(() => {
    if (status !== 'copied') return undefined
    const timeout = globalThis.setTimeout(() => setStatus('idle'), copiedStatusDurationMs)
    return () => globalThis.clearTimeout(timeout)
  }, [status])

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(globalThis.location.href)
      setStatus('copied')
    } catch {
      setStatus('error')
    }
  }

  return (
    <div className="lens-export-control lens-share-control">
      {/* The glyph-only member of the secondary weight, not the icon-only weight:
          `.lens-action-square` keeps the box, the border and the 32px hit target
          of the labelled controls beside it, which is what a bare
          `.lens-icon-button` here got wrong before — a lighter, smaller control
          among bordered peers reads as disabled.

          Labelled, «Копировать ссылку на срез» spent 190px of the header's most
          contested row on the action a reader takes once a session, next to the
          period they change constantly. The name survives where a name is
          actually consulted: the accessible label, the hover title, and the
          confirmation that follows the press. */}
      <button
        aria-label={label}
        className="lens-export-button lens-action-square"
        onClick={() => { void copy() }}
        title={label}
        type="button"
      >
        {status === 'copied' ? <Check /> : <Copy />}
      </button>
      {/* Right-anchored to the trigger and sized to its own text: hung under a
          32px square at the trigger's width it would be one word per line. It
          floats rather than sitting in the flow because this is the dashboard
          header, where nothing clips and a reflow would shove the whole control
          bar sideways for the three seconds it shows. */}
      {status !== 'idle' && (
        <span
          className={`lens-export-message${status === 'error' ? ' lens-export-message-error' : ''}`}
          role="status"
        >
          {status === 'error' ? translate('share.error', 'Unable to copy link') : copiedLabel}
        </span>
      )}
    </div>
  )
}
