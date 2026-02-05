/**
 * CodeBlock Component
 * Syntax highlighted code blocks with copy functionality and dark mode support
 */

import { useState, useEffect, memo } from 'react'
import { Copy, Check } from '@phosphor-icons/react'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { vscDarkPlus, vs } from 'react-syntax-highlighter/dist/esm/styles/prism'

interface CodeBlockProps {
  /** Programming language for syntax highlighting */
  language: string
  /** Code content to display */
  value: string
  /** Whether to render as inline code */
  inline?: boolean
  /** Copy button label (defaults to "Copy") */
  copyLabel?: string
  /** Copied confirmation label (defaults to "Copied!") */
  copiedLabel?: string
}

// Get initial dark mode state from DOM
const getInitialDarkMode = () => {
  if (typeof document === 'undefined') return false
  return document.documentElement.classList.contains('dark')
}

// Language aliases for normalization
const languageMap: Record<string, string> = {
  js: 'javascript',
  ts: 'typescript',
  jsx: 'jsx',
  tsx: 'tsx',
  py: 'python',
  rb: 'ruby',
  yml: 'yaml',
  yaml: 'yaml',
  sh: 'bash',
  bash: 'bash',
  json: 'json',
  xml: 'xml',
  html: 'html',
  css: 'css',
  sql: 'sql',
  go: 'go',
  java: 'java',
  cpp: 'cpp',
  c: 'c',
  csharp: 'csharp',
  php: 'php',
}

function normalizeLanguage(lang: string): string {
  if (!lang) return 'text'
  return languageMap[lang.toLowerCase()] || lang.toLowerCase()
}

function CodeBlock({
  language,
  value,
  inline,
  copyLabel = 'Copy',
  copiedLabel = 'Copied!',
}: CodeBlockProps) {
  const [copied, setCopied] = useState(false)
  const [isDarkMode, setIsDarkMode] = useState(getInitialDarkMode)
  const [isLoaded, setIsLoaded] = useState(false)

  const normalizedLanguage = normalizeLanguage(language)

  // Detect dark mode and mark as loaded
  useEffect(() => {
    setIsDarkMode(document.documentElement.classList.contains('dark'))
    setIsLoaded(true)

    // Watch for dark mode changes
    const observer = new MutationObserver(() => {
      setIsDarkMode(document.documentElement.classList.contains('dark'))
    })

    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    })

    return () => observer.disconnect()
  }, [])

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch (err) {
      console.error('Failed to copy:', err)
    }
  }

  // Inline code styling
  if (inline) {
    return (
      <code className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-800 text-red-600 dark:text-red-400 rounded text-sm font-mono">
        {value}
      </code>
    )
  }

  // Loading fallback
  if (!isLoaded) {
    return (
      <pre className="bg-gray-100 dark:bg-gray-800 rounded-lg p-4 overflow-x-auto my-4 border border-gray-300 dark:border-gray-700">
        <code className="text-gray-700 dark:text-gray-300 text-sm font-mono">{value}</code>
      </pre>
    )
  }

  // Code block with syntax highlighting
  return (
    <div className="relative group my-4 rounded-lg overflow-hidden border border-gray-300 dark:border-gray-700">
      {/* Language label and copy button */}
      <div className="flex items-center justify-between px-4 py-2 bg-gray-200 dark:bg-gray-800 border-b border-gray-300 dark:border-gray-700">
        <span className="text-xs text-gray-600 dark:text-gray-400 font-medium uppercase">
          {normalizedLanguage}
        </span>
        <button
          onClick={handleCopy}
          className="text-xs text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors flex items-center gap-1.5"
          title={copyLabel}
        >
          {copied ? (
            <>
              <Check size={16} className="w-4 h-4" />
              <span>{copiedLabel}</span>
            </>
          ) : (
            <>
              <Copy size={16} className="w-4 h-4" />
              <span>{copyLabel}</span>
            </>
          )}
        </button>
      </div>

      {/* Code content */}
      <SyntaxHighlighter
        language={normalizedLanguage}
        style={isDarkMode ? vscDarkPlus : vs}
        customStyle={{
          margin: 0,
          borderRadius: 0,
          fontSize: '0.875rem',
          lineHeight: '1.5',
          padding: '1rem',
        }}
        showLineNumbers={false}
        wrapLines={true}
        codeTagProps={{
          style: {
            fontFamily: '"JetBrains Mono", "Fira Code", "Menlo", monospace',
          },
        }}
      >
        {value}
      </SyntaxHighlighter>
    </div>
  )
}

const MemoizedCodeBlock = memo(CodeBlock)
MemoizedCodeBlock.displayName = 'CodeBlock'

export { MemoizedCodeBlock as CodeBlock }
export default MemoizedCodeBlock
