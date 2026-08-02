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
      {/* Labelled, like the three controls beside it. As the row's only icon-only
          member it had a smaller hit box, a lighter border and a generic
          two-rectangles glyph, and nothing on screen said it copies a link to
          this filtered slice. The label is also what anchors the confirmation
          below: a 28px button under a 148px note is why the note used to hang
          over its neighbour. */}
      <button
        aria-label={label}
        className="lens-export-button"
        onClick={() => { void copy() }}
        title={label}
        type="button"
      >
        {status === 'copied' ? <Check /> : <Copy />}
        <span>{label}</span>
      </button>
      {/* The note keeps the trigger's own width (see .lens-share-control), so
          it can no longer reach across «Представления» beside it — and the
          label above never changes, so the row does not reflow either. */}
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
