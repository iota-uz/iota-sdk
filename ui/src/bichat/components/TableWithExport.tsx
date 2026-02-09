/**
 * TableWithExport Component
 * Wraps markdown tables with an export button that sends a message to export the table
 */

import { memo, useCallback, type ReactNode } from 'react'
import { TableExportButton } from './TableExportButton'

/** Default message sent when user clicks the export button */
const DEFAULT_EXPORT_MESSAGE = 'Export the table above to Excel'

interface TableWithExportProps {
  /** The table content to render */
  children: ReactNode
  /** Function to send a message (from chat context) */
  sendMessage?: (content: string) => void
  /** Whether sending is disabled (loading or streaming) */
  disabled?: boolean
  /** Custom export message to send */
  exportMessage?: string
  /** Export button label */
  exportLabel?: string
}

export const TableWithExport = memo(function TableWithExport({
  children,
  sendMessage,
  disabled = false,
  exportMessage = DEFAULT_EXPORT_MESSAGE,
  exportLabel = 'Export',
}: TableWithExportProps) {
  const handleExport = useCallback(() => {
    sendMessage?.(exportMessage)
  }, [sendMessage, exportMessage])

  return (
    <>
      <div className="markdown-table-wrapper overflow-x-auto">
        <table className="markdown-table w-full border-collapse">{children}</table>
      </div>
      {sendMessage && (
        <div className="flex justify-end mt-1">
          <TableExportButton onClick={handleExport} disabled={disabled} label={exportLabel} />
        </div>
      )}
    </>
  )
})
