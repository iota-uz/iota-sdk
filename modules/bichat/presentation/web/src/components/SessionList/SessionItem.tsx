/**
 * SessionItem Component
 * Individual chat session item in the sidebar with actions menu
 * Desktop-only version (no drag/drop, no touch gestures)
 */

import { useRef, memo } from 'react'
import { Link } from 'react-router-dom'
import { motion } from 'framer-motion'
import { EditableText, type EditableTextRef } from '@iota-uz/sdk/bichat'
import SessionMenu from './SessionMenu'
import { sessionItemVariants } from '../../animations/variants'
import type { ChatSession } from '../../utils/sessionGrouping'

interface SessionItemProps {
  session: ChatSession
  isActive: boolean
  onDelete: (e: React.MouseEvent) => void
  onTogglePin: (e: React.MouseEvent) => void
  onRename: (newTitle: string) => void
}

const SessionItem = memo<SessionItemProps>(
  ({ session, isActive, onDelete, onTogglePin, onRename }) => {
    const editableTitleRef = useRef<EditableTextRef>(null)

    // Check if title is being generated (null or empty)
    const isTitleGenerating = !session.title

    // Generate title from session (use existing title or show generating state)
    const displayTitle = isTitleGenerating ? 'Generating...' : (session.title ?? 'Untitled Chat')

    return (
      <motion.div
        variants={sessionItemVariants}
        initial="initial"
        animate="animate"
        whileHover="hover"
        exit="exit"
      >
        <Link
          to={`/session/${session.id}`}
          className={`block px-3 py-2.5 rounded-xl transition-all duration-200 group relative cursor-pointer ${
            isActive
              ? 'bg-primary-50 dark:bg-primary-900/30 text-primary-700 dark:text-primary-300 shadow-sm'
              : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800/50'
          }`}
          aria-current={isActive ? 'page' : undefined}
          role="listitem"
        >
          {/* Active indicator bar */}
          {isActive && (
            <div className="absolute left-0 top-1/2 -translate-y-1/2 w-1 h-6 bg-primary-500 rounded-full" />
          )}
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-2 min-w-0 flex-1">
              <EditableText
                ref={editableTitleRef}
                value={displayTitle}
                onSave={onRename}
                maxLength={60}
                isLoading={isTitleGenerating}
                placeholder="Untitled Chat"
                size="sm"
              />
            </div>
            <SessionMenu
              isPinned={session.pinned || false}
              onPin={onTogglePin}
              onRename={() => editableTitleRef.current?.startEditing()}
              onDelete={onDelete}
            />
          </div>
        </Link>
      </motion.div>
    )
  }
)

SessionItem.displayName = 'SessionItem'

export default SessionItem
