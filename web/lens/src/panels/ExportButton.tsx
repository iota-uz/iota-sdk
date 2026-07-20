import { useExport, useTranslate } from '../runtime'

export interface ExportButtonProps {
  panelId?: string
  label?: string
}

export function ExportButton({ panelId, label }: ExportButtonProps) {
  const exportState = useExport(panelId)
  const translate = useTranslate()
  if (!exportState.available) return null

  const defaultLabel = panelId
    ? translate('export.panel', 'Export panel')
    : translate('export.dashboard', 'Export dashboard')
  const resolvedLabel = label ?? defaultLabel
  const pending = exportState.status === 'pending'
  const retry = exportState.status === 'retry'
  return (
    <div className="lens-export-control">
      <button
        aria-busy={pending}
        className={`lens-export-button${retry ? ' lens-export-button-retry' : ''}`}
        disabled={pending}
        onClick={() => { void exportState.run() }}
        title={exportState.message}
        type="button"
      >
        <span aria-hidden="true">{pending ? '···' : retry ? '↻' : '↓'}</span>
        <span>
          {pending
            ? translate('export.pending', 'Exporting…')
            : retry ? translate('export.retry', 'Retry export') : resolvedLabel}
        </span>
      </button>
      {exportState.message && (
        <span className={`lens-export-message${exportState.status === 'error' ? ' lens-export-message-error' : ''}`} role="status">
          {exportState.message}
        </span>
      )}
    </div>
  )
}
