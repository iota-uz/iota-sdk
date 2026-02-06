/**
 * DownloadCard component
 * Polished file-download card for artifacts (Excel, PDF)
 * with type-specific icon, metadata chips, and hover animation.
 */

import { FileXls, FilePdf, DownloadSimple } from '@phosphor-icons/react'
import { Artifact } from '../types'

interface DownloadCardProps {
  artifact: Artifact
}

const typeConfig = {
  excel: {
    Icon: FileXls,
    accentBg: 'bg-emerald-50 dark:bg-emerald-950/30',
    accentBorder: 'border-emerald-200/60 dark:border-emerald-800/40',
    accentIcon: 'text-emerald-600 dark:text-emerald-400',
    badge: 'XLSX',
    badgeClass: 'bg-emerald-100 dark:bg-emerald-900/50 text-emerald-700 dark:text-emerald-300',
  },
  pdf: {
    Icon: FilePdf,
    accentBg: 'bg-rose-50 dark:bg-rose-950/30',
    accentBorder: 'border-rose-200/60 dark:border-rose-800/40',
    accentIcon: 'text-rose-600 dark:text-rose-400',
    badge: 'PDF',
    badgeClass: 'bg-rose-100 dark:bg-rose-900/50 text-rose-700 dark:text-rose-300',
  },
} as const

export function DownloadCard({ artifact }: DownloadCardProps) {
  const { type, filename, url, sizeReadable, rowCount, description } = artifact
  const cfg = typeConfig[type]

  return (
    <a
      href={url}
      download={filename}
      className={[
        'group/dl flex items-center gap-3 px-3.5 py-3 rounded-xl',
        'border transition-all duration-200',
        'border-gray-200/80 dark:border-gray-700/60',
        'bg-white dark:bg-gray-800/60',
        'hover:border-gray-300 dark:hover:border-gray-600',
        'hover:shadow-md dark:hover:shadow-lg dark:hover:shadow-black/20',
        'hover:-translate-y-px active:translate-y-0',
      ].join(' ')}
    >
      {/* File type icon with tinted background */}
      <div
        className={[
          'flex-shrink-0 flex items-center justify-center w-10 h-10 rounded-lg border',
          cfg.accentBg,
          cfg.accentBorder,
          'transition-transform duration-200 group-hover/dl:scale-105',
        ].join(' ')}
      >
        <cfg.Icon size={22} weight="duotone" className={cfg.accentIcon} />
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0">
        {/* Filename */}
        <div className="flex items-center gap-2 min-w-0">
          <span className="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
            {filename}
          </span>
          <span
            className={[
              'flex-shrink-0 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider rounded',
              cfg.badgeClass,
            ].join(' ')}
          >
            {cfg.badge}
          </span>
        </div>

        {/* Metadata row */}
        {(sizeReadable || rowCount !== undefined) && (
          <div className="flex items-center gap-1.5 mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {sizeReadable && <span>{sizeReadable}</span>}
            {sizeReadable && rowCount !== undefined && (
              <span className="w-0.5 h-0.5 rounded-full bg-gray-300 dark:bg-gray-600" />
            )}
            {rowCount !== undefined && (
              <span>{rowCount.toLocaleString()} rows</span>
            )}
          </div>
        )}

        {/* Description */}
        {description && (
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400 line-clamp-2 leading-relaxed">
            {description}
          </p>
        )}
      </div>

      {/* Download arrow */}
      <div className="flex-shrink-0 flex items-center justify-center w-8 h-8 rounded-lg text-gray-400 dark:text-gray-500 group-hover/dl:text-gray-600 dark:group-hover/dl:text-gray-300 group-hover/dl:bg-gray-100 dark:group-hover/dl:bg-gray-700/50 transition-all duration-200">
        <DownloadSimple
          size={18}
          weight="bold"
          className="transition-transform duration-200 group-hover/dl:translate-y-0.5"
        />
      </div>
    </a>
  )
}
