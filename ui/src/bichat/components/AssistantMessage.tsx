/**
 * AssistantMessage Component (Layer 3 Composite)
 * Styled component with slot-based customization for assistant messages
 */

import { useState, useCallback, lazy, Suspense, useRef, useEffect, type ReactNode } from 'react'
import { Check, Copy, ArrowsClockwise } from '@phosphor-icons/react'
import { formatDistanceToNow } from 'date-fns'
import CodeOutputsPanel from './CodeOutputsPanel'
import StreamingCursor from './StreamingCursor'
import { ChartCard } from './ChartCard'
import { SourcesPanel } from './SourcesPanel'
import { DownloadCard } from './DownloadCard'
import { InlineQuestionForm } from './InlineQuestionForm'
import type { AssistantTurn, Citation, ChartData, Artifact, CodeOutput, PendingQuestion } from '../types'
import { DebugPanel } from './DebugPanel'
import { useTranslation } from '../hooks/useTranslation'

const MarkdownRenderer = lazy(() =>
  import('./MarkdownRenderer').then((module) => ({ default: module.MarkdownRenderer }))
)

/* -------------------------------------------------------------------------------------------------
 * Slot Props Types
 * -----------------------------------------------------------------------------------------------*/

export interface AssistantMessageAvatarSlotProps {
  /** Default text */
  text: string
}

export interface AssistantMessageContentSlotProps {
  /** Message content (markdown) */
  content: string
  /** Citations */
  citations?: Citation[]
  /** Whether streaming is active */
  isStreaming: boolean
}

export interface AssistantMessageSourcesSlotProps {
  /** Citations to display */
  citations: Citation[]
}

export interface AssistantMessageChartsSlotProps {
  /** Chart data */
  chartData: ChartData
}

export interface AssistantMessageCodeOutputsSlotProps {
  /** Code execution outputs */
  outputs: CodeOutput[]
}

export interface AssistantMessageArtifactsSlotProps {
  /** Downloadable artifacts */
  artifacts: Artifact[]
}

export interface AssistantMessageActionsSlotProps {
  /** Copy content to clipboard */
  onCopy: () => void
  /** Regenerate response */
  onRegenerate?: () => void
  /** Formatted timestamp */
  timestamp: string
  /** Whether copy action is available */
  canCopy: boolean
  /** Whether regenerate action is available */
  canRegenerate: boolean
}

export interface AssistantMessageExplanationSlotProps {
  /** Explanation content (markdown) */
  explanation: string
  /** Whether expanded */
  isExpanded: boolean
  /** Toggle expansion */
  onToggle: () => void
}

/* -------------------------------------------------------------------------------------------------
 * Component Types
 * -----------------------------------------------------------------------------------------------*/

export interface AssistantMessageSlots {
  /** Custom avatar renderer */
  avatar?: ReactNode | ((props: AssistantMessageAvatarSlotProps) => ReactNode)
  /** Custom content renderer */
  content?: ReactNode | ((props: AssistantMessageContentSlotProps) => ReactNode)
  /** Custom sources renderer */
  sources?: ReactNode | ((props: AssistantMessageSourcesSlotProps) => ReactNode)
  /** Custom charts renderer */
  charts?: ReactNode | ((props: AssistantMessageChartsSlotProps) => ReactNode)
  /** Custom code outputs renderer */
  codeOutputs?: ReactNode | ((props: AssistantMessageCodeOutputsSlotProps) => ReactNode)
  /** Custom artifacts renderer */
  artifacts?: ReactNode | ((props: AssistantMessageArtifactsSlotProps) => ReactNode)
  /** Custom actions renderer */
  actions?: ReactNode | ((props: AssistantMessageActionsSlotProps) => ReactNode)
  /** Custom explanation renderer */
  explanation?: ReactNode | ((props: AssistantMessageExplanationSlotProps) => ReactNode)
}

export interface AssistantMessageClassNames {
  /** Root container */
  root?: string
  /** Inner content wrapper */
  wrapper?: string
  /** Avatar container */
  avatar?: string
  /** Message bubble */
  bubble?: string
  /** Code outputs container */
  codeOutputs?: string
  /** Charts container */
  charts?: string
  /** Artifacts container */
  artifacts?: string
  /** Sources container */
  sources?: string
  /** Explanation container */
  explanation?: string
  /** Actions container */
  actions?: string
  /** Action button */
  actionButton?: string
  /** Timestamp */
  timestamp?: string
}

export interface AssistantMessageProps {
  /** Assistant turn data */
  turn: AssistantTurn
  /** Turn ID for regenerate operations */
  turnId?: string
  /** When true, this is the last turn (Regenerate button shown only on last assistant message) */
  isLastTurn?: boolean
  /** Whether response is being streamed */
  isStreaming?: boolean
  /** Pending question for HITL */
  pendingQuestion?: PendingQuestion | null
  /** Slot overrides */
  slots?: AssistantMessageSlots
  /** Class name overrides */
  classNames?: AssistantMessageClassNames
  /** Copy handler */
  onCopy?: (content: string) => Promise<void> | void
  /** Regenerate handler */
  onRegenerate?: (turnId: string) => Promise<void> | void
  /** Send message handler (for markdown links) */
  onSendMessage?: (content: string) => void
  /** Whether sending is disabled */
  sendDisabled?: boolean
  /** Hide avatar */
  hideAvatar?: boolean
  /** Hide actions */
  hideActions?: boolean
  /** Hide timestamp */
  hideTimestamp?: boolean
  /** Show debug panel */
  showDebug?: boolean
}

const COPY_FEEDBACK_MS = 2000

/* -------------------------------------------------------------------------------------------------
 * Default Styles
 * -----------------------------------------------------------------------------------------------*/

const defaultClassNames: Required<AssistantMessageClassNames> = {
  root: 'flex gap-3 group',
  wrapper: 'flex-1 flex flex-col gap-3 max-w-[85%]',
  avatar: 'flex-shrink-0 w-8 h-8 rounded-full bg-primary-600 flex items-center justify-center text-white font-medium text-xs',
  bubble: 'bg-white dark:bg-gray-800 rounded-2xl rounded-bl-sm px-4 py-3 shadow-sm',
  codeOutputs: '',
  charts: 'mb-1 w-full',
  artifacts: 'mb-1 flex flex-wrap gap-2',
  sources: '',
  explanation: 'mt-4 border-t border-gray-100 dark:border-gray-700 pt-4',
  actions: 'flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity duration-150',
  actionButton: 'cursor-pointer p-2 text-gray-500 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 active:bg-gray-200 dark:active:bg-gray-700 rounded-md transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50',
  timestamp: 'text-xs text-gray-400 dark:text-gray-500 mr-1',
}

function mergeClassNames(
  defaults: Required<AssistantMessageClassNames>,
  overrides?: AssistantMessageClassNames
): Required<AssistantMessageClassNames> {
  if (!overrides) return defaults
  return {
    root: overrides.root ?? defaults.root,
    wrapper: overrides.wrapper ?? defaults.wrapper,
    avatar: overrides.avatar ?? defaults.avatar,
    bubble: overrides.bubble ?? defaults.bubble,
    codeOutputs: overrides.codeOutputs ?? defaults.codeOutputs,
    charts: overrides.charts ?? defaults.charts,
    artifacts: overrides.artifacts ?? defaults.artifacts,
    sources: overrides.sources ?? defaults.sources,
    explanation: overrides.explanation ?? defaults.explanation,
    actions: overrides.actions ?? defaults.actions,
    actionButton: overrides.actionButton ?? defaults.actionButton,
    timestamp: overrides.timestamp ?? defaults.timestamp,
  }
}

/* -------------------------------------------------------------------------------------------------
 * Component
 * -----------------------------------------------------------------------------------------------*/

export function AssistantMessage({
  turn,
  turnId,
  isLastTurn = false,
  isStreaming = false,
  pendingQuestion,
  slots,
  classNames: classNameOverrides,
  onCopy,
  onRegenerate,
  onSendMessage,
  sendDisabled = false,
  hideAvatar = false,
  hideActions = false,
  hideTimestamp = false,
  showDebug = false,
}: AssistantMessageProps) {
  const { t } = useTranslation()
  const [explanationExpanded, setExplanationExpanded] = useState(false)
  const [isCopied, setIsCopied] = useState(false)
  const copyFeedbackTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const classes = mergeClassNames(defaultClassNames, classNameOverrides)
  const isSystemMessage = turn.role === 'system'
  const avatarClassName = isSystemMessage
    ? 'flex-shrink-0 w-8 h-8 rounded-full bg-gray-500 dark:bg-gray-600 flex items-center justify-center text-white font-medium text-xs'
    : classes.avatar
  const bubbleClassName = isSystemMessage
    ? 'bg-gray-50 dark:bg-gray-900/40 rounded-2xl px-4 py-3 shadow-sm'
    : classes.bubble

  useEffect(() => {
    return () => {
      if (copyFeedbackTimeoutRef.current) {
        clearTimeout(copyFeedbackTimeoutRef.current)
        copyFeedbackTimeoutRef.current = null
      }
    }
  }, [])

  const hasContent = turn.content?.trim().length > 0
  const hasExplanation = !!turn.explanation?.trim()
  const hasPendingQuestion =
    !!pendingQuestion &&
    pendingQuestion.status === 'PENDING' &&
    pendingQuestion.turnId === turnId

  const handleCopyClick = useCallback(async () => {
    try {
      if (onCopy) {
        await onCopy(turn.content)
      } else {
        await navigator.clipboard.writeText(turn.content)
      }

      setIsCopied(true)
      if (copyFeedbackTimeoutRef.current) {
        clearTimeout(copyFeedbackTimeoutRef.current)
      }
      copyFeedbackTimeoutRef.current = setTimeout(() => {
        setIsCopied(false)
        copyFeedbackTimeoutRef.current = null
      }, COPY_FEEDBACK_MS)
    } catch (err) {
      setIsCopied(false)
      console.error('Failed to copy:', err)
    }
  }, [onCopy, turn.content])

  const handleRegenerateClick = useCallback(async () => {
    if (onRegenerate && turnId) {
      await onRegenerate(turnId)
    }
  }, [onRegenerate, turnId])

  const timestamp = formatDistanceToNow(new Date(turn.createdAt), { addSuffix: true })

  // Slot props
  const avatarSlotProps: AssistantMessageAvatarSlotProps = { text: isSystemMessage ? 'SYS' : 'AI' }
  const contentSlotProps: AssistantMessageContentSlotProps = {
    content: turn.content,
    citations: turn.citations,
    isStreaming,
  }
  const sourcesSlotProps: AssistantMessageSourcesSlotProps = {
    citations: turn.citations || [],
  }
  const chartsSlotProps: AssistantMessageChartsSlotProps = {
    chartData: turn.chartData!,
  }
  const codeOutputsSlotProps: AssistantMessageCodeOutputsSlotProps = {
    outputs: turn.codeOutputs || [],
  }
  const artifactsSlotProps: AssistantMessageArtifactsSlotProps = {
    artifacts: turn.artifacts || [],
  }
  const actionsSlotProps: AssistantMessageActionsSlotProps = {
    onCopy: handleCopyClick,
    onRegenerate: onRegenerate && turnId && !isSystemMessage && isLastTurn ? handleRegenerateClick : undefined,
    timestamp,
    canCopy: hasContent,
    canRegenerate: !!onRegenerate && !!turnId && !isSystemMessage && isLastTurn,
  }
  const explanationSlotProps: AssistantMessageExplanationSlotProps = {
    explanation: turn.explanation || '',
    isExpanded: explanationExpanded,
    onToggle: () => setExplanationExpanded(!explanationExpanded),
  }

  // Render helpers
  const renderSlot = <T,>(
    slot: ReactNode | ((props: T) => ReactNode) | undefined,
    props: T,
    defaultContent: ReactNode
  ): ReactNode => {
    if (slot === undefined) return defaultContent
    if (typeof slot === 'function') return slot(props)
    return slot
  }

  return (
    <div className={classes.root}>
      {/* Avatar */}
      {!hideAvatar && (
        <div className={avatarClassName}>
          {renderSlot(slots?.avatar, avatarSlotProps, isSystemMessage ? 'SYS' : 'AI')}
        </div>
      )}

      <div className={classes.wrapper}>
        {/* Code outputs */}
        {turn.codeOutputs && turn.codeOutputs.length > 0 && (
          <div className={classes.codeOutputs}>
            {renderSlot(
              slots?.codeOutputs,
              codeOutputsSlotProps,
              <CodeOutputsPanel outputs={turn.codeOutputs} />
            )}
          </div>
        )}

        {/* Charts */}
        {turn.chartData && (
          <div className={classes.charts}>
            {renderSlot(slots?.charts, chartsSlotProps, <ChartCard chartData={turn.chartData} />)}
          </div>
        )}

        {/* Message bubble */}
        {hasContent && (
          <div className={bubbleClassName}>
            {renderSlot(
              slots?.content,
              contentSlotProps,
              <Suspense
                fallback={
                  <div className="flex items-center gap-2 text-sm text-gray-400 dark:text-gray-500">
                    <div className="w-4 h-4 border-2 border-gray-300 dark:border-gray-600 border-t-transparent rounded-full animate-spin" />
                    Loading...
                  </div>
                }
              >
                <MarkdownRenderer
                  content={turn.content}
                  citations={turn.citations}
                  sendMessage={onSendMessage}
                  sendDisabled={sendDisabled || isStreaming}
                />
              </Suspense>
            )}

            {/* Streaming cursor */}
            {isStreaming && <StreamingCursor />}

            {/* Sources panel */}
            {turn.citations && turn.citations.length > 0 && (
              <div className={classes.sources}>
                {renderSlot(
                  slots?.sources,
                  sourcesSlotProps,
                  <SourcesPanel citations={turn.citations} />
                )}
              </div>
            )}

            {/* Explanation section */}
            {hasExplanation && (
              <div className={classes.explanation}>
                {renderSlot(
                  slots?.explanation,
                  explanationSlotProps,
                  <>
                    <button
                      type="button"
                      onClick={() => setExplanationExpanded(!explanationExpanded)}
                      className="cursor-pointer flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 rounded-md p-1 -m-1"
                      aria-expanded={explanationExpanded}
                    >
                      <svg
                        className={`w-4 h-4 transition-transform duration-150 ${explanationExpanded ? 'rotate-90' : ''}`}
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
                      <span className="font-medium">{t('assistant.explanation')}</span>
                    </button>
                    {explanationExpanded && (
                      <div className="pt-3 text-sm text-gray-600 dark:text-gray-400">
                        <Suspense fallback={<div>Loading...</div>}>
                          <MarkdownRenderer content={turn.explanation!} />
                        </Suspense>
                      </div>
                    )}
                  </>
                )}
              </div>
            )}

            {showDebug && <DebugPanel trace={turn.debug} />}
          </div>
        )}

        {/* Artifacts */}
        {turn.artifacts && turn.artifacts.length > 0 && (
          <div className={classes.artifacts}>
            {renderSlot(
              slots?.artifacts,
              artifactsSlotProps,
              turn.artifacts.map((artifact, index) => (
                <DownloadCard key={`${artifact.filename}-${index}`} artifact={artifact} />
              ))
            )}
          </div>
        )}

        {/* Inline Question Form */}
        {hasPendingQuestion && pendingQuestion && (
          <InlineQuestionForm pendingQuestion={pendingQuestion} />
        )}

        {/* Actions */}
        {hasContent && !hideActions && (
          <div className={`${classes.actions} ${isCopied ? 'opacity-100' : ''}`}>
            {renderSlot(
              slots?.actions,
              actionsSlotProps,
              <>
                {!hideTimestamp && <span className={classes.timestamp}>{timestamp}</span>}

                <button
                  onClick={handleCopyClick}
                  className={`cursor-pointer ${classes.actionButton} ${isCopied ? 'text-green-600 dark:text-green-400' : ''}`}
                  aria-label="Copy message"
                  title={isCopied ? t('message.copied') : t('message.copy')}
                >
                  {isCopied ? <Check size={14} weight="bold" /> : <Copy size={14} weight="regular" />}
                </button>

                {onRegenerate && turnId && !isSystemMessage && isLastTurn && (
                  <button
                    onClick={handleRegenerateClick}
                    className={`cursor-pointer ${classes.actionButton}`}
                    aria-label="Regenerate response"
                    title="Regenerate"
                  >
                    <ArrowsClockwise size={14} weight="regular" />
                  </button>
                )}
              </>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

export default AssistantMessage
