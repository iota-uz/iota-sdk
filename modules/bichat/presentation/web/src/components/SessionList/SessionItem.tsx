/**
 * SessionItem Component
 * Individual chat session item in the sidebar with actions menu
 * Desktop-only version (no drag/drop, no touch gestures)
 */

import { useRef, memo } from 'react'
import { Link } from 'react-router-dom'
import { motion } from 'framer-motion'
import SessionMenu from './SessionMenu'
import EditableTitle, { type EditableTitleRef } from '../EditableTitle'
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
    const editableTitleRef = useRef<EditableTitleRef>(null)

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
          className={`block px-3 py-2 rounded-lg transition-colors group relative ${
            isActive
              ? 'bg-primary-50/50 dark:bg-primary-900/30 text-primary-700 dark:text-primary-400 border-l-4 border-primary-400 dark:border-primary-600'
              : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 border-l-4 border-transparent'
          }`}
          aria-current={isActive ? 'page' : undefined}
          role="listitem"
        >
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-2 min-w-0 flex-1">
              <EditableTitle
                ref={editableTitleRef}
                title={displayTitle}
                onSave={onRename}
                maxLength={60}
                isLoading={isTitleGenerating}
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
