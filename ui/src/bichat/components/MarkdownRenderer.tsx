/**
 * MarkdownRenderer Component
 * Renders markdown with syntax highlighting, citations, and table export
 *
 * Features:
 * - Lazy-loaded CodeBlock for bundle optimization
 * - Citation processing (converts raw markers to numbered references)
 * - Table export functionality
 * - Custom CSS class names for styling flexibility
 * - GFM (GitHub Flavored Markdown) support
 */

import { memo, lazy, Suspense, useMemo } from 'react'
import ReactMarkdown, { Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { processCitations } from '../utils/citationProcessor'
import type { Citation } from '../types'
import { TableWithExport } from './TableWithExport'

// Lazy load CodeBlock for bundle optimization
const CodeBlock = lazy(() => import('./CodeBlock').then((module) => ({ default: module.CodeBlock })))

interface MarkdownRendererProps {
  /** Markdown content to render */
  content: string
  /** Optional citations to process and display */
  citations?: Citation[] | null
  /** Optional function to send messages (enables table export) */
  sendMessage?: (content: string) => void
  /** Whether message sending is disabled */
  sendDisabled?: boolean
  /** Copy button label for code blocks */
  copyLabel?: string
  /** Copied confirmation label for code blocks */
  copiedLabel?: string
  /** Export button label for tables */
  exportLabel?: string
}

interface CodeProps {
  inline?: boolean
  className?: string
  children?: React.ReactNode
}

function MarkdownRenderer({
  content,
  citations,
  sendMessage,
  sendDisabled = false,
  copyLabel = 'Copy',
  copiedLabel = 'Copied!',
  exportLabel = 'Export',
}: MarkdownRendererProps) {
  // Process citations to replace raw markers with [1], [2], etc.
  const processed = useMemo(() => {
    return processCitations(content, citations)
  }, [content, citations])

  const components: Components = {
    // Remove <pre> wrapper for code blocks - CodeBlock provides its own container
    pre: ({ children }) => <>{children}</>,
    code({ inline, className, children }: CodeProps) {
      const match = /language-(\w+)/.exec(className || '')
      const language = match ? match[1] : ''
      const value = String(children).replace(/\n$/, '')

      // Treat as inline if explicitly inline OR no className (no language = likely inline)
      // This prevents rendering <div> inside <p> for ambiguous cases
      const isInline = inline === true || !className

      if (isInline) {
        return (
          <code className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-800 text-red-600 dark:text-red-400 rounded text-sm font-mono">
            {value}
          </code>
        )
      }

      // Block code - rendered outside of <p> context due to pre handler above
      return (
        <Suspense
          fallback={
            <pre className="bg-gray-100 dark:bg-gray-800 rounded-lg p-4 overflow-x-auto my-4">
              <code className="text-sm font-mono">{value}</code>
            </pre>
          }
        >
          <CodeBlock
            language={language}
            value={value}
            inline={false}
            copyLabel={copyLabel}
            copiedLabel={copiedLabel}
          />
        </Suspense>
      )
    },
    p: ({ children }) => <p className="markdown-p my-2">{children}</p>,
    a: ({ href, children }) => (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        className="markdown-link text-[var(--bichat-primary)] hover:underline"
      >
        {children}
      </a>
    ),
    h1: ({ children }) => <h1 className="markdown-h1 text-2xl font-bold mt-6 mb-3">{children}</h1>,
    h2: ({ children }) => <h2 className="markdown-h2 text-xl font-bold mt-5 mb-2">{children}</h2>,
    h3: ({ children }) => <h3 className="markdown-h3 text-lg font-semibold mt-4 mb-2">{children}</h3>,
    h4: ({ children }) => <h4 className="markdown-h4 text-base font-semibold mt-3 mb-1">{children}</h4>,
    h5: ({ children }) => <h5 className="markdown-h5 text-sm font-semibold mt-2 mb-1">{children}</h5>,
    h6: ({ children }) => <h6 className="markdown-h6 text-sm font-medium mt-2 mb-1">{children}</h6>,
    ul: ({ children }) => <ul className="markdown-ul list-disc list-inside my-2 space-y-1">{children}</ul>,
    ol: ({ children }) => <ol className="markdown-ol list-decimal list-inside my-2 space-y-1">{children}</ol>,
    li: ({ children }) => <li className="markdown-li">{children}</li>,
    blockquote: ({ children }) => (
      <blockquote className="markdown-blockquote border-l-4 border-gray-300 dark:border-gray-600 pl-4 my-2 italic text-gray-600 dark:text-gray-400">
        {children}
      </blockquote>
    ),
    table: ({ children }) => (
      <TableWithExport
        sendMessage={sendMessage}
        disabled={sendDisabled}
        exportLabel={exportLabel}
      >
        {children}
      </TableWithExport>
    ),
    thead: ({ children }) => (
      <thead className="markdown-thead bg-gray-100 dark:bg-gray-800">{children}</thead>
    ),
    tbody: ({ children }) => <tbody className="markdown-tbody">{children}</tbody>,
    tr: ({ children }) => (
      <tr className="markdown-tr border-b border-gray-200 dark:border-gray-700">{children}</tr>
    ),
    th: ({ children }) => (
      <th className="markdown-th px-3 py-2 text-left text-sm font-semibold text-gray-700 dark:text-gray-300">
        {children}
      </th>
    ),
    td: ({ children }) => (
      <td className="markdown-td px-3 py-2 text-sm text-gray-600 dark:text-gray-400">{children}</td>
    ),
    hr: () => <hr className="markdown-hr my-4 border-gray-200 dark:border-gray-700" />,
    strong: ({ children }) => <strong className="markdown-strong font-semibold">{children}</strong>,
    em: ({ children }) => <em className="markdown-em italic">{children}</em>,
  }

  return (
    <div className="markdown-content">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {processed.content}
      </ReactMarkdown>
    </div>
  )
}

const MemoizedMarkdownRenderer = memo(MarkdownRenderer)
MemoizedMarkdownRenderer.displayName = 'MarkdownRenderer'

export { MemoizedMarkdownRenderer as MarkdownRenderer }
export default MemoizedMarkdownRenderer
