/**
 * DownloadCard component
 * File-download card for artifacts (Excel, PDF)
 * with type-specific icon, metadata, and download action.
 */

import { DownloadSimple } from '@phosphor-icons/react'
import type { Artifact } from '../types'
import { getFileVisual } from '../utils/fileUtils'

interface DownloadCardProps {
  artifact: Artifact
}

const MIME_BY_TYPE: Record<string, string> = {
  excel: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  pdf: 'application/pdf',
}

export function DownloadCard({ artifact }: DownloadCardProps) {
  const { type, filename, url, sizeReadable, rowCount, description } = artifact
  const visual = getFileVisual(MIME_BY_TYPE[type], filename)
  const Icon = visual.icon

  return (
    <a
      href={url}
      download={filename}
      className={[
        'group/dl flex items-center gap-2.5 rounded-xl',
        'border border-gray-200/80 dark:border-gray-700/60',
        'bg-white dark:bg-gray-800/60',
        'px-2.5 py-2',
        'transition-all duration-150',
        'hover:border-gray-300 dark:hover:border-gray-600',
        'hover:shadow-sm',
        'hover:-translate-y-px active:translate-y-0',
      ].join(' ')}
    >
      {/* File type icon */}
      <div className={`flex-shrink-0 flex items-center justify-center w-10 h-10 rounded-lg ${visual.bgColor}`}>
        <Icon size={20} weight="duotone" className={visual.iconColor} />
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-1.5 min-w-0">
          <span className="text-[13px] font-medium text-gray-900 dark:text-gray-100 truncate">
            {filename}
          </span>
          <span className="flex-shrink-0 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider rounded bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400">
            {visual.label}
          </span>
        </div>
        {(sizeReadable || rowCount !== undefined) && (
          <div className="flex items-center gap-1.5 text-[11px] text-gray-400 dark:text-gray-500">
            {sizeReadable && <span>{sizeReadable}</span>}
            {sizeReadable && rowCount !== undefined && (
              <span className="w-0.5 h-0.5 rounded-full bg-gray-300 dark:bg-gray-600" />
            )}
            {rowCount !== undefined && (
              <span>{rowCount.toLocaleString()} rows</span>
            )}
          </div>
        )}
        {description && (
          <p className="mt-0.5 text-[11px] text-gray-400 dark:text-gray-500 line-clamp-1">
            {description}
          </p>
        )}
      </div>

      {/* Download arrow */}
      <div className="flex-shrink-0 p-1 text-gray-400 dark:text-gray-500 group-hover/dl:text-gray-600 dark:group-hover/dl:text-gray-300 transition-colors duration-150">
        <DownloadSimple
          size={16}
          weight="bold"
          className="transition-transform duration-200 group-hover/dl:translate-y-0.5"
        />
      </div>
    </a>
  )
}
