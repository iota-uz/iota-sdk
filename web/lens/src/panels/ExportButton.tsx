import { ArrowClockwise, CircleNotch, DownloadSimple } from '../icons'
import { useExport, useTranslate } from '../runtime'

export interface ExportButtonProps {
  panelId?: string
  label?: string
  /** Icon-only buttons keep dense panel headers free of competing text. */
  iconOnly?: boolean
}

export function ExportButton({ panelId, label, iconOnly = false }: ExportButtonProps) {
  const exportState = useExport(panelId)
  const translate = useTranslate()
  if (!exportState.available) return null

  const defaultLabel = panelId
    ? translate('export.panel', 'Export panel')
    : translate('export.dashboard', 'Export dashboard')
  const resolvedLabel = label ?? defaultLabel
  const pending = exportState.status === 'pending'
  const retry = exportState.status === 'retry'
  const text = pending
    ? translate('export.pending', 'Exporting…')
    : retry ? translate('export.retry', 'Retry export') : resolvedLabel
  return (
    <div className="lens-export-control">
      <button
        aria-busy={pending}
        aria-label={iconOnly ? text : undefined}
        className={`lens-export-button${iconOnly ? ' lens-icon-button' : ''}${retry ? ' lens-export-button-retry' : ''}`}
        disabled={pending}
        onClick={() => { void exportState.run() }}
        title={exportState.message ?? (iconOnly ? text : undefined)}
        type="button"
      >
        {pending
          ? <CircleNotch className="lens-icon-spin" />
          : retry ? <ArrowClockwise /> : <DownloadSimple />}
        {!iconOnly && <span>{text}</span>}
      </button>
      {/* Pending is announced in text in both forms: an icon-only button
          swapped a 14px glyph and nothing else, which read as "nothing is
          happening" next to the dashboard button's «Exporting…». */}
      {(pending || exportState.message) && (
        <span
          className={`lens-export-message${exportState.status === 'error' ? ' lens-export-message-error' : ''}${
            pending ? ' lens-export-message-pending' : ''}`}
          role="status"
        >
          {pending ? text : exportState.message}
        </span>
      )}
    </div>
  )
}
