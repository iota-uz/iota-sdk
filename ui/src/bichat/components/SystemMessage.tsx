import { useCallback, useEffect, useRef, useState, lazy, Suspense } from 'react'
import { CaretDown, CaretUp, Check, Copy, ClockCounterClockwise } from '@phosphor-icons/react'
import { formatDistanceToNow } from 'date-fns'
import { useTranslation } from '../hooks/useTranslation'

const MarkdownRenderer = lazy(() =>
  import('./MarkdownRenderer').then((module) => ({ default: module.MarkdownRenderer }))
)

const COPY_FEEDBACK_MS = 2000
const COLLAPSED_HEIGHT = 160

interface SystemMessageProps {
  content: string
  createdAt: string
  onCopy?: (content: string) => Promise<void> | void
  hideActions?: boolean
  hideTimestamp?: boolean
}

export function SystemMessage({
  content,
  createdAt,
  onCopy,
  hideActions = false,
  hideTimestamp = false,
}: SystemMessageProps) {
  const { t } = useTranslation()
  const [isCopied, setIsCopied] = useState(false)
  const [isExpanded, setIsExpanded] = useState(false)
  const [isExpandable, setIsExpandable] = useState(false)
  const [contentHeight, setContentHeight] = useState<number | undefined>(undefined)
  const copyFeedbackTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const contentRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    return () => {
      if (copyFeedbackTimeoutRef.current) {
        clearTimeout(copyFeedbackTimeoutRef.current)
        copyFeedbackTimeoutRef.current = null
      }
    }
  }, [])

  useEffect(() => {
    const node = contentRef.current
    if (!node) return

    const measure = () => {
      const scrollH = node.scrollHeight
      setContentHeight(scrollH)
      if (!isExpanded) {
        setIsExpandable(scrollH > COLLAPSED_HEIGHT + 1)
      }
    }

    measure()

    if (typeof ResizeObserver !== 'undefined') {
      const observer = new ResizeObserver(measure)
      observer.observe(node)
      return () => observer.disconnect()
    }

    window.addEventListener('resize', measure)
    return () => window.removeEventListener('resize', measure)
  }, [content, isExpanded])

  const handleCopyClick = useCallback(async () => {
    try {
      if (onCopy) {
        await onCopy(content)
      } else {
        await navigator.clipboard.writeText(content)
      }

      setIsCopied(true)
      if (copyFeedbackTimeoutRef.current) {
        clearTimeout(copyFeedbackTimeoutRef.current)
      }
      copyFeedbackTimeoutRef.current = setTimeout(() => {
        setIsCopied(false)
        copyFeedbackTimeoutRef.current = null
      }, COPY_FEEDBACK_MS)
    } catch {
      setIsCopied(false)
    }
  }, [content, onCopy])

  const timestamp = formatDistanceToNow(new Date(createdAt), { addSuffix: true })

  const resolvedHeight = isExpanded ? contentHeight : COLLAPSED_HEIGHT

  return (
    <div className="flex justify-center">
      <div className="w-full max-w-3xl px-2 sm:px-4">
        <div className="relative overflow-hidden rounded-xl border border-gray-200/80 dark:border-gray-700/60 bg-gradient-to-b from-gray-50/80 to-gray-100/40 dark:from-gray-800/40 dark:to-gray-900/30 shadow-[0_1px_3px_rgba(0,0,0,0.04)] dark:shadow-[0_1px_3px_rgba(0,0,0,0.2)]">
          {/* Top accent line */}
          <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-gray-300/40 dark:via-gray-600/20 to-transparent" />

          {/* Header */}
          <div className="flex items-center gap-2 px-4 pt-3 pb-2">
            <div className="flex items-center gap-1.5 text-gray-400 dark:text-gray-500">
              <ClockCounterClockwise size={13} weight="bold" />
              <span className="text-[11px] font-semibold uppercase tracking-wider">
                Conversation summary
              </span>
            </div>

            <div className="flex-1" />

            {!hideActions && !hideTimestamp && (
              <span className="text-[11px] text-gray-400 dark:text-gray-500 tabular-nums">
                {timestamp}
              </span>
            )}

            {!hideActions && (
              <button
                onClick={handleCopyClick}
                className={`
                  cursor-pointer -mr-1 p-1 rounded-md transition-all duration-150
                  focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50
                  ${isCopied
                    ? 'text-green-600 dark:text-green-400'
                    : 'text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-200/50 dark:hover:bg-gray-700/40'
                  }
                `}
                aria-label="Copy message"
                title={isCopied ? t('message.copied') : t('message.copy')}
              >
                {isCopied ? <Check size={13} weight="bold" /> : <Copy size={13} weight="regular" />}
              </button>
            )}
          </div>

          {/* Divider */}
          <div className="mx-4 border-t border-gray-200/60 dark:border-gray-700/40" />

          {/* Content area */}
          <div className="relative">
            <div
              ref={contentRef}
              className="px-4 pt-3 text-[13px] leading-relaxed text-gray-600 dark:text-gray-400 overflow-hidden transition-[max-height] duration-300 ease-in-out"
              style={{ maxHeight: resolvedHeight ? `${resolvedHeight}px` : undefined }}
            >
              <Suspense
                fallback={
                  <div className="flex items-center gap-2 text-sm text-gray-400 dark:text-gray-500 py-2">
                    <div className="w-3.5 h-3.5 border-[1.5px] border-gray-300 dark:border-gray-600 border-t-transparent rounded-full animate-spin" />
                    <span className="text-xs">Loading summary...</span>
                  </div>
                }
              >
                <MarkdownRenderer content={content} sendDisabled />
              </Suspense>
            </div>

            {/* Fade overlay when collapsed */}
            {isExpandable && !isExpanded && (
              <div className="pointer-events-none absolute inset-x-0 bottom-0 h-20 bg-gradient-to-t from-gray-100/95 via-gray-100/60 to-transparent dark:from-gray-900/95 dark:via-gray-900/50" />
            )}
          </div>

          {/* Expand/collapse toggle */}
          {isExpandable && (
            <div className="relative px-4 pb-3 pt-1 flex justify-center">
              <button
                type="button"
                onClick={() => setIsExpanded((prev) => !prev)}
                aria-expanded={isExpanded}
                className="cursor-pointer group/toggle inline-flex items-center gap-1 px-3 py-1 rounded-full text-[11px] font-medium text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-200/60 dark:hover:bg-gray-700/50 transition-colors duration-150"
              >
                <span>{isExpanded ? 'Show less' : 'Show more'}</span>
                <CaretDown
                  size={11}
                  weight="bold"
                  className={`transition-transform duration-300 ${isExpanded ? 'rotate-180' : ''}`}
                />
              </button>
            </div>
          )}

          {/* Bottom spacing when not expandable */}
          {!isExpandable && <div className="pb-3" />}
        </div>
      </div>
    </div>
  )
}

export default SystemMessage
