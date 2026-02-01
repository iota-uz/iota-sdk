/**
 * DownloadCard component
 * Displays downloadable artifacts (Excel, PDF)
 */

import { Artifact } from '../types'

interface DownloadCardProps {
  artifact: Artifact
}

export function DownloadCard({ artifact }: DownloadCardProps) {
  const { type, filename, url, sizeReadable, rowCount, description } = artifact

  const icon =
    type === 'excel' ? (
      <svg className="w-8 h-8 text-green-600" fill="currentColor" viewBox="0 0 20 20">
        <path d="M9 2a2 2 0 00-2 2v8a2 2 0 002 2h6a2 2 0 002-2V6.414A2 2 0 0016.414 5L14 2.586A2 2 0 0012.586 2H9z" />
        <path d="M3 8a2 2 0 012-2v10h8a2 2 0 01-2 2H5a2 2 0 01-2-2V8z" />
      </svg>
    ) : (
      <svg className="w-8 h-8 text-red-600" fill="currentColor" viewBox="0 0 20 20">
        <path
          fillRule="evenodd"
          d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z"
          clipRule="evenodd"
        />
      </svg>
    )

  return (
    <a
      href={url}
      download={filename}
      className="flex items-center gap-3 p-4 border border-[var(--bichat-border)] rounded-lg hover:bg-gray-50 transition-colors"
    >
      <div>{icon}</div>
      <div className="flex-1 min-w-0">
        <div className="font-medium text-gray-900 truncate">{filename}</div>
        <div className="flex items-center gap-2 text-sm text-gray-600">
          {sizeReadable && <span>{sizeReadable}</span>}
          {rowCount !== undefined && (
            <>
              <span>•</span>
              <span>{rowCount} rows</span>
            </>
          )}
        </div>
        {description && <div className="text-sm text-gray-600 mt-1">{description}</div>}
      </div>
      <svg
        className="w-5 h-5 text-gray-400"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
        />
      </svg>
    </a>
  )
}
