/**
 * UserMessage Component (Layer 3 Composite)
 * Styled component with slot-based customization for user messages
 */

import { useState, useCallback, useRef, useEffect, type ReactNode } from 'react'
import { Check, Copy, PencilSimple } from '@phosphor-icons/react'
import { formatDistanceToNow } from 'date-fns'
import AttachmentGrid from './AttachmentGrid'
import ImageModal from './ImageModal'
import type { Attachment, ImageAttachment, UserTurn } from '../types'
import { useTranslation } from '../hooks/useTranslation'

/* -------------------------------------------------------------------------------------------------
 * Slot Props Types
 * -----------------------------------------------------------------------------------------------*/

export interface UserMessageAvatarSlotProps {
  /** Default initials */
  initials: string
}

export interface UserMessageContentSlotProps {
  /** Message content text */
  content: string
}

export interface UserMessageAttachmentsSlotProps {
  /** Message attachments */
  attachments: Attachment[]
  /** Handler to open image viewer */
  onView: (index: number) => void
}

export interface UserMessageActionsSlotProps {
  /** Copy content to clipboard */
  onCopy: () => void
  /** Edit message (if available) */
  onEdit?: () => void
  /** Formatted timestamp */
  timestamp: string
  /** Whether copy action is available */
  canCopy: boolean
  /** Whether edit action is available */
  canEdit: boolean
}

/* -------------------------------------------------------------------------------------------------
 * Component Types
 * -----------------------------------------------------------------------------------------------*/

export interface UserMessageSlots {
  /** Custom avatar renderer */
  avatar?: ReactNode | ((props: UserMessageAvatarSlotProps) => ReactNode)
  /** Custom content renderer */
  content?: ReactNode | ((props: UserMessageContentSlotProps) => ReactNode)
  /** Custom attachments renderer */
  attachments?: ReactNode | ((props: UserMessageAttachmentsSlotProps) => ReactNode)
  /** Custom actions renderer */
  actions?: ReactNode | ((props: UserMessageActionsSlotProps) => ReactNode)
}

export interface UserMessageClassNames {
  /** Root container */
  root?: string
  /** Inner content wrapper */
  wrapper?: string
  /** Avatar container */
  avatar?: string
  /** Message bubble */
  bubble?: string
  /** Content text */
  content?: string
  /** Attachments container */
  attachments?: string
  /** Actions container */
  actions?: string
  /** Action button */
  actionButton?: string
  /** Timestamp */
  timestamp?: string
}

export interface UserMessageProps {
  /** User turn data */
  turn: UserTurn
  /** Turn ID for edit operations */
  turnId?: string
  /** User initials for avatar */
  initials?: string
  /** Slot overrides */
  slots?: UserMessageSlots
  /** Class name overrides */
  classNames?: UserMessageClassNames
  /** Copy handler */
  onCopy?: (content: string) => Promise<void> | void
  /** Edit handler */
  onEdit?: (turnId: string, newContent: string) => void
  /** Hide avatar */
  hideAvatar?: boolean
  /** Hide actions */
  hideActions?: boolean
  /** Hide timestamp */
  hideTimestamp?: boolean
}

const COPY_FEEDBACK_MS = 2000

/* -------------------------------------------------------------------------------------------------
 * Default Styles
 * -----------------------------------------------------------------------------------------------*/

const defaultClassNames: Required<UserMessageClassNames> = {
  root: 'flex gap-3 justify-end group',
  wrapper: 'flex-1 flex flex-col items-end max-w-[75%]',
  avatar: 'flex-shrink-0 w-8 h-8 rounded-full bg-primary-600 flex items-center justify-center text-white font-medium text-sm',
  bubble: 'bg-primary-600 text-white rounded-2xl rounded-br-sm px-4 py-3 shadow-sm',
  content: 'text-sm whitespace-pre-wrap break-words leading-relaxed',
  attachments: 'mb-2 w-full',
  actions: 'flex items-center gap-1 mt-2 opacity-0 group-hover:opacity-100 transition-opacity duration-150',
  actionButton: 'cursor-pointer p-2 text-gray-500 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 active:bg-gray-200 dark:active:bg-gray-700 rounded-md transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50',
  timestamp: 'text-xs text-gray-400 dark:text-gray-500 mr-1',
}

function mergeClassNames(
  defaults: Required<UserMessageClassNames>,
  overrides?: UserMessageClassNames
): Required<UserMessageClassNames> {
  if (!overrides) return defaults
  return {
    root: overrides.root ?? defaults.root,
    wrapper: overrides.wrapper ?? defaults.wrapper,
    avatar: overrides.avatar ?? defaults.avatar,
    bubble: overrides.bubble ?? defaults.bubble,
    content: overrides.content ?? defaults.content,
    attachments: overrides.attachments ?? defaults.attachments,
    actions: overrides.actions ?? defaults.actions,
    actionButton: overrides.actionButton ?? defaults.actionButton,
    timestamp: overrides.timestamp ?? defaults.timestamp,
  }
}

/* -------------------------------------------------------------------------------------------------
 * Component
 * -----------------------------------------------------------------------------------------------*/

export function UserMessage({
  turn,
  turnId,
  initials = 'U',
  slots,
  classNames: classNameOverrides,
  onCopy,
  onEdit,
  hideAvatar = false,
  hideActions = false,
  hideTimestamp = false,
}: UserMessageProps) {
  const { t } = useTranslation()
  const [selectedImageIndex, setSelectedImageIndex] = useState<number | null>(null)
  const [isEditing, setIsEditing] = useState(false)
  const [draftContent, setDraftContent] = useState('')
  const [isCopied, setIsCopied] = useState(false)
  const copyFeedbackTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const classes = mergeClassNames(defaultClassNames, classNameOverrides)

  useEffect(() => {
    return () => {
      if (copyFeedbackTimeoutRef.current) {
        clearTimeout(copyFeedbackTimeoutRef.current)
        copyFeedbackTimeoutRef.current = null
      }
    }
  }, [])

  const normalizedAttachments: Attachment[] = turn.attachments.map((attachment) => {
    if (!attachment.mimeType.startsWith('image/')) {
      return attachment
    }

    if (attachment.preview) {
      return attachment
    }
    if (attachment.base64Data) {
      if (attachment.base64Data.startsWith('data:')) {
        return {
          ...attachment,
          preview: attachment.base64Data,
        }
      }
      return {
        ...attachment,
        preview: `data:${attachment.mimeType};base64,${attachment.base64Data}`,
      }
    }
    if (attachment.url) {
      return {
        ...attachment,
        preview: attachment.url,
      }
    }
    return attachment
  })

  const imageAttachments: ImageAttachment[] = []
  const imageIndexByAttachmentIndex = new Map<number, number>()
  normalizedAttachments.forEach((attachment, index) => {
    if (!attachment.mimeType.startsWith('image/')) {
      return
    }
    if (!attachment.preview && !attachment.url) {
      return
    }
    imageIndexByAttachmentIndex.set(index, imageAttachments.length)
    imageAttachments.push({
      ...attachment,
      base64Data: attachment.base64Data || '',
      preview: attachment.preview || attachment.url || '',
    })
  })

  const handleCopyClick = useCallback(async () => {
    try {
      if (onCopy) {
        await onCopy(turn.content)
      } else {
        await navigator.clipboard.writeText(turn.content)
      }

      setIsCopied(true)
      if (copyFeedbackTimeoutRef.current) {
        clearTimeout(copyFeedbackTimeoutRef.current)
      }
      copyFeedbackTimeoutRef.current = setTimeout(() => {
        setIsCopied(false)
        copyFeedbackTimeoutRef.current = null
      }, COPY_FEEDBACK_MS)
    } catch (err) {
      setIsCopied(false)
      console.error('Failed to copy:', err)
    }
  }, [onCopy, turn.content])

  const handleEditClick = useCallback(() => {
    if (onEdit && turnId) {
      setDraftContent(turn.content)
      setIsEditing(true)
    }
  }, [onEdit, turnId, turn.content])

  const handleEditCancel = useCallback(() => {
    setIsEditing(false)
    setDraftContent('')
  }, [])

  const handleEditSave = useCallback(() => {
    if (!onEdit || !turnId) return
    const newContent = draftContent
    if (!newContent.trim()) return
    if (newContent === turn.content) {
      setIsEditing(false)
      return
    }
    onEdit(turnId, newContent)
    setIsEditing(false)
  }, [onEdit, turnId, draftContent, turn.content])

  const handleNavigate = useCallback(
    (direction: 'prev' | 'next') => {
      if (selectedImageIndex === null) return

      if (direction === 'prev' && selectedImageIndex > 0) {
        setSelectedImageIndex(selectedImageIndex - 1)
      } else if (direction === 'next' && selectedImageIndex < imageAttachments.length - 1) {
        setSelectedImageIndex(selectedImageIndex + 1)
      }
    },
    [selectedImageIndex, imageAttachments.length]
  )

  const currentAttachment =
    selectedImageIndex !== null ? imageAttachments[selectedImageIndex] : null

  const timestamp = formatDistanceToNow(new Date(turn.createdAt), { addSuffix: true })

  // Slot props
  const avatarSlotProps: UserMessageAvatarSlotProps = { initials }
  const contentSlotProps: UserMessageContentSlotProps = { content: turn.content }
  const attachmentsSlotProps: UserMessageAttachmentsSlotProps = {
    attachments: normalizedAttachments,
    onView: (index) => {
      const imageIndex = imageIndexByAttachmentIndex.get(index)
      if (imageIndex === undefined) {
        return
      }
      setSelectedImageIndex(imageIndex)
    },
  }
  const actionsSlotProps: UserMessageActionsSlotProps = {
    onCopy: handleCopyClick,
    onEdit: onEdit && turnId ? handleEditClick : undefined,
    timestamp,
    canCopy: true,
    canEdit: !!onEdit && !!turnId,
  }

  // Render helpers
  const renderSlot = <T,>(
    slot: ReactNode | ((props: T) => ReactNode) | undefined,
    props: T,
    defaultContent: ReactNode
  ): ReactNode => {
    if (slot === undefined) return defaultContent
    if (typeof slot === 'function') return slot(props)
    return slot
  }

  return (
    <div className={classes.root}>
      <div className={classes.wrapper}>
        {/* Attachments */}
        {normalizedAttachments.length > 0 && (
          <div className={classes.attachments}>
            {renderSlot(
              slots?.attachments,
              attachmentsSlotProps,
              <AttachmentGrid
                attachments={normalizedAttachments}
                onView={attachmentsSlotProps.onView}
              />
            )}
          </div>
        )}

        {/* Message bubble */}
        {turn.content && (
          <div className={classes.bubble}>
            <div className={classes.content}>
              {isEditing ? (
                <div className="space-y-2">
                  <textarea
                    value={draftContent}
                    onChange={(e) => setDraftContent(e.target.value)}
                    className="w-full min-h-[80px] resize-y rounded-lg px-3 py-2 bg-white/10 text-white placeholder-white/70 outline-none focus:ring-2 focus:ring-white/30"
                    aria-label="Edit message"
                  />
                  <div className="flex justify-end gap-2">
                    <button
                      type="button"
                      onClick={handleEditCancel}
                      className="px-3 py-1.5 rounded-lg bg-white/10 hover:bg-white/15 transition-colors text-sm font-medium"
                    >
                      Cancel
                    </button>
                    <button
                      type="button"
                      onClick={handleEditSave}
                      className="px-3 py-1.5 rounded-lg bg-white/20 hover:bg-white/25 transition-colors text-sm font-medium disabled:opacity-50 disabled:cursor-not-allowed"
                      disabled={!draftContent.trim() || draftContent === turn.content}
                    >
                      Save
                    </button>
                  </div>
                </div>
              ) : (
                renderSlot(slots?.content, contentSlotProps, turn.content)
              )}
            </div>
          </div>
        )}

        {/* Actions */}
        {!hideActions && (
          <div className={`${classes.actions} ${isCopied ? 'opacity-100' : ''}`}>
            {renderSlot(
              slots?.actions,
              actionsSlotProps,
              <>
                {!hideTimestamp && <span className={classes.timestamp}>{timestamp}</span>}

                <button
                  onClick={handleCopyClick}
                  className={`cursor-pointer ${classes.actionButton} ${isCopied ? 'text-green-600 dark:text-green-400' : ''}`}
                  aria-label="Copy message"
                  title={isCopied ? t('message.copied') : t('message.copy')}
                >
                  {isCopied ? <Check size={14} weight="bold" /> : <Copy size={14} weight="regular" />}
                </button>
                {isCopied && (
                  <span className="text-xs font-medium text-green-600 dark:text-green-400">
                    {t('message.copied')}
                  </span>
                )}

                {onEdit && turnId && (
                  <button
                    onClick={handleEditClick}
                    className={`cursor-pointer ${classes.actionButton}`}
                    aria-label="Edit message"
                    title="Edit"
                    disabled={isEditing}
                  >
                    <PencilSimple size={14} weight="regular" />
                  </button>
                )}
              </>
            )}
          </div>
        )}
      </div>

      {/* Avatar */}
      {!hideAvatar && (
        <div className={classes.avatar}>
          {renderSlot(slots?.avatar, avatarSlotProps, initials)}
        </div>
      )}

      {/* Image modal */}
      {currentAttachment && (
        <ImageModal
          isOpen={selectedImageIndex !== null}
          onClose={() => setSelectedImageIndex(null)}
          attachment={currentAttachment}
          allAttachments={imageAttachments}
          currentIndex={selectedImageIndex ?? 0}
          onNavigate={handleNavigate}
        />
      )}
    </div>
  )
}

export default UserMessage
