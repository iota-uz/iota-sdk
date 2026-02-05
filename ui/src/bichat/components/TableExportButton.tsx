/**
 * TableExportButton Component
 * Small inline button for exporting markdown tables to Excel
 */

import { memo } from 'react'
import { FileXls } from '@phosphor-icons/react'

interface TableExportButtonProps {
  /** Click handler for export action */
  onClick: () => void
  /** Whether the button should be disabled */
  disabled?: boolean
  /** Export button label (defaults to "Export") */
  label?: string
  /** Disabled tooltip text */
  disabledTooltip?: string
}

export const TableExportButton = memo(function TableExportButton({
  onClick,
  disabled = false,
  label = 'Export',
  disabledTooltip = 'Please wait...',
}: TableExportButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-green-600 dark:text-green-500 opacity-60 hover:opacity-90 disabled:opacity-30 disabled:cursor-not-allowed transition-opacity"
      aria-label={label}
      title={disabled ? disabledTooltip : label}
    >
      <FileXls size={16} weight="fill" />
      <span>{label}</span>
    </button>
  )
})
