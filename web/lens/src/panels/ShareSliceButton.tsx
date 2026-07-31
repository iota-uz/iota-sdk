import { useEffect, useState } from 'react'
import { Check, Copy } from '../icons'
import { useTranslate } from '../runtime'

/** Copies the canonical browser URL, which is the Lens slice state contract. */
export function ShareSliceButton() {
  const translate = useTranslate()
  const [status, setStatus] = useState<'idle' | 'copied' | 'error'>('idle')
  const label = status === 'copied'
    ? translate('share.copied', 'Link copied')
    : translate('share.copy', 'Copy slice link')

  useEffect(() => {
    if (status !== 'copied') return undefined
    const timeout = globalThis.setTimeout(() => setStatus('idle'), 1800)
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
    <div className="lens-export-control">
      <button
        aria-label={label}
        className="lens-export-button lens-icon-button"
        onClick={() => { void copy() }}
        title={label}
        type="button"
      >
        {status === 'copied' ? <Check /> : <Copy />}
      </button>
      {status !== 'idle' && (
        <span className={`lens-export-message${status === 'error' ? ' lens-export-message-error' : ''}`} role="status">
          {status === 'error' ? translate('share.error', 'Unable to copy link') : label}
        </span>
      )}
    </div>
  )
}
