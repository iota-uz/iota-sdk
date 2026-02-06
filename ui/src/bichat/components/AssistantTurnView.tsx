/**
 * AssistantTurnView Component (Layer 4 - Backward Compatible)
 * Displays assistant messages with markdown, charts, sources, downloads, code outputs, and streaming cursor
 *
 * Uses turn-based architecture - receives a ConversationTurn and displays
 * the assistantTurn content.
 *
 * For more customization, use the AssistantMessage component directly with slots.
 */

import { useChat } from '../context/ChatContext'
import { AssistantMessage, type AssistantMessageSlots, type AssistantMessageClassNames } from './AssistantMessage'
import { SystemMessage } from './SystemMessage'
import type { ConversationTurn } from '../types'

export interface AssistantTurnViewProps {
  /** The conversation turn containing the assistant response */
  turn: ConversationTurn
  /** Whether the response is currently being streamed */
  isStreaming?: boolean
  /** Slot overrides for customization */
  slots?: AssistantMessageSlots
  /** Class name overrides */
  classNames?: AssistantMessageClassNames
  /** Hide avatar */
  hideAvatar?: boolean
  /** Hide actions */
  hideActions?: boolean
  /** Hide timestamp */
  hideTimestamp?: boolean
}

export function AssistantTurnView({
  turn,
  isStreaming = false,
  slots,
  classNames,
  hideAvatar,
  hideActions,
  hideTimestamp,
}: AssistantTurnViewProps) {
  const { handleCopy, handleRegenerate, pendingQuestion, sendMessage, loading, debugMode } = useChat()

  const assistantTurn = turn.assistantTurn
  if (!assistantTurn) return null

  if (assistantTurn.role === 'system') {
    return (
      <SystemMessage
        content={assistantTurn.content}
        createdAt={assistantTurn.createdAt}
        onCopy={handleCopy}
        hideActions={hideActions}
        hideTimestamp={hideTimestamp}
      />
    )
  }

  return (
    <AssistantMessage
      turn={assistantTurn}
      turnId={turn.id}
      isStreaming={isStreaming}
      pendingQuestion={pendingQuestion}
      slots={slots}
      classNames={classNames}
      onCopy={handleCopy}
      onRegenerate={handleRegenerate}
      onSendMessage={sendMessage}
      sendDisabled={loading || isStreaming}
      hideAvatar={hideAvatar}
      hideActions={hideActions}
      hideTimestamp={hideTimestamp}
      showDebug={debugMode}
    />
  )
}

export default AssistantTurnView
