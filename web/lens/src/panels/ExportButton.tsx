/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { Show } from 'solid-js'
import { ArrowClockwise, CircleNotch, DownloadSimple } from '../icons'
import { useExport, useTranslate } from '../runtime'

export interface ExportButtonProps {
  panelId?: string
  label?: string
  /** Icon-only buttons keep dense panel headers free of competing text. */
  iconOnly?: boolean
}

export function ExportButton(props: ExportButtonProps) {
  const exportState = useExport(props.panelId)
  const translate = useTranslate()
  const text = () => {
    if (exportState.status === 'pending') return translate('export.pending', 'Exporting…')
    if (exportState.status === 'retry') return translate('export.retry', 'Retry export')
    return props.label ?? (props.panelId
      ? translate('export.panel', 'Export panel')
      : translate('export.dashboard', 'Export dashboard'))
  }
  return (
    <Show when={exportState.available}>
      <div class="lens-export-control">
        <button
          aria-busy={exportState.status === 'pending'}
          aria-label={props.iconOnly ? text() : undefined}
          class={`lens-export-button${props.iconOnly ? ' lens-icon-button' : ''}${exportState.status === 'retry' ? ' lens-export-button-retry' : ''}`}
          disabled={exportState.status === 'pending'}
          onClick={() => { void exportState.run() }}
          title={exportState.message ?? (props.iconOnly ? text() : undefined)}
          type="button"
        >
          {exportState.status === 'pending'
            ? <CircleNotch className="lens-icon-spin" />
            : exportState.status === 'retry' ? <ArrowClockwise /> : <DownloadSimple />}
          {!props.iconOnly && <span>{text()}</span>}
        </button>
        {/* Pending is announced in text in both forms: an icon-only button
            swapped a 14px glyph and nothing else, which read as "nothing is
            happening" next to the dashboard button's «Exporting…». */}
        {(exportState.status === 'pending' || exportState.message) && (
          <span
            class={`lens-export-message${exportState.status === 'error' ? ' lens-export-message-error' : ''}${
              exportState.status === 'pending' ? ' lens-export-message-pending' : ''}`}
            role="status"
          >
            {exportState.status === 'pending' ? text() : exportState.message}
          </span>
        )}
      </div>
    </Show>
  )
}
