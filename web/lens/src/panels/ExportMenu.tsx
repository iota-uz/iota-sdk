import { useCallback, useEffect, useRef, useState } from 'react'
import { ArrowClockwise, CaretDown, CircleNotch, DownloadSimple } from '../icons'
import { useExport, usePrint, useTranslate } from '../runtime'

/** One artefact the dashboard can produce. */
interface ExportChoice {
  key: string
  label: string
  pending: boolean
  retry: boolean
  message?: string
  run: () => void
}

/**
 * One export control for the whole dashboard: the first click asks which
 * artefact, the second produces it.
 *
 * The header used to carry a button per format, which reads as two unrelated
 * commands and grows with every format added. A single trigger keeps the
 * header's action row to one entry and makes the choice explicit — while a
 * dashboard that can only produce one artefact still exports it in one click,
 * because a menu of one is a question with no answer.
 */
export function ExportMenu() {
  const exportState = useExport()
  const print = usePrint()
  const translate = useTranslate()
  const [open, setOpen] = useState(false)
  const container = useRef<HTMLDivElement>(null)

  const close = useCallback(() => setOpen(false), [])

  useEffect(() => {
    if (!open || typeof document === 'undefined') return undefined
    const onPointerDown = (event: PointerEvent) => {
      if (!(event.target instanceof Node)) return
      if (container.current?.contains(event.target)) return
      close()
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.stopPropagation()
      close()
    }
    document.addEventListener('pointerdown', onPointerDown, true)
    document.addEventListener('keydown', onKeyDown, true)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown, true)
      document.removeEventListener('keydown', onKeyDown, true)
    }
  }, [close, open])

  const choices: ExportChoice[] = []
  if (exportState.available) {
    choices.push({
      key: 'data',
      label: translate('export.data', 'Data (XLSX)'),
      pending: exportState.status === 'pending',
      retry: exportState.status === 'retry',
      message: exportState.message ?? undefined,
      run: () => { void exportState.run() },
    })
  }
  if (print.available) {
    choices.push({
      key: 'report',
      label: translate('export.report', 'Report (PDF)'),
      pending: print.status === 'pending',
      retry: false,
      message: print.message ?? undefined,
      run: () => { void print.run() },
    })
  }
  if (choices.length === 0) return null

  const busy = choices.find((choice) => choice.pending)
  const retry = choices.find((choice) => choice.retry)
  // Only one thing can be in flight at a time, so the trigger speaks for
  // whichever it is: an export that says nothing while it runs reads as a
  // click that did not land.
  const status = busy ?? retry ?? choices.find((choice) => choice.message)
  const single = choices.length === 1 ? choices[0] : undefined
  const label = busy
    ? (busy.key === 'report'
      ? translate('print.pending', 'Preparing report…')
      : translate('export.pending', 'Exporting…'))
    : retry
      ? translate('export.retry', 'Retry export')
      : single
        ? single.label
        : translate('export.menu', 'Export')

  return (
    <div className="lens-export-control" ref={container}>
      <button
        aria-busy={Boolean(busy)}
        aria-expanded={single ? undefined : open}
        aria-haspopup={single ? undefined : 'menu'}
        className={`lens-export-button${retry ? ' lens-export-button-retry' : ''}`}
        disabled={Boolean(busy)}
        onClick={() => {
          if (single) {
            single.run()
            return
          }
          setOpen((current) => !current)
        }}
        title={status?.message ?? undefined}
        type="button"
      >
        {busy
          ? <CircleNotch className="lens-icon-spin" />
          : retry ? <ArrowClockwise /> : <DownloadSimple />}
        <span>{label}</span>
        {!single && !busy && <CaretDown className="lens-export-caret" />}
      </button>
      {open && !single && (
        <div className="lens-export-menu" role="menu">
          {choices.map((choice) => (
            <button
              className="lens-export-menu-item"
              key={choice.key}
              onClick={() => {
                close()
                choice.run()
              }}
              role="menuitem"
              type="button"
            >
              {choice.label}
            </button>
          ))}
        </div>
      )}
      {(busy || status?.message) && (
        <span
          className={`lens-export-message${exportState.status === 'error' || print.status === 'error' ? ' lens-export-message-error' : ''}${
            busy ? ' lens-export-message-pending' : ''}`}
          role="status"
        >
          {busy ? label : status?.message}
        </span>
      )}
    </div>
  )
}
