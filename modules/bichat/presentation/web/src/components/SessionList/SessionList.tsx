/**
 * SessionList Component
 * Main list with date grouping for chat sessions
 */

import { motion } from 'framer-motion'
import SessionItem from './SessionItem'
import DateGroupHeader from './DateGroupHeader'
import { sessionListContainerVariants } from '../../animations/variants'
import type { ChatSession, SessionGroup } from '../../utils/sessionGrouping'

interface SessionListProps {
  groups: SessionGroup[]
  pinnedSessions: ChatSession[]
  activeSessionId: string | undefined
  onDelete: (sessionId: string, e: React.MouseEvent) => void
  onTogglePin: (sessionId: string, isPinned: boolean, e: React.MouseEvent) => void
  onRename: (sessionId: string, newTitle: string) => void
  onNavigate?: () => void
}

export default function SessionList({
  groups,
  pinnedSessions,
  activeSessionId,
  onDelete,
  onTogglePin,
  onRename,
  onNavigate,
}: SessionListProps) {
  return (
    <>
      {/* Pinned Sessions */}
      {pinnedSessions.length > 0 && (
        <div className="mb-4">
          <DateGroupHeader groupName="Pinned" count={pinnedSessions.length} />
          <motion.div
            className="space-y-1 mt-2"
            variants={sessionListContainerVariants}
            initial="hidden"
            animate="visible"
            role="list"
            aria-label="Pinned chats"
          >
            {pinnedSessions.map((session) => (
              <SessionItem
                key={session.id}
                session={session}
                isActive={session.id === activeSessionId}
                onDelete={(e) => onDelete(session.id, e)}
                onTogglePin={(e) => onTogglePin(session.id, session.pinned || false, e)}
                onRename={(newTitle) => onRename(session.id, newTitle)}
                onNavigate={onNavigate}
              />
            ))}
          </motion.div>
          <div className="border-b border-gray-200 dark:border-gray-700 my-3" />
        </div>
      )}

      {/* Grouped Sessions by Date */}
      {groups.map((group) => (
        <div key={group.name} className="mb-4">
          <DateGroupHeader groupName={group.name} count={group.sessions.length} />
          <motion.div
            className="space-y-1 mt-2"
            variants={sessionListContainerVariants}
            initial="hidden"
            animate="visible"
            role="list"
            aria-label={`${group.name} chats`}
          >
            {group.sessions.map((session) => (
              <SessionItem
                key={session.id}
                session={session}
                isActive={session.id === activeSessionId}
                onDelete={(e) => onDelete(session.id, e)}
                onTogglePin={(e) => onTogglePin(session.id, session.pinned || false, e)}
                onRename={(newTitle) => onRename(session.id, newTitle)}
                onNavigate={onNavigate}
              />
            ))}
          </motion.div>
        </div>
      ))}
    </>
  )
}
