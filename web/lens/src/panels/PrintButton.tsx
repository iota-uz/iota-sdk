import { CircleNotch } from '../icons'
import { usePrint, useTranslate } from '../runtime'

function Printer() {
  return (
    <svg aria-hidden="true" className="lens-icon" focusable="false" height="14" viewBox="0 0 256 256" width="14">
      <rect x="64" y="24" width="128" height="64" rx="8" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
      <rect x="64" y="152" width="128" height="80" rx="8" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
      <path d="M64,184H40a16,16,0,0,1-16-16V112A24,24,0,0,1,48,88H208a24,24,0,0,1,24,24v56a16,16,0,0,1-16,16H192" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
      <circle cx="192" cy="120" r="8" fill="currentColor" />
    </svg>
  )
}

export function PrintButton() {
  const print = usePrint()
  const translate = useTranslate()
  if (!print.available) return null
  const pending = print.status === 'pending'
  const label = pending
    ? translate('print.pending', 'Preparing report…')
    : translate('print.button', 'Export to PDF')
  return (
    <div className="lens-export-control">
      <button
        aria-busy={pending}
        className="lens-export-button lens-print-button"
        disabled={pending}
        onClick={() => { void print.run() }}
        title={print.message}
        type="button"
      >
        {pending ? <CircleNotch className="lens-icon-spin" /> : <Printer />}
        <span>{label}</span>
      </button>
      {(pending || print.message) && (
        <span
          className={`lens-export-message${print.status === 'error' ? ' lens-export-message-error' : ''}`}
          role="status"
        >
          {pending ? label : print.message}
        </span>
      )}
    </div>
  )
}
