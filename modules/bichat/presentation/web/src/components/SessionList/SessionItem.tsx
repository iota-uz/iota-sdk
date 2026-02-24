/**
 * SessionItem Component
 * Individual chat session item in the sidebar with actions menu
 * Supports swipe-left gesture to reveal archive action
 */

import { useRef, useState, memo } from 'react'
import { Link } from 'react-router-dom'
import { motion, useMotionValue, useTransform, type PanInfo } from 'framer-motion'
import { Archive, UsersThree } from '@phosphor-icons/react'
import { EditableText, type EditableTextRef, useTranslation } from '@iota-uz/sdk/bichat'
import SessionMenu from './SessionMenu'
import { sessionItemVariants } from '../../animations/variants'
import type { ChatSession } from '../../utils/sessionGrouping'

const ARCHIVE_THRESHOLD = -80

interface SessionItemProps {
  session: ChatSession
  isActive: boolean
  onTogglePin: (e: React.MouseEvent) => void
  onRename: (newTitle: string) => void
  onArchive: (e?: React.MouseEvent) => void
  onNavigate?: () => void
}

const SessionItem = memo<SessionItemProps>(
  ({ session, isActive, onTogglePin, onRename, onArchive, onNavigate }) => {
    const editableTitleRef = useRef<EditableTextRef>(null)
    const { t } = useTranslation()
    const [isDragging, setIsDragging] = useState(false)
    const dragX = useMotionValue(0)
    const archiveOpacity = useTransform(dragX, [-80, -40, 0], [1, 0.5, 0])
    const archiveScale = useTransform(dragX, [-80, -40, 0], [1, 0.8, 0.6])

    // Check if title is being generated (null or empty)
    const isTitleGenerating = !session.title

    // Generate title from session (use existing title or show generating state)
    const displayTitle = isTitleGenerating ? t('BiChat.Common.Generating') : (session.title ?? t('BiChat.Common.Untitled'))
    const source = (session.access?.source ?? '').toLowerCase()
    const isMemberSession = source === 'member'
    const isGroupOrShared = Boolean(
      session.isGroup ||
        isMemberSession ||
        (session.access?.role && session.access.role !== 'owner' && session.access.role !== 'read_all') ||
        (session.memberCount !== undefined && session.memberCount > 1),
    )

    const handleDragEnd = (_: MouseEvent | TouchEvent | PointerEvent, info: PanInfo) => {
      setIsDragging(false)
      if (info.offset.x < ARCHIVE_THRESHOLD) {
        onArchive()
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
        {/* Archive zone revealed behind the item */}
        <motion.div
          className="absolute inset-y-0 right-0 w-20 flex items-center justify-center bg-gray-500 dark:bg-gray-600 rounded-r-xl"
          style={{ opacity: archiveOpacity, scale: archiveScale }}
          aria-hidden="true"
        >
          <Archive size={20} weight="bold" className="text-white" />
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
              if (!isDragging) onNavigate?.()
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
                {isGroupOrShared && (
                  <UsersThree size={14} weight="duotone" className="text-primary-500 dark:text-primary-400 flex-shrink-0" />
                )}
                <div className="flex flex-col min-w-0 flex-1">
                  <EditableText
                    ref={editableTitleRef}
                    value={displayTitle}
                    onSave={onRename}
                    maxLength={60}
                    isLoading={isTitleGenerating}
                    placeholder={t('BiChat.Common.Untitled')}
                    size="sm"
                  />
                  {isGroupOrShared && (
                    <span className="text-[11px] text-gray-400 dark:text-gray-500 truncate mt-0.5">
                      {session.isGroup ? t('BiChat.Sidebar.GroupChat') : t('BiChat.Sidebar.SharedWithYou')}
                      {session.memberCount && session.memberCount > 0 && (
                        <span className="inline-flex items-center ml-1 rounded-full bg-primary-50 dark:bg-primary-900/30 px-1.5 text-[10px] font-medium text-primary-600 dark:text-primary-400">
                          {session.memberCount}
                        </span>
                      )}
                    </span>
                  )}
                </div>
              </div>
              <SessionMenu
                isPinned={session.pinned || false}
                onPin={onTogglePin}
                onRename={() => editableTitleRef.current?.startEditing()}
                onArchive={onArchive}
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
