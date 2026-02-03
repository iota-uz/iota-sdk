/**
 * CodeOutputsPanel Component
 * Displays code interpreter outputs (images, text, errors)
 */

import type { CodeOutput, ImageAttachment } from '../types'

interface CodeOutputsPanelProps {
  outputs: CodeOutput[]
}

export default function CodeOutputsPanel({ outputs }: CodeOutputsPanelProps) {
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
              <img
                src={(output.content as ImageAttachment).preview}
                alt="Code output"
                className="max-w-full rounded border border-gray-300 dark:border-gray-600"
              />
            )}
            {output.type === 'text' && (
              <pre className="text-xs bg-white dark:bg-gray-800 p-2 rounded overflow-x-auto border border-gray-200 dark:border-gray-700">
                <code className="text-gray-900 dark:text-gray-100">
                  {output.content as string}
                </code>
              </pre>
            )}
            {output.type === 'error' && (
              <div className="text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 p-2 rounded border border-red-200 dark:border-red-800">
                {output.content as string}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
