/**
 * CodeOutputsPanel Component
 * Displays code interpreter outputs (images, text, errors)
 *
 * Output types:
 * - image: Base64-encoded image data (content is base64 string, mimeType specifies format)
 * - text: Plain text output from code execution
 * - error: Error messages from failed code execution
 */

import { Download } from '@phosphor-icons/react'
import type { CodeOutput } from '../types'
import { formatFileSize } from '../utils/fileUtils'

interface CodeOutputsPanelProps {
  outputs: CodeOutput[]
}

function CodeOutputsPanel({ outputs }: CodeOutputsPanelProps) {
  if (!outputs || outputs.length === 0) return null

  return (
    <div className="mb-2 p-3 bg-gray-50 dark:bg-gray-900/50 rounded-lg border border-gray-200 dark:border-gray-700">
      <div className="text-xs font-semibold text-gray-600 dark:text-gray-400 mb-2">
        Code Output
      </div>
      <div className="space-y-2">
        {outputs.map((output, index) => (
          <div key={index}>
            {output.type === 'image' && (
              <div className="relative group">
                <img
                  src={
                    output.content.startsWith('data:')
                      ? output.content
                      : `data:${output.mimeType || 'image/png'};base64,${output.content}`
                  }
                  alt={output.filename || 'Code output'}
                  className="max-w-full rounded border border-gray-300 dark:border-gray-600"
                />
                {/* File info overlay */}
                {output.filename && (
                  <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/60 to-transparent p-2 opacity-0 group-hover:opacity-100 transition-opacity">
                    <div className="flex items-center justify-between text-white text-xs">
                      <span className="truncate">{output.filename}</span>
                      {output.sizeBytes && (
                        <span className="text-gray-300">{formatFileSize(output.sizeBytes)}</span>
                      )}
                    </div>
                  </div>
                )}
              </div>
            )}
            {output.type === 'text' && (
              <div>
                <pre className="text-xs bg-white dark:bg-gray-800 p-2 rounded overflow-x-auto border border-gray-200 dark:border-gray-700">
                  <code className="text-gray-900 dark:text-gray-100">{output.content}</code>
                </pre>
                {/* File download link */}
                {output.filename && (
                  <div className="flex items-center gap-2 mt-1 text-xs">
                    <a
                      href={`data:${output.mimeType || 'text/plain'};base64,${btoa(output.content)}`}
                      download={output.filename}
                      className="flex items-center gap-1 text-blue-600 dark:text-blue-400 hover:underline"
                    >
                      <Download size={12} weight="bold" />
                      {output.filename}
                    </a>
                    {output.sizeBytes && (
                      <span className="text-gray-500 dark:text-gray-400">
                        ({formatFileSize(output.sizeBytes)})
                      </span>
                    )}
                  </div>
                )}
              </div>
            )}
            {output.type === 'error' && (
              <div className="text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 p-2 rounded border border-red-200 dark:border-red-800">
                <div className="font-semibold mb-1">Error</div>
                <pre className="whitespace-pre-wrap">{output.content}</pre>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

export { CodeOutputsPanel }
export default CodeOutputsPanel
