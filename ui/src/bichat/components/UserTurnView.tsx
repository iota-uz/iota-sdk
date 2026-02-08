/**
 * UserTurnView Component (Layer 4 - Backward Compatible)
 * Displays user messages with attachments, image modal, and actions
 *
 * Uses turn-based architecture - receives a ConversationTurn and displays
 * the userTurn content.
 *
 * For more customization, use the UserMessage component directly with slots.
 */

import { useChat } from '../context/ChatContext'
import { UserMessage, type UserMessageSlots, type UserMessageClassNames } from './UserMessage'
import type { ConversationTurn } from '../types'

export interface UserTurnViewProps {
  /** The conversation turn containing the user message */
  turn: ConversationTurn
  /** Slot overrides for customization */
  slots?: UserMessageSlots
  /** Class name overrides */
  classNames?: UserMessageClassNames
  /** User initials for avatar */
  initials?: string
  /** Hide avatar */
  hideAvatar?: boolean
  /** Hide actions */
  hideActions?: boolean
  /** Hide timestamp */
  hideTimestamp?: boolean
  /** Whether edit action should be available */
  allowEdit?: boolean
}

export function UserTurnView({
  turn,
  slots,
  classNames,
  initials = 'U',
  hideAvatar,
  hideActions,
  hideTimestamp,
  allowEdit,
}: UserTurnViewProps) {
  const { handleEdit, handleCopy } = useChat()

  return (
    <UserMessage
      turn={turn.userTurn}
      turnId={turn.id}
      initials={initials}
      slots={slots}
      classNames={classNames}
      onCopy={handleCopy}
      onEdit={handleEdit}
      hideAvatar={hideAvatar}
      hideActions={hideActions}
      hideTimestamp={hideTimestamp}
      allowEdit={allowEdit}
    />
  )
}

export default UserTurnView
