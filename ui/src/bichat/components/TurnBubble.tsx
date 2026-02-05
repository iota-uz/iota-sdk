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
import { UserTurnView, type UserTurnViewProps } from './UserTurnView'
import { AssistantTurnView, type AssistantTurnViewProps } from './AssistantTurnView'
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
  const classes = {
    root: classNames?.root ?? defaultClassNames.root,
    userTurn: classNames?.userTurn ?? defaultClassNames.userTurn,
    assistantTurn: classNames?.assistantTurn ?? defaultClassNames.assistantTurn,
  }

  return (
    <div className={classes.root} data-turn-id={turn.id}>
      {/* User message */}
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

      {/* Assistant response (if available) */}
      {turn.assistantTurn && (
        <div className={classes.assistantTurn}>
          {renderAssistantTurn ? (
            renderAssistantTurn(turn)
          ) : (
            <AssistantTurnView
              turn={turn}
              isStreaming={isStreaming}
              slots={assistantMessageSlots}
              classNames={assistantMessageClassNames}
              {...assistantTurnProps}
            />
          )}
        </div>
      )}
    </div>
  )
}

export default TurnBubble
