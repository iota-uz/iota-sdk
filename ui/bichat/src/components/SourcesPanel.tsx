/**
 * SourcesPanel component
 * Displays citations and sources
 */

import { useState } from 'react'
import { Citation } from '../types'

interface SourcesPanelProps {
  citations: Citation[]
}

export function SourcesPanel({ citations }: SourcesPanelProps) {
  const [expanded, setExpanded] = useState(false)

  if (!citations || citations.length === 0) {
    return null
  }

  return (
    <div className="mt-4 border-t border-[var(--bichat-border)] pt-3">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-2 text-sm text-gray-600 hover:text-gray-800 transition-colors"
        aria-expanded={expanded}
      >
        <svg
          className={`w-4 h-4 transition-transform ${expanded ? 'rotate-90' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M9 5l7 7-7 7"
          />
        </svg>
        <span>{citations.length} {citations.length === 1 ? 'source' : 'sources'}</span>
      </button>
      {expanded && (
        <div className="mt-2 space-y-2">
          {citations.map((citation, index) => (
            <div
              key={citation.id}
              className="p-3 bg-gray-50 rounded-lg text-sm"
            >
              <div className="flex items-start gap-2">
                <span className="flex-shrink-0 w-5 h-5 bg-[var(--bichat-primary)] text-white rounded-full flex items-center justify-center text-xs">
                  {index + 1}
                </span>
                <div className="flex-1">
                  <div className="font-medium text-gray-900">{citation.source}</div>
                  {citation.url && (
                    <a
                      href={citation.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-[var(--bichat-primary)] hover:underline"
                    >
                      {citation.url}
                    </a>
                  )}
                  {citation.excerpt && (
                    <div className="mt-1 text-gray-600 italic">"{citation.excerpt}"</div>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
