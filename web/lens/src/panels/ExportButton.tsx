import { useExport } from '../runtime'

export interface ExportButtonProps {
  panelId?: string
  label?: string
}

export function ExportButton({ panelId, label = panelId ? 'Export panel' : 'Export dashboard' }: ExportButtonProps) {
  const exportState = useExport(panelId)
  if (!exportState.available) return null

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
        <span>{pending ? 'Exporting…' : retry ? 'Retry export' : label}</span>
      </button>
      {exportState.message && (
        <span className={`lens-export-message${exportState.status === 'error' ? ' lens-export-message-error' : ''}`} role="status">
          {exportState.message}
        </span>
      )}
    </div>
  )
}
