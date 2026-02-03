/**
 * TurnBubble component
 * Container for individual messages with role-based styling
 */

import { ReactNode } from 'react'
import { Message, MessageRole } from '../types'
import { UserTurnView } from './UserTurnView'
import { AssistantTurnView } from './AssistantTurnView'

interface TurnBubbleProps {
  message: Message
  renderUserMessage?: (message: Message) => ReactNode
  renderAssistantMessage?: (message: Message) => ReactNode
}

export function TurnBubble({
  message,
  renderUserMessage,
  renderAssistantMessage,
}: TurnBubbleProps) {
  if (message.role === MessageRole.User) {
    if (renderUserMessage) {
      return <>{renderUserMessage(message)}</>
    }
    return <UserTurnView message={message} />
  }

  if (message.role === MessageRole.Assistant) {
    if (renderAssistantMessage) {
      return <>{renderAssistantMessage(message)}</>
    }
    return <AssistantTurnView message={message} />
  }

  // System and Tool messages are hidden by default
  return null
}
