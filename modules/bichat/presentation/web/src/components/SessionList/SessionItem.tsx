/**
 * SessionItem Component
 * Individual chat session item in the sidebar with actions menu
 * Supports swipe-left gesture to reveal delete action
 */

import { useRef, useState, memo } from 'react'
import { Link } from 'react-router-dom'
import { motion, useMotionValue, useTransform, type PanInfo } from 'framer-motion'
import { Trash } from '@phosphor-icons/react'
import { EditableText, type EditableTextRef } from '@iota-uz/sdk/bichat'
import SessionMenu from './SessionMenu'
import { sessionItemVariants } from '../../animations/variants'
import type { ChatSession } from '../../utils/sessionGrouping'

const DELETE_THRESHOLD = -80

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
    const [isDragging, setIsDragging] = useState(false)
    const dragX = useMotionValue(0)
    const deleteOpacity = useTransform(dragX, [-80, -40, 0], [1, 0.5, 0])
    const deleteScale = useTransform(dragX, [-80, -40, 0], [1, 0.8, 0.6])

    // Check if title is being generated (null or empty)
    const isTitleGenerating = !session.title

    // Generate title from session (use existing title or show generating state)
    const displayTitle = isTitleGenerating ? 'Generating...' : (session.title ?? 'Untitled Chat')

    const handleDragEnd = (_: MouseEvent | TouchEvent | PointerEvent, info: PanInfo) => {
      setIsDragging(false)
      if (info.offset.x < DELETE_THRESHOLD) {
        // Trigger delete via a synthetic click event
        const syntheticEvent = new MouseEvent('click', { bubbles: true }) as unknown as React.MouseEvent
        onDelete(syntheticEvent)
      }
    }

    return (
      <motion.div
        variants={sessionItemVariants}
        initial="initial"
        animate="animate"
        whileHover="hover"
        exit="exit"
        className="relative overflow-hidden rounded-xl"
      >
        {/* Delete zone revealed behind the item */}
        <motion.div
          className="absolute inset-y-0 right-0 w-20 flex items-center justify-center bg-red-500 rounded-r-xl"
          style={{ opacity: deleteOpacity, scale: deleteScale }}
          aria-hidden="true"
        >
          <Trash size={20} weight="bold" className="text-white" />
        </motion.div>

        {/* Draggable content layer */}
        <motion.div
          drag="x"
          dragDirectionLock
          dragConstraints={{ left: -100, right: 0 }}
          dragElastic={{ left: 0.2, right: 0.5 }}
          dragSnapToOrigin
          style={{ x: dragX }}
          onDragStart={() => setIsDragging(true)}
          onDragEnd={handleDragEnd}
        >
          <Link
            to={`/session/${session.id}`}
            onClick={(e) => {
              if (isDragging) e.preventDefault()
            }}
            className={`block px-3 py-2.5 rounded-xl transition-all duration-200 group relative cursor-pointer bg-white dark:bg-gray-900 ${
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
      </motion.div>
    )
  }
)

SessionItem.displayName = 'SessionItem'

export default SessionItem
