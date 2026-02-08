/**
 * TurnBubble component (Layer 4 - Backward Compatible)
 * Container for a conversation turn (user message + assistant response)
 *
 * Renders both the user's message and the assistant's response in a single
 * visual grouping. If the assistant hasn't responded yet, only shows user message.
 *
 * For primitive-level control, use Turn from '@iota-uz/sdk/bichat/primitives'
 */

import { type ReactNode } from 'react'
import type { ConversationTurn } from '../types'
import { useChatMessaging } from '../context/ChatContext'
import { UserTurnView, type UserTurnViewProps } from './UserTurnView'
import { AssistantTurnView, type AssistantTurnViewProps } from './AssistantTurnView'
import { InlineQuestionForm } from './InlineQuestionForm'
import type { UserMessageSlots, UserMessageClassNames } from './UserMessage'
import type { AssistantMessageSlots, AssistantMessageClassNames } from './AssistantMessage'

export interface TurnBubbleClassNames {
  /** Root container */
  root?: string
  /** User turn wrapper */
  userTurn?: string
  /** Assistant turn wrapper */
  assistantTurn?: string
}

export interface TurnBubbleProps {
  /** The conversation turn containing user and optional assistant content */
  turn: ConversationTurn
  /** When true, this turn is the last in the list (e.g. Regenerate shows only on last assistant message) */
  isLastTurn?: boolean
  /** Custom render function for user turn (full control) */
  renderUserTurn?: (turn: ConversationTurn) => ReactNode
  /** Custom render function for assistant turn (full control) */
  renderAssistantTurn?: (turn: ConversationTurn) => ReactNode
  /** Props passed to UserTurnView (when not using custom renderer) */
  userTurnProps?: Omit<UserTurnViewProps, 'turn'>
  /** Props passed to AssistantTurnView (when not using custom renderer) */
  assistantTurnProps?: Omit<AssistantTurnViewProps, 'turn'>
  /** Slots for user message customization */
  userMessageSlots?: UserMessageSlots
  /** Slots for assistant message customization */
  assistantMessageSlots?: AssistantMessageSlots
  /** Class names for user message */
  userMessageClassNames?: UserMessageClassNames
  /** Class names for assistant message */
  assistantMessageClassNames?: AssistantMessageClassNames
  /** Class names for turn bubble container */
  classNames?: TurnBubbleClassNames
  /** Whether assistant response is streaming */
  isStreaming?: boolean
}

const defaultClassNames: Required<TurnBubbleClassNames> = {
  root: 'space-y-4',
  userTurn: '',
  assistantTurn: '',
}

export function TurnBubble({
  turn,
  isLastTurn = false,
  renderUserTurn,
  renderAssistantTurn,
  userTurnProps,
  assistantTurnProps,
  userMessageSlots,
  assistantMessageSlots,
  userMessageClassNames,
  assistantMessageClassNames,
  classNames,
  isStreaming = false,
}: TurnBubbleProps) {
  const { pendingQuestion } = useChatMessaging()
  const classes = {
    root: classNames?.root ?? defaultClassNames.root,
    userTurn: classNames?.userTurn ?? defaultClassNames.userTurn,
    assistantTurn: classNames?.assistantTurn ?? defaultClassNames.assistantTurn,
  }
  const isSystemSummaryTurn =
    turn.userTurn.content.trim() === '' && turn.assistantTurn?.role === 'system'

  // Show standalone pending question when there's no assistant turn
  // (agent called ask_user_question without generating content first)
  const showStandalonePendingQuestion =
    !turn.assistantTurn &&
    !!pendingQuestion &&
    pendingQuestion.status === 'PENDING' &&
    pendingQuestion.turnId === turn.id

  return (
    <div className={classes.root} data-turn-id={turn.id}>
      {/* User message */}
      {!isSystemSummaryTurn && (
        <div className={classes.userTurn}>
          {renderUserTurn ? (
            renderUserTurn(turn)
          ) : (
            <UserTurnView
              turn={turn}
              slots={userMessageSlots}
              classNames={userMessageClassNames}
              {...userTurnProps}
            />
          )}
        </div>
      )}

      {/* Assistant response (if available) */}
      {turn.assistantTurn && (
        <div className={classes.assistantTurn}>
          {renderAssistantTurn ? (
            renderAssistantTurn(turn)
          ) : (
            <AssistantTurnView
              turn={turn}
              isLastTurn={isLastTurn}
              isStreaming={isStreaming}
              slots={assistantMessageSlots}
              classNames={assistantMessageClassNames}
              {...assistantTurnProps}
            />
          )}
        </div>
      )}

      {/* Standalone pending question (no assistant turn yet) */}
      {showStandalonePendingQuestion && (
        <InlineQuestionForm pendingQuestion={pendingQuestion} />
      )}
    </div>
  )
}

export default TurnBubble
