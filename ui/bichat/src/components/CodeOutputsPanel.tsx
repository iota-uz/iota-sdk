/**
 * CodeOutputsPanel Component
 * Displays code interpreter outputs (images, text, errors)
 */

import type { CodeOutput, CodeOutputFile, ImageAttachment } from '../types'
import { useTranslation } from '../hooks/useTranslation'

interface CodeOutputsPanelProps {
  outputs: CodeOutput[]
}

export default function CodeOutputsPanel({ outputs }: CodeOutputsPanelProps) {
  const { t } = useTranslation()

  if (!outputs || outputs.length === 0) return null

  const formatBytes = (bytes: number) => {
    if (!Number.isFinite(bytes) || bytes <= 0) return ''
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
    const value = bytes / Math.pow(1024, i)
    const digits = i === 0 ? 0 : value < 10 ? 2 : value < 100 ? 1 : 0
    return `${value.toFixed(digits)} ${units[i]}`
  }

  return (
    <div className="mb-2 p-3 bg-gray-50 dark:bg-gray-900/50 rounded-lg border border-gray-200 dark:border-gray-700">
      <div className="text-xs font-semibold text-gray-600 dark:text-gray-400 mb-2">
        {t('codeOutputs.title')}
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
            {output.type === 'file' && (
              <a
                href={(output.content as CodeOutputFile).url}
                target="_blank"
                rel="noopener noreferrer"
                className="block p-2 bg-white dark:bg-gray-800 rounded border border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600 transition-colors"
              >
                <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {(output.content as CodeOutputFile).name}
                </div>
                <div className="text-xs text-gray-600 dark:text-gray-400">
                  {(output.content as CodeOutputFile).mimeType}
                  {formatBytes((output.content as CodeOutputFile).size)
                    ? ` • ${formatBytes((output.content as CodeOutputFile).size)}`
                    : ''}
                </div>
              </a>
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
