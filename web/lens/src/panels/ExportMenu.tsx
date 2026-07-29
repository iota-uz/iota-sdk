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
  const triggerRef = useRef<HTMLButtonElement>(null)
  const itemRefs = useRef<Array<HTMLButtonElement | null>>([])

  const close = useCallback(() => setOpen(false), [])
  // Escape and item activation close the menu; per the menu-button pattern,
  // both also hand focus back to the trigger instead of leaving it stranded
  // on a node that just left the document.
  const closeAndFocusTrigger = useCallback(() => {
    close()
    triggerRef.current?.focus()
  }, [close])

  useEffect(() => {
    if (!open || typeof document === 'undefined') return undefined
    itemRefs.current[0]?.focus()
    const onPointerDown = (event: PointerEvent) => {
      if (!(event.target instanceof Node)) return
      if (container.current?.contains(event.target)) return
      close()
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.stopPropagation()
      closeAndFocusTrigger()
    }
    document.addEventListener('pointerdown', onPointerDown, true)
    document.addEventListener('keydown', onKeyDown, true)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown, true)
      document.removeEventListener('keydown', onKeyDown, true)
    }
  }, [close, closeAndFocusTrigger, open])

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
  const status = busy ?? choices.find((choice) => choice.message)
  const single = choices.length === 1 ? choices[0] : undefined
  // Retry only changes the trigger's face when it is also what a click does:
  // a click on a multi-choice trigger opens the menu regardless of whether
  // one of its choices needs retrying, so the retry label/icon belong to
  // `single` alone.
  const retrying = Boolean(single?.retry)
  const label = busy
    ? (busy.key === 'report'
      ? translate('print.pending', 'Preparing report…')
      : translate('export.pending', 'Exporting…'))
    : single
      ? (retrying ? translate('export.retry', 'Retry export') : single.label)
      : translate('export.menu', 'Export')

  return (
    <div className="lens-export-control" ref={container}>
      <button
        aria-busy={Boolean(busy)}
        aria-expanded={single ? undefined : open}
        aria-haspopup={single ? undefined : 'menu'}
        className={`lens-export-button${retrying ? ' lens-export-button-retry' : ''}`}
        disabled={Boolean(busy)}
        onClick={() => {
          if (single) {
            single.run()
            return
          }
          setOpen((current) => !current)
        }}
        ref={triggerRef}
        title={status?.message ?? undefined}
        type="button"
      >
        {busy
          ? <CircleNotch className="lens-icon-spin" />
          : retrying ? <ArrowClockwise /> : <DownloadSimple />}
        <span>{label}</span>
        {!single && !busy && <CaretDown className="lens-export-caret" />}
      </button>
      {open && !single && (
        <div
          className="lens-export-menu"
          onKeyDown={(event) => {
            if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
            event.preventDefault()
            const items = itemRefs.current.filter((item): item is HTMLButtonElement => item !== null)
            if (items.length === 0) return
            const current = items.indexOf(document.activeElement as HTMLButtonElement)
            const delta = event.key === 'ArrowDown' ? 1 : -1
            items[(current + delta + items.length) % items.length]?.focus()
          }}
          role="menu"
        >
          {choices.map((choice, index) => (
            <button
              className="lens-export-menu-item"
              key={choice.key}
              onClick={() => {
                closeAndFocusTrigger()
                choice.run()
              }}
              ref={(element) => { itemRefs.current[index] = element }}
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
